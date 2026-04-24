package dcgm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Available bool
	Reason    string
}

type Session struct {
	Endpoint string
	cmd      *exec.Cmd
	stdout   bytes.Buffer
	stderr   bytes.Buffer
}

type StartResult struct {
	Available bool
	Reason    string
	Endpoint  string
	Session   *Session
}

func Check(ctx context.Context) Result {
	_ = ctx
	_, err := exec.LookPath("dcgm-exporter")
	if err != nil {
		return Result{Available: false, Reason: "dcgm-exporter not found"}
	}
	return Result{Available: true}
}

func Start(ctx context.Context) StartResult {
	if endpoint := detectExistingEndpoint(ctx); endpoint != "" {
		return StartResult{Available: true, Endpoint: endpoint}
	}
	path, err := exec.LookPath("dcgm-exporter")
	if err != nil {
		return StartResult{Available: false, Reason: "dcgm-exporter not found"}
	}
	port, err := reservePort()
	if err != nil {
		return StartResult{Available: false, Reason: err.Error()}
	}
	address := fmt.Sprintf("127.0.0.1:%d", port)
	candidates := [][]string{
		{"-a", address},
		{"--web.listen-address=" + address},
	}
	for _, args := range candidates {
		if result := startAttempt(ctx, path, args, address); result.Available {
			return result
		}
	}
	return StartResult{Available: false, Reason: "dcgm-exporter failed to start"}
}

func detectExistingEndpoint(ctx context.Context) string {
	candidates := []string{}
	if endpoint := strings.TrimSpace(os.Getenv("INFERLEAN_DCGM_EXPORTER_ENDPOINT")); endpoint != "" {
		candidates = append(candidates, endpoint)
	}
	candidates = append(candidates,
		"http://127.0.0.1:9400/metrics",
		"http://localhost:9400/metrics",
	)
	for _, candidate := range candidates {
		endpoint := normalizeEndpoint(candidate)
		if endpoint == "" {
			continue
		}
		if err := waitReady(ctx, endpoint, 1500*time.Millisecond); err == nil {
			return endpoint
		}
	}
	return ""
}

func normalizeEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	if strings.TrimSpace(parsed.Path) == "" || parsed.Path == "/" {
		parsed.Path = "/metrics"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func startAttempt(ctx context.Context, path string, args []string, address string) StartResult {
	endpoint := fmt.Sprintf("http://%s/metrics", address)
	cmd := exec.CommandContext(ctx, path, args...)
	session := &Session{Endpoint: endpoint, cmd: cmd}
	cmd.Stdout = &session.stdout
	cmd.Stderr = &session.stderr
	if err := cmd.Start(); err != nil {
		return StartResult{Available: false, Reason: err.Error()}
	}
	if err := waitReady(ctx, endpoint, 5*time.Second); err != nil {
		_ = session.Stop(context.Background())
		return StartResult{Available: false, Reason: "dcgm-exporter failed readiness"}
	}
	return StartResult{Available: true, Endpoint: endpoint, Session: session}
}

func reservePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("failed to reserve port")
	}
	return addr.Port, nil
}

func waitReady(ctx context.Context, endpoint string, timeout time.Duration) error {
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

func (s *Session) Stop(ctx context.Context) error {
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
		return normalizeWaitErr(err)
	case <-ctx.Done():
		_ = s.cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		return normalizeWaitErr(<-done)
	}
}

func normalizeWaitErr(err error) error {
	if err == nil {
		return nil
	}
	if stringsContains(err.Error(), "signal: interrupt") || stringsContains(err.Error(), "signal: killed") {
		return nil
	}
	return err
}

func stringsContains(text, part string) bool {
	return bytes.Contains([]byte(text), []byte(part))
}

func (s *Session) Stdout() string {
	if s == nil {
		return ""
	}
	return s.stdout.String()
}

func (s *Session) Stderr() string {
	if s == nil {
		return ""
	}
	return s.stderr.String()
}
