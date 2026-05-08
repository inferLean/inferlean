package artifactnormalize

import (
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
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

func TestNormalizeRuntimeConfigCapturesQuantizationPosture(t *testing.T) {
	input := Input{
		Configurations: types.Configurations{
			ParsedArgs: map[string]string{
				"model":          "Qwen/Qwen3-32B-FP8",
				"quantization":   "fp8",
				"dtype":          "bfloat16",
				"kv-cache-dtype": "fp8",
			},
		},
	}

	runtime := normalizeRuntimeConfig(input)

	if got, want := runtime.Model, "Qwen/Qwen3-32B-FP8"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
	if got, want := runtime.Quantization, "fp8"; got != want {
		t.Fatalf("quantization = %q, want %q", got, want)
	}
	if got, want := runtime.DType, "bfloat16"; got != want {
		t.Fatalf("dtype = %q, want %q", got, want)
	}
	if got, want := runtime.KVCacheDType, "fp8"; got != want {
		t.Fatalf("kv_cache_dtype = %q, want %q", got, want)
	}
}

func TestNormalizeRuntimeConfigPrefersExplicitRuntimeHostPort(t *testing.T) {
	input := Input{
		Target: TargetInput{
			MetricsEndpoint: "http://127.0.0.1:19000/metrics",
		},
		Configurations: types.Configurations{
			ParsedArgs: map[string]string{
				"host": "0.0.0.0",
				"port": "9000",
			},
		},
	}

	runtime := normalizeRuntimeConfig(input)

	if got, want := runtime.Host, "0.0.0.0"; got != want {
		t.Fatalf("host = %q, want %q", got, want)
	}
	if got, want := runtime.Port, 9000; got != want {
		t.Fatalf("port = %d, want %d", got, want)
	}
}

func TestNormalizeRuntimeConfigPrefersLivePrefixCachingConfig(t *testing.T) {
	input := Input{
		Configurations: types.Configurations{
			ParsedArgs: map[string]string{
				"enable-prefix-caching": "true",
			},
			ParsedArgSources: map[string]string{
				"enable-prefix-caching": "effective_engine_config.cache_config.enable_prefix_caching",
			},
		},
		Observations: ObservationsInput{
			Prometheus: map[string][]promcollector.Sample{
				"vllm": {
					{
						Timestamp: time.Unix(10, 0).UTC(),
						Metrics: []promcollector.MetricPoint{
							{
								Name: "vllm:cache_config_info",
								Labels: map[string]string{
									"enable_prefix_caching": "False",
								},
								Value: 1,
							},
						},
					},
				},
			},
		},
	}

	runtime := normalizeRuntimeConfig(input)

	if runtime.PrefixCaching == nil {
		t.Fatal("prefix caching is nil")
	}
	if *runtime.PrefixCaching {
		t.Fatal("prefix caching = true, want false from live vLLM cache config")
	}
	if got, want := runtime.ValueSources["prefix_caching"], "metrics.vllm.cache_config_info.enable_prefix_caching"; got != want {
		t.Fatalf("prefix_caching source = %q, want %q", got, want)
	}
}

func TestNormalizeRuntimeConfigCapturesSchedulerAndCacheDetails(t *testing.T) {
	input := Input{
		Configurations: types.Configurations{
			ParsedArgs: map[string]string{
				"async-scheduling":             "true",
				"scheduler-policy":             "fcfs",
				"max-num-partial-prefills":     "2",
				"max-long-partial-prefills":    "1",
				"long-prefill-token-threshold": "256",
				"disable-chunked-mm-input":     "false",
				"kv-offloading-backend":        "native",
				"kv-sharing-fast-prefill":      "true",
				"prefix-caching-hash-algo":     "sha256",
			},
		},
		Observations: ObservationsInput{
			Prometheus: map[string][]promcollector.Sample{
				"vllm": {
					{
						Timestamp: time.Unix(10, 0).UTC(),
						Metrics: []promcollector.MetricPoint{
							{
								Name: "vllm:cache_config_info",
								Labels: map[string]string{
									"block_size":               "16",
									"cache_dtype":              "auto",
									"num_gpu_blocks":           "351",
									"num_cpu_blocks":           "8",
									"enable_prefix_caching":    "False",
									"gpu_memory_utilization":   "0.95",
									"kv_offloading_backend":    "native",
									"kv_sharing_fast_prefill":  "True",
									"prefix_caching_hash_algo": "sha256",
								},
								Value: 1,
							},
						},
					},
				},
			},
		},
	}

	runtime := normalizeRuntimeConfig(input)

	if runtime.Scheduler.AsyncScheduling == nil || !*runtime.Scheduler.AsyncScheduling {
		t.Fatalf("async_scheduling = %v, want true", runtime.Scheduler.AsyncScheduling)
	}
	if got, want := runtime.Scheduler.MaxNumPartialPrefills, 2; got != want {
		t.Fatalf("max_num_partial_prefills = %d, want %d", got, want)
	}
	if got, want := runtime.Cache.NumGPUBlocks, 351; got != want {
		t.Fatalf("num_gpu_blocks = %d, want %d", got, want)
	}
	if got, want := runtime.Cache.BlockSize, 16; got != want {
		t.Fatalf("block_size = %d, want %d", got, want)
	}
	if runtime.Cache.KVSharingFastPrefill == nil || !*runtime.Cache.KVSharingFastPrefill {
		t.Fatalf("kv_sharing_fast_prefill = %v, want true", runtime.Cache.KVSharingFastPrefill)
	}
}

func TestNormalizeRuntimeConfigUsesNoImageProcessorForNonMultimodalRuns(t *testing.T) {
	runtime := normalizeRuntimeConfig(Input{})

	if got, want := runtime.ImageProcessor, "none"; got != want {
		t.Fatalf("image_processor = %q, want %q", got, want)
	}
}

func TestNormalizeRuntimeConfigCarriesFlashinferPresence(t *testing.T) {
	input := Input{
		Configurations: types.Configurations{
			ParsedArgs: map[string]string{
				"flashinfer-present": "false",
			},
			ParsedArgSources: map[string]string{
				"flashinfer-present": "runtime_import.flashinfer",
			},
		},
	}

	runtime := normalizeRuntimeConfig(input)

	if runtime.FlashinferPresent == nil {
		t.Fatal("flashinfer_present is nil")
	}
	if *runtime.FlashinferPresent {
		t.Fatal("flashinfer_present = true, want false")
	}
	if got, want := runtime.ValueSources["flashinfer_presence"], "runtime_import.flashinfer"; got != want {
		t.Fatalf("flashinfer_presence source = %q, want %q", got, want)
	}
}
