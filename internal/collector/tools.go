package collector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/debug"
	"github.com/inferLean/inferlean/internal/discovery"
)

func resolveToolPaths() (toolPaths, error) {
	root, err := resolveToolsRoot()
	if err != nil {
		return toolPaths{}, err
	}

	archDir := "linux_" + runtime.GOARCH
	prometheus, err := resolveBundledTool(root, archDir, "prometheus")
	if err != nil {
		return toolPaths{}, err
	}
	nodeExporter, err := resolveBundledTool(root, archDir, "node_exporter")
	if err != nil {
		return toolPaths{}, err
	}
	dcgmExporter := resolveOptionalDCGMExporter(root, archDir)
	dcgmCollectors := resolveOptionalToolFile(filepath.Join(root, archDir, "dcgm"), "default-counters.csv")
	return toolPaths{
		Prometheus:     prometheus,
		NodeExporter:   nodeExporter,
		DCGMExporter:   dcgmExporter,
		DCGMCollectors: dcgmCollectors,
	}, nil
}

func resolveToolsRoot() (string, error) {
	for _, candidate := range []string{
		strings.TrimSpace(os.Getenv("INFERLEAN_TOOLS_DIR")),
		dirIfExists(executableToolsDir()),
		dirIfExists(filepath.Join(currentWorkingDir(), "dist", "tools")),
	} {
		if candidate != "" {
			return candidate, nil
		}
	}
	return "", errors.New("could not locate bundled tools; install InferLean from a Linux release bundle or set INFERLEAN_TOOLS_DIR")
}

func executableToolsDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exePath), "tools")
}

func currentWorkingDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func dirIfExists(path string) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return path
}

func resolveBundledTool(root, archDir, name string) (string, error) {
	return findToolExecutable(filepath.Join(root, archDir, name), name)
}

func resolveOptionalDCGMExporter(root, archDir string) string {
	if path := strings.TrimSpace(os.Getenv("INFERLEAN_DCGM_EXPORTER_BIN")); path != "" {
		return path
	}
	for _, name := range []string{"dcgm-exporter", "dcgm_exporter"} {
		path, err := findToolExecutable(filepath.Join(root, archDir), name)
		if err == nil {
			return path
		}
	}
	return ""
}

func findToolExecutable(root, name string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || info.Name() != name || !isExecutable(info) {
			return walkErr
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found under %s", name, root)
	}
	return found, nil
}

func isExecutable(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func resolveOptionalToolFile(root, name string) string {
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || info.Name() != name {
			return walkErr
		}
		found = path
		return filepath.SkipAll
	})
	return found
}

func reservePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func buildVLLMTarget(target discovery.CandidateGroup) string {
	if target.RuntimeConfig.Port == 0 {
		return ""
	}
	host := normalizeListenHost(target.RuntimeConfig.Host)
	return net.JoinHostPort(host, strconv.Itoa(target.RuntimeConfig.Port))
}

func normalizeListenHost(host string) string {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1"
	default:
		return host
	}
}

func discoverExternalDCGMTarget() string {
	if endpoint := strings.TrimSpace(os.Getenv("INFERLEAN_DCGM_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	for _, candidate := range []string{"127.0.0.1:9400", "localhost:9400"} {
		if endpointResponds("http://" + candidate + "/metrics") {
			return candidate
		}
	}
	return ""
}

func endpointResponds(rawURL string) bool {
	client := newHTTPClient(2 * time.Second)
	resp, err := client.Get(rawURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func startProcess(ctx context.Context, binary string, args []string, stdoutPath, stderrPath string) (*managedProcess, error) {
	stdout, stderr, err := openProcessLogs(stdoutPath, stderrPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		stdout.Close()
		stderr.Close()
		return nil, err
	}

	debug.Debugf("started process %s pid=%d", binary, cmd.Process.Pid)
	return &managedProcess{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

func openProcessLogs(stdoutPath, stderrPath string) (*os.File, *os.File, error) {
	if err := os.MkdirAll(filepath.Dir(stdoutPath), defaultCollectDirMode); err != nil {
		return nil, nil, err
	}
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return nil, nil, err
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		stdout.Close()
		return nil, nil, err
	}
	return stdout, stderr, nil
}

func (p *managedProcess) Close() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	waitDone(p.cmd.Process)
	closeFile(p.stdout)
	closeFile(p.stderr)
}

func waitDone(process *os.Process) {
	done := make(chan struct{})
	go func() {
		_, _ = process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = process.Kill()
		<-done
	}
}

func closeFile(file *os.File) {
	if file != nil {
		_ = file.Close()
	}
}

func nodeExporterArgs(target string) []string {
	return []string{"--web.listen-address=" + target, "--log.level=error"}
}

func prometheusArgs(configPath, runDir string, port int) []string {
	return []string{
		"--config.file=" + configPath,
		"--storage.tsdb.path=" + filepath.Join(runDir, "prometheus-data"),
		"--web.listen-address=127.0.0.1:" + strconv.Itoa(port),
		"--log.level=error",
	}
}

func dcgmExporterArgs(target string, interval time.Duration, collectorsPath string) []string {
	args := []string{
		"--address=" + target,
		"--collect-interval=" + strconv.Itoa(int(interval.Milliseconds())),
	}
	if collectorsPath != "" {
		args = append(args, "--collectors="+collectorsPath)
	}
	return args
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
