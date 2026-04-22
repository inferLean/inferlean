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
				"max_num_seqs":           512,
				"max_model_len":          16384,
				"gpu_memory_utilization": 0.9,
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
	if out.AppliedDefaults != 2 {
		t.Fatalf("AppliedDefaults = %d, want 2", out.AppliedDefaults)
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
