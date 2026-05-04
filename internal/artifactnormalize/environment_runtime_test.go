package artifactnormalize

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestNormalizeRuntimeConfigCarriesValueSources(t *testing.T) {
	input := Input{
		Configurations: types.Configurations{
			ParsedArgs: map[string]string{
				"max-model-len":          "8192",
				"max-num-batched-tokens": "2048",
				"max-num-seqs":           "64",
				"gpu-memory-utilization": "0.9",
				"enable-prefix-caching":  "false",
				"enable-chunked-prefill": "true",
			},
			ParsedArgSources: map[string]string{
				"max-model-len":          "effective_engine_config.derived_model_defaults.max_model_len",
				"max-num-batched-tokens": "explicit",
				"max-num-seqs":           "effective_engine_config.scheduler_config.max_num_seqs",
				"gpu-memory-utilization": "effective_engine_config.cache_config.gpu_memory_utilization",
				"enable-prefix-caching":  "effective_engine_config.cache_config.enable_prefix_caching",
				"enable-chunked-prefill": "explicit",
			},
		},
	}

	runtime := normalizeRuntimeConfig(input)

	if got, want := runtime.ValueSources["max_model_len"], "effective_engine_config.derived_model_defaults.max_model_len"; got != want {
		t.Fatalf("max_model_len source = %q, want %q", got, want)
	}
	if got, want := runtime.ValueSources["max_num_batched_tokens"], "explicit"; got != want {
		t.Fatalf("max_num_batched_tokens source = %q, want %q", got, want)
	}
	if got, want := runtime.ValueSources["prefix_caching"], "effective_engine_config.cache_config.enable_prefix_caching"; got != want {
		t.Fatalf("prefix_caching source = %q, want %q", got, want)
	}
	if got, want := runtime.ValueSources["chunked_prefill"], "explicit"; got != want {
		t.Fatalf("chunked_prefill source = %q, want %q", got, want)
	}
}
