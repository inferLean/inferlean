package vllmdefaults

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFromDumpAppliesEffectiveDefaults(t *testing.T) {
	t.Parallel()

	out, err := resolveFromDump(
		Input{
			RawCommandLine: "vllm serve --model model-a --max-num-seqs 2048",
			ExplicitArgs: map[string]string{
				"model":        "model-a",
				"max-num-seqs": "2048",
			},
		},
		runtimeDumpFile{
			EffectiveServeParameters: map[string]any{
				"served_model_name":      "served-a",
				"tensor_parallel_size":   2,
				"data_parallel_size":     1,
				"pipeline_parallel_size": 1,
				"max_num_seqs":           512,
				"max_model_len":          16384,
				"gpu_memory_utilization": 0.9,
				"kv_cache_dtype":         "auto",
				"enable_prefix_caching":  true,
				"quantization":           "none",
				"dtype":                  "bfloat16",
				"attention_backend":      "default",
				"_sources":               map[string]any{"max_model_len": "x"},
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDump() error = %v", err)
	}
	if out.Args["max-num-seqs"] != "2048" {
		t.Fatalf("max-num-seqs overridden, got %q", out.Args["max-num-seqs"])
	}
	if out.Args["max-model-len"] != "16384" {
		t.Fatalf("max-model-len = %q", out.Args["max-model-len"])
	}
	if out.Args["gpu-memory-utilization"] != "0.9" {
		t.Fatalf("gpu-memory-utilization = %q", out.Args["gpu-memory-utilization"])
	}
	if out.Args["served-model-name"] != "served-a" {
		t.Fatalf("served-model-name = %q", out.Args["served-model-name"])
	}
	if out.Args["tensor-parallel-size"] != "2" {
		t.Fatalf("tensor-parallel-size = %q", out.Args["tensor-parallel-size"])
	}
	if out.Args["enable-prefix-caching"] != "true" {
		t.Fatalf("enable-prefix-caching = %q", out.Args["enable-prefix-caching"])
	}
	if out.Args["dtype"] != "bfloat16" {
		t.Fatalf("dtype = %q", out.Args["dtype"])
	}
	if out.Args["attention-backend"] != "default" {
		t.Fatalf("attention-backend = %q", out.Args["attention-backend"])
	}
	if out.AppliedDefaults != 11 {
		t.Fatalf("AppliedDefaults = %d, want 11", out.AppliedDefaults)
	}
}

func TestResolveFromDumpRequiresEffectiveServeParameters(t *testing.T) {
	t.Parallel()
	_, err := resolveFromDump(Input{}, runtimeDumpFile{})
	if err == nil {
		t.Fatal("resolveFromDump() expected error")
	}
}

func TestFindDumpScriptUnderRootFindsCLIScript(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	scriptPath := filepath.Join(root, "scripts", "dump_vllm_defaults.py")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir script dir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env python\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := findDumpScriptUnderRoot(filepath.Join(root, "cmd", "inferlean"))
	if got != scriptPath {
		t.Fatalf("findDumpScriptUnderRoot() = %q, want %q", got, scriptPath)
	}
}
