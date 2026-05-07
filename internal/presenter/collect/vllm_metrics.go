package collect

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

type vllmMetricsSession struct {
	Endpoint string
	cmd      *exec.Cmd
	stdout   bytes.Buffer
	stderr   bytes.Buffer
}

func startVLLMMetrics(ctx context.Context, target vllmdiscovery.Candidate) (string, *vllmMetricsSession, error) {
	switch strings.ToLower(strings.TrimSpace(target.Source)) {
	case "pod", "kubernetes":
		return startVLLMPortForward(ctx, target)
	case "docker":
		endpoint := strings.TrimSpace(target.MetricsEndpoint)
		if endpoint == "" {
			return "", nil, fmt.Errorf("docker vLLM metrics port is not published; expose the vLLM port with docker -p and run collection again")
		}
		return endpoint, nil, nil
	default:
		endpoint := strings.TrimSpace(target.MetricsEndpoint)
		if endpoint == "" {
			endpoint = shared.MetricsEndpoint("127.0.0.1", shared.InferMetricsPort(target.RawCommandLine, nil))
		}
		return endpoint, nil, nil
	}
}

func startVLLMPortForward(ctx context.Context, target vllmdiscovery.Candidate) (string, *vllmMetricsSession, error) {
	podName := strings.TrimSpace(target.PodName)
	if podName == "" {
		return "", nil, fmt.Errorf("cannot port-forward vLLM metrics without pod name")
	}
	namespace := strings.TrimSpace(target.Namespace)
	if namespace == "" {
		namespace = "default"
	}
	remotePort := endpointPort(target.MetricsEndpoint)
	if remotePort <= 0 {
		remotePort = shared.InferMetricsPort(target.RawCommandLine, nil)
	}
	localPort, err := reserveLocalPort()
	if err != nil {
		return "", nil, err
	}
	endpoint := shared.MetricsEndpoint("127.0.0.1", localPort)
	args := []string{"port-forward", "-n", namespace, "pod/" + podName, fmt.Sprintf("%d:%d", localPort, remotePort)}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	session := &vllmMetricsSession{Endpoint: endpoint, cmd: cmd}
	cmd.Stdout = &session.stdout
	cmd.Stderr = &session.stderr
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start vLLM metrics port-forward: %w", err)
	}
	if err := waitMetricsReady(ctx, endpoint, 5*time.Second); err != nil {
		_ = session.Stop(context.Background())
		return "", nil, fmt.Errorf("vLLM metrics port-forward failed: %w", err)
	}
	return endpoint, session, nil
}

func endpointPort(raw string) int {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func reserveLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to reserve local port")
	}
	return addr.Port, nil
}

func waitMetricsReady(ctx context.Context, endpoint string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s", endpoint)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err == nil {
			resp, reqErr := client.Do(req)
			if reqErr == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (s *vllmMetricsSession) Stop(ctx context.Context) error {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	_ = s.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()
	select {
	case err := <-done:
		return normalizeMetricsWaitErr(err)
	case <-ctx.Done():
		_ = s.cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		return normalizeMetricsWaitErr(<-done)
	}
}

func normalizeMetricsWaitErr(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "signal: interrupt") || strings.Contains(err.Error(), "signal: killed") {
		return nil
	}
	return err
}

func (s *vllmMetricsSession) Stdout() string {
	if s == nil {
		return ""
	}
	return s.stdout.String()
}

func (s *vllmMetricsSession) Stderr() string {
	if s == nil {
		return ""
	}
	return s.stderr.String()
}
