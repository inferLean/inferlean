package vllmdefaults

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
				"_effective_mode":        "full_vllm_config",
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
	if out.RuntimeEffectiveMode != "full_vllm_config" {
		t.Fatalf("RuntimeEffectiveMode = %q, want full_vllm_config", out.RuntimeEffectiveMode)
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

func TestResolveFromDumpDoesNotApplyFallbackEffectiveParameters(t *testing.T) {
	t.Parallel()

	out, err := resolveFromDump(
		Input{
			RawCommandLine: "vllm serve model-a",
			ExplicitArgs:   map[string]string{},
		},
		runtimeDumpFile{
			EffectiveServeParameters: map[string]any{
				"_effective_mode":          "fallback",
				"model":                    "model-a",
				"max_num_batched_tokens":   8192,
				"enable_chunked_prefill":   true,
				"gpu_memory_utilization":   0.9,
				"tensor_parallel_size":     1,
				"served_model_name":        "model-a",
				"flashinfer_present":       false,
				"attention_backend":        "FLASH_ATTN",
				"prefix_caching_hash_algo": "sha256",
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDump() error = %v", err)
	}
	if out.AppliedDefaults != 0 {
		t.Fatalf("AppliedDefaults = %d, want 0", out.AppliedDefaults)
	}
	if len(out.Args) != 0 {
		t.Fatalf("Args = %#v, want no applied defaults", out.Args)
	}
	if out.RuntimeEffectiveMode != "fallback" {
		t.Fatalf("RuntimeEffectiveMode = %q, want fallback", out.RuntimeEffectiveMode)
	}
	if out.SelectedModel != "model-a" {
		t.Fatalf("SelectedModel = %q, want model-a", out.SelectedModel)
	}
}

func TestResolveFromDumpRestoresModelLabelsWhenModelPathOverrideWasUsed(t *testing.T) {
	t.Parallel()

	out, err := resolveFromDump(
		Input{
			RawCommandLine: "vllm serve google/gemma",
			ExplicitArgs:   map[string]string{},
		},
		runtimeDumpFile{
			PIDProcess: runtimePIDProcess{
				ModelPathOverride:        "/cache/models--google--gemma/snapshots/abc",
				ModelPathOverrideApplied: true,
			},
			EffectiveServeParameters: map[string]any{
				"_effective_mode":   "full_vllm_config",
				"model":             "/cache/models--google--gemma/snapshots/abc",
				"served_model_name": "/cache/models--google--gemma/snapshots/abc",
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDump() error = %v", err)
	}
	if out.Args["model"] != "google/gemma" {
		t.Fatalf("model = %q, want original repo id", out.Args["model"])
	}
	if out.Args["served-model-name"] != "google/gemma" {
		t.Fatalf("served-model-name = %q, want original repo id", out.Args["served-model-name"])
	}
}

func TestResolveFromDumpWithGeneratedFallbackUsesStaticDefaultsAndKeepsRuntimeFailures(t *testing.T) {
	t.Parallel()

	warnings := map[string]string{
		"effective.create_engine_config": "create_engine_config fallback: ConnectError('[Errno -3] Temporary failure in name resolution')",
	}
	errors := map[string]string{
		"input_cli_args_parse": "RuntimeError('local-only cache miss')",
	}
	out, err := resolveFromDumpWithGeneratedFallback(
		Input{
			RawCommandLine: "vllm serve model-a",
			ExplicitArgs:   map[string]string{},
			VLLMVersion:    "0.18.0",
		},
		runtimeDumpFile{
			Metadata: struct {
				VLLMVersion  string `json:"vllm_version"`
				TorchVersion string `json:"torch_version"`
			}{VLLMVersion: "0.19.0"},
			EffectiveServeParameters: map[string]any{
				"_effective_mode": "unavailable",
				"model":           "model-a",
			},
		},
		warnings,
		errors,
		func(input Input) (Output, error) {
			if input.VLLMVersion != "0.19.0" {
				t.Fatalf("fallback VLLMVersion = %q, want runtime metadata version", input.VLLMVersion)
			}
			return Output{
				Args: map[string]string{
					"max-num-seqs":           "256",
					"gpu-memory-utilization": "0.9",
				},
				ArgSources: map[string]string{
					"max-num-seqs":           "profile_default",
					"gpu-memory-utilization": "profile_default",
				},
				SelectedModel:   "model-a",
				AppliedDefaults: 2,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDumpWithGeneratedFallback() error = %v", err)
	}
	if out.Args["max-num-seqs"] != "256" {
		t.Fatalf("max-num-seqs = %q, want generated fallback default", out.Args["max-num-seqs"])
	}
	if out.AppliedDefaults != 2 {
		t.Fatalf("AppliedDefaults = %d, want 2", out.AppliedDefaults)
	}
	if out.RuntimeEffectiveMode != "unavailable" {
		t.Fatalf("RuntimeEffectiveMode = %q, want unavailable", out.RuntimeEffectiveMode)
	}
	if warnings["effective.create_engine_config"] == "" {
		t.Fatal("runtime warning was not preserved")
	}
	if warnings["defaults.generated_fallback"] == "" {
		t.Fatal("generated fallback warning was not recorded")
	}
	if errors["input_cli_args_parse"] == "" {
		t.Fatal("runtime error was not preserved")
	}
}

func TestResolveFromDumpWithGeneratedFallbackKeepsTrustedRuntimeObservedDefaults(t *testing.T) {
	t.Parallel()

	warnings := map[string]string{}
	errors := map[string]string{}
	out, err := resolveFromDumpWithGeneratedFallback(
		Input{
			RawCommandLine: "vllm serve model-a",
			ExplicitArgs:   map[string]string{},
			VLLMVersion:    "0.20.0",
		},
		runtimeDumpFile{
			EffectiveServeParameters: map[string]any{
				"_effective_mode":          "fallback",
				"model":                    "model-a",
				"flashinfer_present":       true,
				"attention_backend":        "FLASH_ATTN",
				"enable_chunked_prefill":   true,
				"prefix_caching_hash_algo": "sha256",
				"_sources": map[string]any{
					"flashinfer_present":     "runtime_import.flashinfer",
					"attention_backend":      "effective_engine_config.attention_backend",
					"enable_chunked_prefill": "engine_args_from_input.enable_chunked_prefill",
				},
			},
		},
		warnings,
		errors,
		func(input Input) (Output, error) {
			return Output{
				Args: map[string]string{
					"max-num-seqs": "256",
				},
				ArgSources: map[string]string{
					"max-num-seqs": "profile_default",
				},
				SelectedModel:   "model-a",
				AppliedDefaults: 1,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDumpWithGeneratedFallback() error = %v", err)
	}
	if got := out.Args["flashinfer-present"]; got != "true" {
		t.Fatalf("flashinfer-present = %q, want trusted runtime import", got)
	}
	if got := out.ArgSources["flashinfer-present"]; got != "runtime_import.flashinfer" {
		t.Fatalf("flashinfer-present source = %q, want runtime_import.flashinfer", got)
	}
	if _, ok := out.Args["attention-backend"]; ok {
		t.Fatalf("attention-backend unexpectedly copied from fallback dump: %q", out.Args["attention-backend"])
	}
	if _, ok := out.Args["enable-chunked-prefill"]; ok {
		t.Fatalf("enable-chunked-prefill unexpectedly copied from fallback dump: %q", out.Args["enable-chunked-prefill"])
	}
	if out.AppliedDefaults != 2 {
		t.Fatalf("AppliedDefaults = %d, want generated default plus trusted runtime value", out.AppliedDefaults)
	}
}

func TestResolveFromDumpWithGeneratedFallbackReportsFallbackFailure(t *testing.T) {
	t.Parallel()

	warnings := map[string]string{}
	errors := map[string]string{
		"input_cli_args_parse": "RuntimeError('local-only cache miss')",
	}
	out, err := resolveFromDumpWithGeneratedFallback(
		Input{
			RawCommandLine: "vllm serve model-a",
			ExplicitArgs:   map[string]string{},
		},
		runtimeDumpFile{
			EffectiveServeParameters: map[string]any{
				"_effective_mode":    "unavailable",
				"flashinfer_present": false,
				"_sources": map[string]any{
					"flashinfer_present": "runtime_import.flashinfer",
				},
			},
		},
		warnings,
		errors,
		func(input Input) (Output, error) {
			return Output{}, os.ErrNotExist
		},
	)
	if err != nil {
		t.Fatalf("resolveFromDumpWithGeneratedFallback() error = %v", err)
	}
	if out.RuntimeEffectiveMode != "unavailable" {
		t.Fatalf("RuntimeEffectiveMode = %q, want unavailable", out.RuntimeEffectiveMode)
	}
	if errors["input_cli_args_parse"] == "" {
		t.Fatal("runtime error was not preserved")
	}
	if errors["defaults.generated_fallback"] == "" {
		t.Fatal("generated fallback failure was not reported")
	}
	if got := out.Args["flashinfer-present"]; got != "false" {
		t.Fatalf("flashinfer-present = %q, want trusted runtime value", got)
	}
}

func TestFlattenStatusMapFormatsRuntimeWarningsDeterministically(t *testing.T) {
	t.Parallel()

	got := flattenStatusMap(map[string]string{
		"effective.derive_model_defaults": "Failed to recover effective config in fallback mode: ConnectError('[Errno -3] Temporary failure in name resolution')",
		"empty":                           " ",
		"effective.create_engine_config":  "create_engine_config fallback: ConnectError('[Errno -3] Temporary failure in name resolution')",
	})
	want := "effective.create_engine_config=create_engine_config fallback: ConnectError('[Errno -3] Temporary failure in name resolution'); effective.derive_model_defaults=Failed to recover effective config in fallback mode: ConnectError('[Errno -3] Temporary failure in name resolution')"
	if got != want {
		t.Fatalf("flattenStatusMap() = %q, want %q", got, want)
	}
}

func TestResolveFromRuntimeFallsBackToGeneratedDefaultsWhenScriptExecutionFails(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "dump_vllm_defaults.py")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env python\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv(defaultsScriptEnv, scriptPath)
	t.Setenv(defaultsDirEnv, writeTestDefaults(t))

	out, err := ResolveFromRuntime(context.Background(), RuntimeInput{
		Input: Input{
			RawCommandLine: "vllm serve model-a",
			VLLMVersion:    "0.19.0",
		},
		Target: shared.Candidate{
			Source: "process",
			PID:    999999,
		},
		DumpPath: filepath.Join(dir, "dump.json"),
	})
	if err != nil {
		t.Fatalf("ResolveFromRuntime() error = %v", err)
	}
	if got := out.Args["max-num-seqs"]; got != "256" {
		t.Fatalf("max-num-seqs = %q, want generated fallback default", got)
	}
	if got := out.RuntimeEffectiveMode; got != "unavailable" {
		t.Fatalf("RuntimeEffectiveMode = %q, want unavailable", got)
	}
	if !strings.Contains(out.RuntimeErrors, "runtime_dump.execute=") {
		t.Fatalf("RuntimeErrors = %q, want runtime_dump.execute", out.RuntimeErrors)
	}
	if !strings.Contains(out.RuntimeWarnings, "defaults.generated_fallback=") {
		t.Fatalf("RuntimeWarnings = %q, want generated fallback warning", out.RuntimeWarnings)
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

func TestDumpScriptArgsIncludesModelPathOverride(t *testing.T) {
	t.Parallel()

	got := dumpScriptArgs(17, "/tmp/dump.json", " /models/snapshot ")
	want := []string{
		"--pid",
		"17",
		"--out",
		"/tmp/dump.json",
		"--model-path-override",
		"/models/snapshot",
		"--effective-timeout-seconds",
		"20",
	}
	if len(got) != len(want) {
		t.Fatalf("len(dumpScriptArgs()) = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dumpScriptArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRuntimeTimeoutBudgetAllowsPythonFallbackPaths(t *testing.T) {
	t.Parallel()

	minimum := time.Duration(runtimeDefaultsEffectiveTimeoutSeconds*3+15) * time.Second
	if runtimeDefaultsScriptTimeout < minimum {
		t.Fatalf("runtimeDefaultsScriptTimeout = %s, want at least %s", runtimeDefaultsScriptTimeout, minimum)
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
