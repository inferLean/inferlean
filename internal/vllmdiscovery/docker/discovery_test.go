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

func TestCandidatesFromRecordsUsesEnvMetricsPort(t *testing.T) {
	t.Parallel()
	candidates, err := candidatesFromRecords([]containerRecord{{
		id:   "container-1",
		name: "vllm",
		inspected: inspectedContainer{
			RawCommandLine: "vllm serve Qwen/Qwen3.5-0.8B",
			Env:            []string{"VLLM_PORT=9100"},
			Ports: map[string][]portBinding{
				"9100/tcp": {{HostIP: "0.0.0.0", HostPort: "19100"}},
			},
		},
		internalPID: 17,
	}})
	if err != nil {
		t.Fatalf("candidatesFromRecords returned error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if got, want := candidates[0].MetricsEndpoint, "http://127.0.0.1:19100/metrics"; got != want {
		t.Fatalf("MetricsEndpoint = %q, want %q", got, want)
	}
	if got := candidates[0].InternalPID; got != 17 {
		t.Fatalf("InternalPID = %d, want 17", got)
	}
}

func TestPreferredRecordsUsesExactNameBeforeMissingPublishedPort(t *testing.T) {
	t.Parallel()
	records := []containerRecord{
		{
			id:   "old",
			name: "vllm-old",
			inspected: inspectedContainer{
				RawCommandLine: "vllm serve Qwen/Qwen3.5-0.8B --port 9000",
			},
		},
		{
			id:   "exact",
			name: "vllm",
			inspected: inspectedContainer{
				RawCommandLine: "vllm serve Qwen/Qwen3.5-0.8B --port 9000",
				Ports: map[string][]portBinding{
					"9000/tcp": {{HostIP: "0.0.0.0", HostPort: "19000"}},
				},
			},
		},
	}

	candidates, err := candidatesFromRecords(preferredRecords(records, "vllm"))
	if err != nil {
		t.Fatalf("candidatesFromRecords returned error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].ContainerID != "exact" {
		t.Fatalf("ContainerID = %q, want exact", candidates[0].ContainerID)
	}
}

func TestCandidatesFromRecordsErrorsWhenPortIsNotPublished(t *testing.T) {
	t.Parallel()
	_, err := candidatesFromRecords([]containerRecord{{
		id:   "container-1",
		name: "vllm",
		inspected: inspectedContainer{
			RawCommandLine: "vllm serve Qwen/Qwen3.5-0.8B --port 9000",
		},
	}})
	if err == nil {
		t.Fatal("expected missing published port error")
	}
	if !strings.Contains(err.Error(), "docker -p 9000:9000") {
		t.Fatalf("error = %q, want docker publish guidance", err)
	}
}

func TestPublishedHostNormalizesWildcardBinds(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "0.0.0.0", "::"} {
		if got := publishedHost(input); got != "127.0.0.1" {
			t.Fatalf("publishedHost(%q) = %q, want 127.0.0.1", input, got)
		}
	}
	if got := publishedHost("192.168.1.10"); got != "192.168.1.10" {
		t.Fatalf("publishedHost() = %q, want explicit host", got)
	}
}
