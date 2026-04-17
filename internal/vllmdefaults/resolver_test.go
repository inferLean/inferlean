package vllmdefaults

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithDirFillsMissingFromDefaults(t *testing.T) {
	t.Parallel()
	defaultsDir := writeTestDefaults(t)
	out, err := ResolveWithDir(defaultsDir, Input{
		RawCommandLine: "vllm serve --model model-a --max-num-seqs 2048",
		ExplicitArgs: map[string]string{
			"model":        "model-a",
			"max-num-seqs": "2048",
		},
		VLLMVersion: "0.18.9",
	})
	if err != nil {
		t.Fatalf("ResolveWithDir() error = %v", err)
	}
	if out.SelectedTag != "v0.18.0" {
		t.Fatalf("SelectedTag = %q", out.SelectedTag)
	}
	if out.Args["max-num-seqs"] != "2048" {
		t.Fatalf("max-num-seqs overridden, got %q", out.Args["max-num-seqs"])
	}
	if out.Args["gpu-memory-utilization"] != "0.85" {
		t.Fatalf("gpu-memory-utilization = %q", out.Args["gpu-memory-utilization"])
	}
	if out.Args["max-model-len"] != "16384" {
		t.Fatalf("max-model-len = %q", out.Args["max-model-len"])
	}
}

func TestResolveWithDirUsesPositionalModel(t *testing.T) {
	t.Parallel()
	defaultsDir := writeTestDefaults(t)
	out, err := ResolveWithDir(defaultsDir, Input{
		RawCommandLine: "vllm serve model-a",
		ExplicitArgs:   map[string]string{},
		VLLMVersion:    "v0.19.0",
	})
	if err != nil {
		t.Fatalf("ResolveWithDir() error = %v", err)
	}
	if out.SelectedModel != "model-a" {
		t.Fatalf("SelectedModel = %q", out.SelectedModel)
	}
	if out.Args["max-model-len"] != "4096" {
		t.Fatalf("max-model-len = %q", out.Args["max-model-len"])
	}
}

func TestResolveWithDirSelectsHighMemoryProfile(t *testing.T) {
	t.Parallel()
	defaultsDir := writeTestDefaults(t)
	out, err := ResolveWithDir(defaultsDir, Input{
		RawCommandLine: "vllm serve --model model-a",
		ExplicitArgs: map[string]string{
			"model": "model-a",
		},
		VLLMVersion:  "0.19.0",
		GPUMemoryMiB: 81920,
		GPUModel:     "NVIDIA RTX 6000 Ada",
	})
	if err != nil {
		t.Fatalf("ResolveWithDir() error = %v", err)
	}
	if out.SelectedProfile != "cuda_high_memory_non_a100" {
		t.Fatalf("SelectedProfile = %q", out.SelectedProfile)
	}
	if out.Args["max-num-seqs"] != "1024" {
		t.Fatalf("max-num-seqs = %q", out.Args["max-num-seqs"])
	}
}

func TestResolveWithDirDoesNotAutoInjectModelWhenUnknown(t *testing.T) {
	t.Parallel()
	defaultsDir := writeTestDefaults(t)
	out, err := ResolveWithDir(defaultsDir, Input{
		RawCommandLine: "python -m vllm.entrypoints.openai.api_server --port 8000",
		ExplicitArgs: map[string]string{
			"port": "8000",
		},
		VLLMVersion: "0.19.0",
	})
	if err != nil {
		t.Fatalf("ResolveWithDir() error = %v", err)
	}
	if out.Args["model"] != "" {
		t.Fatalf("model unexpectedly injected: %q", out.Args["model"])
	}
}

func writeTestDefaults(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifestDir := filepath.Join(root, "simple_cuda_by_tag")
	tagDir := filepath.Join(manifestDir, "tags")
	if err := os.MkdirAll(tagDir, 0o755); err != nil {
		t.Fatalf("mkdir defaults tree: %v", err)
	}

	manifest := map[string]any{
		"generator": map[string]any{
			"processed_tags": []string{"v0.18.0", "v0.19.0"},
		},
	}
	writeJSON(t, filepath.Join(manifestDir, "manifest.json"), manifest)

	v018 := map[string]any{
		"profiles": map[string]any{
			"cuda_default": map[string]any{
				"resolved": map[string]any{
					"--gpu-memory-utilization": 0.85,
					"--max-num-seqs":           256,
					"--model":                  "default-model",
				},
				"models": map[string]any{
					"model-a": map[string]any{
						"resolved": map[string]any{
							"--max-model-len": 16384,
						},
					},
				},
			},
		},
	}
	writeJSON(t, filepath.Join(tagDir, "v0.18.0.json"), v018)

	v019 := map[string]any{
		"profiles": map[string]any{
			"cuda_default": map[string]any{
				"resolved": map[string]any{
					"--gpu-memory-utilization": 0.9,
					"--max-num-seqs":           256,
					"--model":                  "default-model",
				},
				"models": map[string]any{
					"model-a": map[string]any{
						"resolved": map[string]any{
							"--max-model-len": 4096,
						},
					},
				},
			},
			"cuda_high_memory_non_a100": map[string]any{
				"resolved": map[string]any{
					"--max-num-seqs": 1024,
				},
				"models": map[string]any{
					"model-a": map[string]any{
						"resolved": map[string]any{
							"--max-model-len": 4096,
						},
					},
				},
			},
		},
	}
	writeJSON(t, filepath.Join(tagDir, "v0.19.0.json"), v019)
	return root
}

func writeJSON(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write json %s: %v", path, err)
	}
}
