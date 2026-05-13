package vllmdefaults

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
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
				"flashinfer_present":     false,
				"_sources": map[string]any{
					"max_model_len":      "x",
					"flashinfer_present": "runtime_import.flashinfer",
				},
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
	if out.Args["flashinfer-present"] != "false" {
		t.Fatalf("flashinfer-present = %q", out.Args["flashinfer-present"])
	}
	if out.ArgSources["max-num-seqs"] != "explicit" {
		t.Fatalf("max-num-seqs source = %q, want explicit", out.ArgSources["max-num-seqs"])
	}
	if out.ArgSources["max-model-len"] != "x" {
		t.Fatalf("max-model-len source = %q, want x", out.ArgSources["max-model-len"])
	}
	if out.ArgSources["enable-prefix-caching"] == "" {
		t.Fatal("enable-prefix-caching source is empty")
	}
	if out.ArgSources["flashinfer-present"] != "runtime_import.flashinfer" {
		t.Fatalf("flashinfer-present source = %q, want runtime_import.flashinfer", out.ArgSources["flashinfer-present"])
	}
	if out.AppliedDefaults != 12 {
		t.Fatalf("AppliedDefaults = %d, want 12", out.AppliedDefaults)
	}
}

func TestResolveFromDumpRequiresEffectiveServeParameters(t *testing.T) {
	t.Parallel()
	_, err := resolveFromDump(Input{}, runtimeDumpFile{})
	if err == nil {
		t.Fatal("resolveFromDump() expected error")
	}
}

func TestResolveFromDumpSkipsUnknownAttentionBackend(t *testing.T) {
	t.Parallel()

	out, err := resolveFromDump(
		Input{
			RawCommandLine: "vllm serve model-a",
			ExplicitArgs:   map[string]string{},
		},
		runtimeDumpFile{
			EffectiveServeParameters: map[string]any{
				"attention_backend": nil,
				"_sources":          map[string]any{"attention_backend": nil},
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDump() error = %v", err)
	}
	if _, ok := out.Args["attention-backend"]; ok {
		t.Fatalf("attention-backend unexpectedly applied: %q", out.Args["attention-backend"])
	}
	if _, ok := out.ArgSources["attention-backend"]; ok {
		t.Fatalf("attention-backend source unexpectedly applied: %q", out.ArgSources["attention-backend"])
	}
	if out.AppliedDefaults != 0 {
		t.Fatalf("AppliedDefaults = %d, want 0", out.AppliedDefaults)
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

func TestRuntimePIDUsesInternalPID(t *testing.T) {
	t.Parallel()
	pid, err := runtimePID(shared.Candidate{
		Source:      "docker",
		PID:         4321,
		InternalPID: 17,
	}, "docker")
	if err != nil {
		t.Fatalf("runtimePID() error = %v", err)
	}
	if pid != 17 {
		t.Fatalf("runtimePID() = %d, want 17", pid)
	}
}

func TestRuntimePIDAllowsProcessPIDFallback(t *testing.T) {
	t.Parallel()
	pid, err := runtimePID(shared.Candidate{
		Source: "process",
		PID:    4321,
	}, "process")
	if err != nil {
		t.Fatalf("runtimePID() error = %v", err)
	}
	if pid != 4321 {
		t.Fatalf("runtimePID() = %d, want 4321", pid)
	}
}

func TestRuntimePIDRequiresInternalPIDForDocker(t *testing.T) {
	t.Parallel()
	_, err := runtimePID(shared.Candidate{
		Source: "docker",
		PID:    4321,
	}, "docker")
	if err == nil {
		t.Fatal("runtimePID() expected error")
	}
}

func TestPythonCandidatesPreferCommandLinePython(t *testing.T) {
	t.Parallel()
	got := pythonCandidates(shared.Candidate{
		RawCommandLine: "/home/bale1/gemma4/.venv/bin/python3 /home/bale1/gemma4/.venv/bin/vllm serve google/gemma-4-26B-A4B-it --max-model-len 32768",
	}, 331110)
	want := []string{
		"/home/bale1/gemma4/.venv/bin/python3",
		"/proc/331110/exe",
	}
	if len(got) != len(want) {
		t.Fatalf("len(pythonCandidates()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pythonCandidates()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPythonCandidatesFallbackToTargetProcessExe(t *testing.T) {
	t.Parallel()
	got := pythonCandidates(shared.Candidate{}, 17)
	want := []string{"/proc/17/exe"}
	if len(got) != len(want) {
		t.Fatalf("len(pythonCandidates()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pythonCandidates()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPythonCandidatesIgnoreVLLMConsoleScript(t *testing.T) {
	t.Parallel()
	got := pythonCandidates(shared.Candidate{
		RawCommandLine: "/home/bale1/gemma4/.venv/bin/vllm serve google/gemma-4-26B-A4B-it",
	}, 17)
	want := []string{"/proc/17/exe"}
	if len(got) != len(want) {
		t.Fatalf("len(pythonCandidates()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pythonCandidates()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
