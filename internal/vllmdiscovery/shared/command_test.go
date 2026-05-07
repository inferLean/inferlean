package shared

import "testing"

func TestIsServeCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "vllm serve",
			command: "vllm serve meta-llama/Llama-3.1-8B --port 8000",
			want:    true,
		},
		{
			name:    "python api_server module",
			command: "python -m vllm.entrypoints.openai.api_server --model qwen",
			want:    true,
		},
		{
			name:    "bench command",
			command: "python -m vllm.entrypoints.cli.main bench serve --backend vllm",
			want:    false,
		},
		{
			name:    "generic api server",
			command: "python /srv/api_server.py",
			want:    false,
		},
		{
			name:    "empty",
			command: "",
			want:    false,
		},
		{
			name:    "irrelevant process",
			command: "node /opt/service/index.js",
			want:    false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsServeCommand(tc.command)
			if got != tc.want {
				t.Fatalf("IsServeCommand(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestIsVLLMImage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		image string
		want  bool
	}{
		{image: "vllm/vllm-openai:latest", want: true},
		{image: "ghcr.io/acme/llm-service:latest", want: false},
		{image: "", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.image, func(t *testing.T) {
			t.Parallel()
			if got := IsVLLMImage(tc.image); got != tc.want {
				t.Fatalf("IsVLLMImage(%q) = %v, want %v", tc.image, got, tc.want)
			}
		})
	}
}

func TestInferMetricsPort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		command string
		env     []string
		want    int
	}{
		{
			name:    "port flag with space",
			command: "vllm serve model-a --port 9000",
			want:    9000,
		},
		{
			name:    "port flag with equals",
			command: "python -m vllm.entrypoints.openai.api_server --port=9100",
			want:    9100,
		},
		{
			name: "vllm port env",
			env:  []string{"VLLM_PORT=9200"},
			want: 9200,
		},
		{
			name: "default",
			want: DefaultMetricsPort,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := InferMetricsPort(tc.command, tc.env); got != tc.want {
				t.Fatalf("InferMetricsPort() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestInferMetricsHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		command string
		env     []string
		want    string
	}{
		{
			name:    "host flag with space",
			command: "vllm serve model-a --host 192.168.1.20",
			want:    "192.168.1.20",
		},
		{
			name:    "host flag with equals",
			command: "python -m vllm.entrypoints.openai.api_server --host=::1",
			want:    "::1",
		},
		{
			name:    "wildcard ipv4",
			command: "vllm serve model-a --host 0.0.0.0",
			want:    "127.0.0.1",
		},
		{
			name:    "wildcard ipv6",
			command: "vllm serve model-a --host ::",
			want:    "::1",
		},
		{
			name: "vllm host env",
			env:  []string{"VLLM_HOST=10.0.0.8"},
			want: "10.0.0.8",
		},
		{
			name: "default",
			want: "127.0.0.1",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := InferMetricsHost(tc.command, tc.env); got != tc.want {
				t.Fatalf("InferMetricsHost() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInferMetricsEndpoint(t *testing.T) {
	t.Parallel()
	got := InferMetricsEndpoint("vllm serve model-a --host 0.0.0.0 --port 9100", nil)
	if got != "http://127.0.0.1:9100/metrics" {
		t.Fatalf("InferMetricsEndpoint() = %q", got)
	}
}
