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
