package parse

import "strings"

func applyRuntimeHint(cfg *RuntimeConfig, name, value string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "attention-backend"), strings.Contains(lower, "attn-backend"):
		cfg.AttentionBackend = value
	case strings.Contains(lower, "flashinfer"):
		cfg.FlashinferPresent = boolPointer(flagEnabled(name))
	case strings.Contains(lower, "flash-attn"), strings.Contains(lower, "flash_attention"):
		cfg.FlashAttentionPresent = boolPointer(flagEnabled(name))
	case strings.Contains(lower, "image") && strings.Contains(lower, "processor"):
		cfg.ImageProcessor = value
	case strings.Contains(lower, "cache") && (strings.Contains(lower, "multimodal") || strings.Contains(lower, "mm-") || strings.Contains(lower, "image")):
		cfg.MultimodalCacheHints = appendUnique(cfg.MultimodalCacheHints, formatFlag(name, value))
	case strings.Contains(lower, "multimodal"), strings.Contains(lower, "mm-"):
		cfg.MultimodalFlags = appendUnique(cfg.MultimodalFlags, formatFlag(name, value))
	}
}

func applyEnvRuntimeHints(cfg *RuntimeConfig) {
	for key, value := range cfg.EnvHints {
		lower := strings.ToLower(key)
		switch {
		case strings.Contains(lower, "attention_backend"):
			cfg.AttentionBackend = value
		case strings.Contains(lower, "flashinfer"):
			cfg.FlashinferPresent = boolPointer(valueEnabled(value))
		case strings.Contains(lower, "flash_attn"), strings.Contains(lower, "flashattention"):
			cfg.FlashAttentionPresent = boolPointer(valueEnabled(value))
		case strings.Contains(lower, "image") && strings.Contains(lower, "processor"):
			cfg.ImageProcessor = value
		case strings.Contains(lower, "cache") && (strings.Contains(lower, "multimodal") || strings.Contains(lower, "mm_") || strings.Contains(lower, "image")):
			cfg.MultimodalCacheHints = appendUnique(cfg.MultimodalCacheHints, key+"="+value)
		case strings.HasPrefix(key, "VLLM_MM_"), strings.Contains(lower, "multimodal"):
			cfg.MultimodalFlags = appendUnique(cfg.MultimodalFlags, key+"="+value)
		}
	}
}

func formatFlag(name, value string) string {
	if value == "" {
		return name
	}
	return name + "=" + value
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func boolPointer(value bool) *bool {
	clone := value
	return &clone
}

func flagEnabled(name string) bool {
	lower := strings.ToLower(name)
	return !strings.Contains(lower, "disable") && !strings.Contains(lower, "no-")
}

func valueEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "off", "disabled", "no":
		return false
	default:
		return true
	}
}
