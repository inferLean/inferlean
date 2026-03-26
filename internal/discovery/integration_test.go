package discovery

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/discovery/process"
)

func TestIntegrationDiscoverSingleProcess(t *testing.T) {
	t.Parallel()

	cmd := startHelperProcess(t, []string{"vllm", "serve", "meta-llama/Llama-3.1-8B-Instruct", "--host", "0.0.0.0", "--port", "8010"})
	defer stopHelperProcess(t, cmd)

	service := NewService(process.SystemInspector{})
	result, err := eventuallyDiscover(t, service, Options{PID: int32(cmd.Process.Pid)})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if result.Selected == nil {
		t.Fatal("expected selected result")
	}
	if result.Selected.PrimaryPID != int32(cmd.Process.Pid) {
		t.Fatalf("selected pid = %d, want %d", result.Selected.PrimaryPID, cmd.Process.Pid)
	}
}

func TestIntegrationAmbiguousProcesses(t *testing.T) {
	t.Parallel()

	cmdA := startHelperProcess(t, []string{"vllm", "serve", "model-a", "--port", "8011"})
	defer stopHelperProcess(t, cmdA)
	cmdB := startHelperProcess(t, []string{"vllm", "serve", "model-b", "--port", "8012"})
	defer stopHelperProcess(t, cmdB)

	service := NewService(process.SystemInspector{})
	_, err := eventuallyDiscover(t, service, Options{})
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("err = %v, want ambiguous", err)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	select {}
}

func startHelperProcess(t *testing.T, args []string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Args = append([]string{"helper", "-test.run=TestHelperProcess", "--"}, args...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	return cmd
}

func stopHelperProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func eventuallyDiscover(t *testing.T, service Service, opts Options) (Result, error) {
	t.Helper()

	var (
		result Result
		err    error
	)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err = service.Discover(context.Background(), opts)
		if err == nil || errors.Is(err, ErrAmbiguous) {
			return result, err
		}
		time.Sleep(150 * time.Millisecond)
	}

	return result, err
}
