package parse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inferLean/inferlean/internal/discovery/process"
)

type fixture struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
	Want struct {
		Matched            bool   `json:"matched"`
		EntryPoint         string `json:"entry_point"`
		Model              string `json:"model"`
		Host               string `json:"host"`
		Port               int    `json:"port"`
		PortDefaulted      bool   `json:"port_defaulted"`
		TensorParallelSize int    `json:"tensor_parallel_size"`
		MaxNumSeqs         int    `json:"max_num_seqs"`
		Quantization       string `json:"quantization"`
	} `json:"want"`
}

func TestParseFixtures(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "testdata", "discovery", "parse_cases.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var fixtures []fixture
	if err := json.Unmarshal(contents, &fixtures); err != nil {
		t.Fatalf("unmarshal fixtures: %v", err)
	}

	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			parsed := Parse(process.Snapshot{Args: tc.Args})
			if parsed.Matched != tc.Want.Matched {
				t.Fatalf("matched = %v, want %v", parsed.Matched, tc.Want.Matched)
			}
			if parsed.EntryPoint != tc.Want.EntryPoint {
				t.Fatalf("entrypoint = %q, want %q", parsed.EntryPoint, tc.Want.EntryPoint)
			}
			if parsed.RuntimeConfig.Model != tc.Want.Model {
				t.Fatalf("model = %q, want %q", parsed.RuntimeConfig.Model, tc.Want.Model)
			}
			if parsed.RuntimeConfig.Host != tc.Want.Host {
				t.Fatalf("host = %q, want %q", parsed.RuntimeConfig.Host, tc.Want.Host)
			}
			if parsed.RuntimeConfig.Port != tc.Want.Port {
				t.Fatalf("port = %d, want %d", parsed.RuntimeConfig.Port, tc.Want.Port)
			}
			if parsed.RuntimeConfig.PortDefaulted != tc.Want.PortDefaulted {
				t.Fatalf("port defaulted = %v, want %v", parsed.RuntimeConfig.PortDefaulted, tc.Want.PortDefaulted)
			}
			if parsed.RuntimeConfig.TensorParallelSize != tc.Want.TensorParallelSize {
				t.Fatalf("tensor parallel = %d, want %d", parsed.RuntimeConfig.TensorParallelSize, tc.Want.TensorParallelSize)
			}
			if parsed.RuntimeConfig.MaxNumSeqs != tc.Want.MaxNumSeqs {
				t.Fatalf("max num seqs = %d, want %d", parsed.RuntimeConfig.MaxNumSeqs, tc.Want.MaxNumSeqs)
			}
			if parsed.RuntimeConfig.Quantization != tc.Want.Quantization {
				t.Fatalf("quantization = %q, want %q", parsed.RuntimeConfig.Quantization, tc.Want.Quantization)
			}
		})
	}
}

func TestParseRunPodServeCommand(t *testing.T) {
	t.Parallel()

	parsed := Parse(process.Snapshot{
		Args: []string{
			"/root/.venv/bin/python3",
			"/root/.venv/bin/vllm",
			"serve",
			"Qwen/Qwen3.5-0.8B",
			"--host",
			"0.0.0.0",
			"--port",
			"8000",
			"--api-key",
			"local-bench-key",
			"--dtype",
			"auto",
			"--generation-config",
			"vllm",
		},
	})

	if !parsed.Matched {
		t.Fatal("expected process to match vLLM serve")
	}
	if parsed.EntryPoint != "vllm serve" {
		t.Fatalf("entrypoint = %q, want %q", parsed.EntryPoint, "vllm serve")
	}
	if parsed.RuntimeConfig.Model != "Qwen/Qwen3.5-0.8B" {
		t.Fatalf("model = %q, want %q", parsed.RuntimeConfig.Model, "Qwen/Qwen3.5-0.8B")
	}
	if parsed.RuntimeConfig.Host != "0.0.0.0" {
		t.Fatalf("host = %q, want %q", parsed.RuntimeConfig.Host, "0.0.0.0")
	}
	if parsed.RuntimeConfig.Port != 8000 {
		t.Fatalf("port = %d, want %d", parsed.RuntimeConfig.Port, 8000)
	}
	if !parsed.RuntimeConfig.APIKeyConfigured {
		t.Fatal("expected api key to be marked as configured")
	}
	if parsed.RuntimeConfig.DType != "auto" {
		t.Fatalf("dtype = %q, want %q", parsed.RuntimeConfig.DType, "auto")
	}
	if parsed.RuntimeConfig.GenerationConfig != "vllm" {
		t.Fatalf("generation config = %q, want %q", parsed.RuntimeConfig.GenerationConfig, "vllm")
	}
}

func TestParseModernFlagsAndAliases(t *testing.T) {
	t.Parallel()

	parsed := Parse(process.Snapshot{
		Args: []string{
			"vllm",
			"serve",
			"meta-llama/Llama-3.1-8B-Instruct",
			"-tp",
			"2",
			"-dp",
			"4",
			"-pp",
			"3",
			"-q",
			"awq",
			"--served-model-name",
			"llama-public",
			"llama-alias",
			"--max-model-len",
			"16k",
			"--max-num-batched-tokens",
			"1K",
			"--no-enable-chunked-prefill",
			"--no-enable-prefix-caching",
		},
	})

	if parsed.RuntimeConfig.TensorParallelSize != 2 {
		t.Fatalf("tensor parallel = %d, want %d", parsed.RuntimeConfig.TensorParallelSize, 2)
	}
	if parsed.RuntimeConfig.DataParallelSize != 4 {
		t.Fatalf("data parallel = %d, want %d", parsed.RuntimeConfig.DataParallelSize, 4)
	}
	if parsed.RuntimeConfig.PipelineParallelSize != 3 {
		t.Fatalf("pipeline parallel = %d, want %d", parsed.RuntimeConfig.PipelineParallelSize, 3)
	}
	if parsed.RuntimeConfig.Quantization != "awq" {
		t.Fatalf("quantization = %q, want %q", parsed.RuntimeConfig.Quantization, "awq")
	}
	if parsed.RuntimeConfig.ServedModelName != "llama-public" {
		t.Fatalf("served model name = %q, want %q", parsed.RuntimeConfig.ServedModelName, "llama-public")
	}
	if parsed.RuntimeConfig.MaxModelLen != 16000 {
		t.Fatalf("max model len = %d, want %d", parsed.RuntimeConfig.MaxModelLen, 16000)
	}
	if parsed.RuntimeConfig.MaxNumBatchedTokens != 1024 {
		t.Fatalf("max num batched tokens = %d, want %d", parsed.RuntimeConfig.MaxNumBatchedTokens, 1024)
	}
	if parsed.RuntimeConfig.ChunkedPrefill == nil || *parsed.RuntimeConfig.ChunkedPrefill {
		t.Fatalf("chunked prefill = %v, want disabled", parsed.RuntimeConfig.ChunkedPrefill)
	}
	if parsed.RuntimeConfig.PrefixCaching == nil || *parsed.RuntimeConfig.PrefixCaching {
		t.Fatalf("prefix caching = %v, want disabled", parsed.RuntimeConfig.PrefixCaching)
	}
}

func TestParseMaxModelLenAuto(t *testing.T) {
	t.Parallel()

	parsed := Parse(process.Snapshot{
		Args: []string{
			"vllm",
			"serve",
			"model-a",
			"--max-model-len",
			"-1",
		},
	})

	if parsed.RuntimeConfig.MaxModelLen != -1 {
		t.Fatalf("max model len = %d, want %d", parsed.RuntimeConfig.MaxModelLen, -1)
	}
}
