package docker

import (
	"strings"
	"testing"
)

func TestParseInspectContainerIncludesPID(t *testing.T) {
	t.Parallel()
	payload := []byte(`[{
		"Config": {
			"Entrypoint": ["vllm"],
			"Cmd": ["serve", "--model", "Qwen/Qwen3.5-0.8B"]
		},
		"Path": "ignored",
		"Args": ["ignored"],
		"State": {"Pid": 4321}
	}]`)
	got, err := parseInspectContainer(payload)
	if err != nil {
		t.Fatalf("parseInspectContainer returned error: %v", err)
	}
	if got.PID != 4321 {
		t.Fatalf("PID = %d, want %d", got.PID, 4321)
	}
	if !strings.HasPrefix(got.RawCommandLine, "vllm serve") {
		t.Fatalf("expected vllm serve prefix, got %q", got.RawCommandLine)
	}
}
