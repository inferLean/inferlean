package collect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

func TestApplyVLLMDefaultsStoresRuntimeWarningsHint(t *testing.T) {
	dir := t.TempDir()
	dumpScript := filepath.Join(dir, "dump_vllm_defaults.py")
	if err := os.WriteFile(dumpScript, []byte("# fake script\n"), 0o644); err != nil {
		t.Fatalf("write dump script: %v", err)
	}
	fakePython := filepath.Join(dir, "python3")
	script := `#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--out" ]; then
    shift
    out="$1"
  fi
  shift
done
cat > "$out" <<'JSON'
{
  "metadata": {"vllm_version": "0.11.0", "torch_version": "2.8.0"},
  "effective_serve_parameters": {
    "_effective_mode": "fallback",
    "model": "google/gemma-4-26B-A4B-it",
    "served_model_name": "google/gemma-4-26B-A4B-it",
    "_sources": {
      "model": "parsed_cli_from_input.model_tag",
      "served_model_name": "effective_serve_parameters.model"
    }
  },
  "warnings": {
    "effective.derive_model_defaults": "Failed to recover effective config in fallback mode: ConnectError('[Errno -3] Temporary failure in name resolution')",
    "effective.create_engine_config": "create_engine_config fallback: ConnectError('[Errno -3] Temporary failure in name resolution')"
  },
  "errors": {
    "input_cli_args_parse": "RuntimeError('local-only cache miss')"
  }
}
JSON
`
	if err := os.WriteFile(fakePython, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake python: %v", err)
	}
	t.Setenv("INFERLEAN_VLLM_DEFAULTS_SCRIPT", dumpScript)
	t.Setenv("INFERLEAN_VLLM_DEFAULTS_DIR", writeRuntimeDefaultsTestData(t, dir))

	rawCommandLine := fakePython + " -m vllm serve google/gemma-4-26B-A4B-it"
	cfg := types.Configurations{
		ParsedArgs: map[string]string{},
	}
	applyVLLMDefaults(
		context.Background(),
		&cfg,
		vllmdiscovery.Candidate{
			Source:         "process",
			PID:            int32(os.Getpid()),
			RawCommandLine: rawCommandLine,
		},
		dir,
		rawCommandLine,
		"",
		"",
		gpuSnapshot{},
		"/home/bale1/.cache/huggingface/hub/models--google--gemma-4-26B-A4B-it/snapshots/47b6801b24d15ff9bcd8c96dfaea0be9ed3a0301",
	)

	wantWarnings := "defaults.generated_fallback=used generated vLLM defaults because runtime effective config mode was fallback; effective.create_engine_config=create_engine_config fallback: ConnectError('[Errno -3] Temporary failure in name resolution'); effective.derive_model_defaults=Failed to recover effective config in fallback mode: ConnectError('[Errno -3] Temporary failure in name resolution')"
	if got := cfg.EnvironmentHints["vllm_defaults_warnings"]; got != wantWarnings {
		t.Fatalf("vllm_defaults_warnings = %q, want %q", got, wantWarnings)
	}
	if got := cfg.EnvironmentHints["vllm_defaults_effective_mode"]; got != "fallback" {
		t.Fatalf("vllm_defaults_effective_mode = %q, want fallback", got)
	}
	if got := cfg.EnvironmentHints["vllm_defaults_model_path_override"]; got != "/home/bale1/.cache/huggingface/hub/models--google--gemma-4-26B-A4B-it/snapshots/47b6801b24d15ff9bcd8c96dfaea0be9ed3a0301" {
		t.Fatalf("vllm_defaults_model_path_override = %q, want observed snapshot path", got)
	}
	if got := cfg.EnvironmentHints["vllm_defaults_applied"]; got != "2" {
		t.Fatalf("vllm_defaults_applied = %q, want 2", got)
	}
	if got := cfg.EnvironmentHints["vllm_defaults_runtime_errors"]; got != "input_cli_args_parse=RuntimeError('local-only cache miss')" {
		t.Fatalf("vllm_defaults_runtime_errors = %q, want input CLI parse failure", got)
	}
	if got := cfg.EnvironmentHints["vllm_model"]; got != "google/gemma-4-26B-A4B-it" {
		t.Fatalf("vllm_model = %q, want google/gemma-4-26B-A4B-it", got)
	}
	if got := cfg.ParsedArgs["max-num-seqs"]; got != "256" {
		t.Fatalf("max-num-seqs = %q, want generated fallback default", got)
	}
	if got := cfg.ParsedArgSources["max-num-seqs"]; got != "profile_default" {
		t.Fatalf("max-num-seqs source = %q, want profile_default", got)
	}
}

