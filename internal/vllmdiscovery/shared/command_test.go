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
