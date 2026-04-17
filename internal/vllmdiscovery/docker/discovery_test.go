package docker

import (
	"strings"
	"testing"
)

func TestParseInspectRawCommandLine(t *testing.T) {
	t.Parallel()
	payload := []byte(`[{
		"Config": {
			"Entrypoint": ["vllm"],
			"Cmd": ["serve", "--model", "Qwen/Qwen3.5 0.8B", "--port", "8000"]
		},
		"Path": "ignored",
		"Args": ["ignored"]
	}]`)
	got, err := parseInspectRawCommandLine(payload)
	if err != nil {
		t.Fatalf("parseInspectRawCommandLine returned error: %v", err)
	}
	if strings.Contains(got, "...") {
		t.Fatalf("expected full command without truncation, got %q", got)
	}
	if !strings.Contains(got, `"Qwen/Qwen3.5 0.8B"`) {
		t.Fatalf("expected quoted model argument, got %q", got)
	}
	if !strings.HasPrefix(got, "vllm serve") {
		t.Fatalf("expected vllm serve prefix, got %q", got)
	}
}

func TestParseInspectRawCommandLineFallback(t *testing.T) {
	t.Parallel()
	payload := []byte(`[{
		"Config": {
			"Entrypoint": null,
			"Cmd": null
		},
		"Path": "python3",
		"Args": ["-m", "vllm.entrypoints.openai.api_server", "--host", "0.0.0.0"]
	}]`)
	got, err := parseInspectRawCommandLine(payload)
	if err != nil {
		t.Fatalf("parseInspectRawCommandLine returned error: %v", err)
	}
	if !strings.Contains(got, "vllm.entrypoints.openai.api_server") {
		t.Fatalf("expected path/args fallback, got %q", got)
	}
}

func TestParseInspectRawCommandLineInvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := parseInspectRawCommandLine([]byte(`{not-json`))
	if err == nil {
		t.Fatal("expected error for invalid inspect JSON")
	}
}

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
