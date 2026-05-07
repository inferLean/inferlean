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

func TestPublishedMetricsEndpointUsesPublishedPort(t *testing.T) {
	t.Parallel()
	payload := []byte(`[{
		"Config": {
			"Entrypoint": ["vllm"],
			"Cmd": ["serve", "--port", "9000"],
			"Env": ["IGNORED=1"]
		},
		"NetworkSettings": {
			"Ports": {
				"9000/tcp": [{"HostIp": "0.0.0.0", "HostPort": "19000"}]
			}
		},
		"State": {"Pid": 4321}
	}]`)
	inspected, err := parseInspectContainer(payload)
	if err != nil {
		t.Fatalf("parseInspectContainer returned error: %v", err)
	}
	endpoint, ok := publishedMetricsEndpoint(inspected.Ports, 9000)
	if !ok {
		t.Fatal("expected published endpoint")
	}
	if endpoint != "http://127.0.0.1:19000/metrics" {
		t.Fatalf("endpoint = %q, want published host endpoint", endpoint)
	}
}