func TestObservedVLLMModelPathPrefersSnapshotPath(t *testing.T) {
	t.Parallel()

	promRes := promcollector.Result{
		Samples: map[string][]promcollector.Sample{
			"vllm": {
				{
					Timestamp: time.Unix(1, 0),
					Metrics: []promcollector.MetricPoint{
						{
							Name:   "vllm:prompt_tokens_total",
							Labels: map[string]string{"model_name": "google/gemma"},
							Value:  1,
						},
						{
							Name:   "vllm:request_success_total",
							Labels: map[string]string{"model_name": "/models/gemma"},
							Value:  1,
						},
						{
							Name: "vllm:prompt_tokens_cached_total",
							Labels: map[string]string{
								"model_name": "/home/bale1/.cache/huggingface/hub/models--google--gemma/snapshots/abc",
							},
							Value: 1,
						},
					},
				},
			},
		},
	}

	got := observedVLLMModelPath(promRes)
	want := "/home/bale1/.cache/huggingface/hub/models--google--gemma/snapshots/abc"
	if got != want {
		t.Fatalf("observedVLLMModelPath() = %q, want %q", got, want)
	}
}

func TestObservedVLLMModelPathIgnoresAmbiguousLocalPaths(t *testing.T) {
	t.Parallel()

	promRes := promcollector.Result{
		Samples: map[string][]promcollector.Sample{
			"vllm": {
				{
					Timestamp: time.Unix(1, 0),
					Metrics: []promcollector.MetricPoint{
						{
							Name: "vllm:prompt_tokens_total",
							Labels: map[string]string{
								"model_name": "/cache/models--google--gemma/snapshots/a",
							},
							Value: 1,
						},
						{
							Name: "vllm:generation_tokens_total",
							Labels: map[string]string{
								"model_name": "/cache/models--google--gemma/snapshots/b",
							},
							Value: 1,
						},
					},
				},
			},
		},
	}

	if got := observedVLLMModelPath(promRes); got != "" {
		t.Fatalf("observedVLLMModelPath() = %q, want empty for ambiguous paths", got)
	}
}

func writeRuntimeDefaultsTestData(t *testing.T, root string) string {
	t.Helper()
	defaultsDir := filepath.Join(root, "vllm_defaults")
	tagDir := filepath.Join(defaultsDir, "simple_cuda_by_tag", "tags")
	if err := os.MkdirAll(tagDir, 0o755); err != nil {
		t.Fatalf("mkdir defaults test data: %v", err)
	}
	writeJSONFile(t, filepath.Join(defaultsDir, "simple_cuda_by_tag", "manifest.json"), map[string]any{
		"generator": map[string]any{
			"processed_tags": []string{"v0.11.0"},
		},
	})
	writeJSONFile(t, filepath.Join(tagDir, "v0.11.0.json"), map[string]any{
		"profiles": map[string]any{
			"cuda_default": map[string]any{
				"resolved": map[string]any{
					"--max-num-seqs":           256,
					"--gpu-memory-utilization": 0.9,
				},
			},
		},
	})
	return defaultsDir
}

func writeJSONFile(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal defaults test data: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write defaults test data %s: %v", path, err)
	}
}
