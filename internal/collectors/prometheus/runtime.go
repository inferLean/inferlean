package prometheus

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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/new-cli/internal/tools"
)

type runtimeProcess struct {
	cmd     *exec.Cmd
	workDir string
}

func tryStartRuntime(ctx context.Context, targets []Target, every time.Duration) *runtimeProcess {
	path, err := tools.ResolveBinary("prometheus")
	if err != nil {
		return nil
	}
	configBody := buildConfig(targets, every)
	if strings.TrimSpace(configBody) == "" {
		return nil
	}
	workDir, err := os.MkdirTemp("", "inferlean-prom-*")
	if err != nil {
		return nil
	}
	configPath := filepath.Join(workDir, "prometheus.yml")
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		_ = os.RemoveAll(workDir)
		return nil
	}
	port, err := reservePort()
	if err != nil {
		_ = os.RemoveAll(workDir)
		return nil
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	cmd := exec.CommandContext(
		ctx,
		path,
		"--config.file="+configPath,
		"--storage.tsdb.path="+filepath.Join(workDir, "tsdb"),
		"--web.listen-address="+addr,
		"--log.level=error",
	)
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(workDir)
		return nil
	}
	runtime := &runtimeProcess{cmd: cmd, workDir: workDir}
	readyURL := fmt.Sprintf("http://%s/-/ready", addr)
	if err := waitReady(ctx, readyURL, 5*time.Second); err != nil {
		_ = runtime.Stop(context.Background())
		return nil
	}
	return runtime
}

func buildConfig(targets []Target, every time.Duration) string {
	rows := []string{
		"global:",
		fmt.Sprintf("  scrape_interval: %s", parseDurationSeconds(every)),
	}
	jobs := make([]string, 0)
	for _, target := range targets {
		if block := buildJob(target); block != "" {
			jobs = append(jobs, block)
		}
	}
	if len(jobs) == 0 {
		return ""
	}
	rows = append(rows, "scrape_configs:")
	rows = append(rows, jobs...)
	return strings.Join(rows, "\n") + "\n"
}

func buildJob(target Target) string {
	parsed, err := url.Parse(strings.TrimSpace(target.Endpoint))
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	metricsPath := parsed.Path
	if strings.TrimSpace(metricsPath) == "" {
		metricsPath = "/metrics"
	}
	lines := []string{
		fmt.Sprintf("- job_name: %q", target.Name),
		fmt.Sprintf("  metrics_path: %q", metricsPath),
		fmt.Sprintf("  scheme: %q", parsed.Scheme),
		"  static_configs:",
		fmt.Sprintf("  - targets: [%q]", parsed.Host),
	}
	return strings.Join(lines, "\n")
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

func (r *runtimeProcess) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	defer func() {
		if strings.TrimSpace(r.workDir) != "" {
			_ = os.RemoveAll(r.workDir)
		}
	}()
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	_ = r.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() {
		done <- r.cmd.Wait()
	}()
	select {
	case err := <-done:
		return normalizeWaitErr(err)
	case <-ctx.Done():
		_ = r.cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(2 * time.Second):
		_ = r.cmd.Process.Kill()
		return normalizeWaitErr(<-done)
	}
}

func normalizeWaitErr(err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	if strings.Contains(text, "signal: interrupt") || strings.Contains(text, "signal: killed") {
		return nil
	}
	return err
}

func parseDurationSeconds(duration time.Duration) string {
	seconds := duration.Seconds()
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatFloat(seconds, 'f', -1, 64) + "s"
}
