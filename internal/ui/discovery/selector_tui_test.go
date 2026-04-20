package discovery

import (
	"strings"
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

func TestTargetLabelDockerUsesContainerNameWithoutPID(t *testing.T) {
	t.Parallel()
	label := targetLabel(vllmdiscovery.Candidate{
		Source:      "docker",
		PID:         1234,
		ContainerID: "abc123",
		Executable:  "docker-container:my-vllm",
	})
	if !strings.Contains(label, "container=my-vllm") {
		t.Fatalf("docker label %q missing container name", label)
	}
	if strings.Contains(label, "pid=") {
		t.Fatalf("docker label %q should not include pid", label)
	}
}

func TestTargetLabelProcessKeepsPID(t *testing.T) {
	t.Parallel()
	label := targetLabel(vllmdiscovery.Candidate{
		Source: "process",
		PID:    42,
	})
	if !strings.Contains(label, "pid=42") {
		t.Fatalf("process label %q missing pid", label)
	}
}
