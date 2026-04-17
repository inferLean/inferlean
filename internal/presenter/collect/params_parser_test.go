package collect

import "testing"

func TestParseVLLMArgs(t *testing.T) {
	t.Parallel()
	raw := `vllm serve --model "Qwen/Qwen3-VL 8B" --max-model-len 16384 --gpu-memory-utilization=0.90 --enforce-eager`
	got := parseVLLMArgs(raw)
	if got["model"] != "Qwen/Qwen3-VL 8B" {
		t.Fatalf("model = %q", got["model"])
	}
	if got["max-model-len"] != "16384" {
		t.Fatalf("max-model-len = %q", got["max-model-len"])
	}
	if got["gpu-memory-utilization"] != "0.90" {
		t.Fatalf("gpu-memory-utilization = %q", got["gpu-memory-utilization"])
	}
	if got["enforce-eager"] != "true" {
		t.Fatalf("enforce-eager = %q", got["enforce-eager"])
	}
}

func TestParseVLLMArgsHandlesSingleQuotes(t *testing.T) {
	t.Parallel()
	raw := "python -m vllm.entrypoints.openai.api_server --model 'meta llama 3.1' --port 8000"
	got := parseVLLMArgs(raw)
	if got["model"] != "meta llama 3.1" {
		t.Fatalf("model = %q", got["model"])
	}
	if got["port"] != "8000" {
		t.Fatalf("port = %q", got["port"])
	}
}

func TestParseVLLMArgsInfersPositionalModel(t *testing.T) {
	t.Parallel()
	raw := "vllm serve Qwen/Qwen3.5-0.8B --port 8000"
	got := parseVLLMArgs(raw)
	if got["model"] != "Qwen/Qwen3.5-0.8B" {
		t.Fatalf("model = %q", got["model"])
	}
	if got["port"] != "8000" {
		t.Fatalf("port = %q", got["port"])
	}
}
