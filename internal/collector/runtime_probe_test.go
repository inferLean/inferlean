package collector

import (
	"testing"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestMergeRuntimeProbePrefersPrimaryValues(t *testing.T) {
	primary := runtimeProbeResult{
		VLLMVersion:           "0.8.4",
		TorchVersion:          "2.6.0",
		CUDARuntimeVersion:    "12.4",
		AttentionBackend:      "flash-attn",
		FlashinferPresent:     boolPointer(true),
		FlashAttentionPresent: boolPointer(false),
	}
	fallback := runtimeProbeResult{
		VLLMVersion:           "0.8.1",
		TorchVersion:          "2.5.1",
		CUDARuntimeVersion:    "12.1",
		AttentionBackend:      "xformers",
		FlashinferPresent:     boolPointer(false),
		FlashAttentionPresent: boolPointer(true),
	}

	merged := mergeRuntimeProbe(primary, fallback)

	if merged.VLLMVersion != "0.8.4" || merged.TorchVersion != "2.6.0" || merged.AttentionBackend != "flash-attn" {
		t.Fatalf("expected primary values to win, got %+v", merged)
	}
	if merged.FlashinferPresent == nil || !*merged.FlashinferPresent {
		t.Fatalf("expected primary boolean pointer to win, got %+v", merged)
	}
	if merged.FlashAttentionPresent == nil || *merged.FlashAttentionPresent {
		t.Fatalf("expected primary flash-attention pointer to win, got %+v", merged)
	}
}

func TestRuntimeCoverageMarksPopulatedAndMissingFields(t *testing.T) {
	cfg := contracts.RuntimeConfig{
		MaxModelLen:           8192,
		MaxNumBatchedTokens:   4096,
		MaxNumSeqs:            128,
		GPUMemoryUtilization:  0.9,
		TensorParallelSize:    2,
		Quantization:          "none",
		PrefixCaching:         boolPointer(true),
		ChunkedPrefill:        boolPointer(false),
		MultimodalFlags:       []string{"image"},
		VLLMVersion:           "0.8.4",
		TorchVersion:          "2.6.0",
		CUDARuntimeVersion:    "12.4",
		NvidiaDriverVersion:   "550.54.15",
		AttentionBackend:      "flash-attn",
		FlashinferPresent:     boolPointer(true),
		FlashAttentionPresent: boolPointer(true),
	}

	coverage := runtimeCoverage(cfg, "raw/runtime-probe.json")

	if !containsCoverageName(coverage.PresentFields, "vllm_version") {
		t.Fatalf("expected vllm_version to be present: %+v", coverage)
	}
	if !containsCoverageName(coverage.MissingFields, "image_processor") {
		t.Fatalf("expected image_processor to be marked missing: %+v", coverage)
	}
	if coverage.RawEvidenceRef != "raw/runtime-probe.json" {
		t.Fatalf("expected raw evidence ref to be preserved, got %q", coverage.RawEvidenceRef)
	}
}

func TestRuntimeCoverageMarksZeroIntegerFieldsMissing(t *testing.T) {
	coverage := runtimeCoverage(contracts.RuntimeConfig{}, "raw/runtime-probe.json")

	for _, field := range []string{"max_model_len", "max_num_batched_tokens", "max_num_seqs"} {
		if !containsCoverageName(coverage.MissingFields, field) {
			t.Fatalf("expected %s to be marked missing: %+v", field, coverage)
		}
		if containsCoverageName(coverage.PresentFields, field) {
			t.Fatalf("expected %s not to be marked present: %+v", field, coverage)
		}
	}
}

func TestRuntimeCoverageKeepsAutoMaxModelLenPresent(t *testing.T) {
	coverage := runtimeCoverage(contracts.RuntimeConfig{MaxModelLen: -1}, "raw/runtime-probe.json")

	if !containsCoverageName(coverage.PresentFields, "max_model_len") {
		t.Fatalf("expected auto max_model_len sentinel to be present: %+v", coverage)
	}
	if containsCoverageName(coverage.MissingFields, "max_model_len") {
		t.Fatalf("expected auto max_model_len sentinel not to be missing: %+v", coverage)
	}
}

func boolPointer(value bool) *bool {
	return &value
}
