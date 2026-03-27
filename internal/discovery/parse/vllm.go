package parse

import (
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean/internal/discovery/process"
)

var legacyModules = map[string]struct{}{
	"vllm.entrypoints.api_server":        {},
	"vllm.entrypoints.openai.api_server": {},
}

type ParsedProcess struct {
	Matched       bool
	EntryPoint    string
	Signature     string
	RuntimeConfig RuntimeConfig
	Warnings      []string
}

type RuntimeConfig struct {
	Model                 string
	ServedModelName       string
	Host                  string
	Port                  int
	TensorParallelSize    int
	DataParallelSize      int
	PipelineParallelSize  int
	MaxModelLen           int
	MaxNumBatchedTokens   int
	MaxNumSeqs            int
	GPUMemoryUtilization  float64
	KVCacheDType          string
	ChunkedPrefill        *bool
	PrefixCaching         *bool
	Quantization          string
	DType                 string
	GenerationConfig      string
	APIKeyConfigured      bool
	MultimodalFlags       []string
	AttentionBackend      string
	FlashinferPresent     *bool
	FlashAttentionPresent *bool
	ImageProcessor        string
	MultimodalCacheHints  []string
	EnvHints              map[string]string
}

func Parse(snapshot process.Snapshot) ParsedProcess {
	entryPoint, offset, matched := detectEntryPoint(snapshot.Args)
	if !matched {
		return ParsedProcess{}
	}

	cfg := RuntimeConfig{
		EnvHints: copyEnvHints(snapshot.EnvHints),
	}
	warnings := []string{}
	tokens := snapshot.Args[offset:]

	if entryPoint == "vllm serve" && len(tokens) > 0 && !strings.HasPrefix(tokens[0], "-") {
		cfg.Model = tokens[0]
		tokens = tokens[1:]
	}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if !strings.HasPrefix(token, "-") {
			continue
		}

		name, inlineValue, hasInlineValue := strings.Cut(token, "=")
		value := inlineValue
		if !hasInlineValue {
			value, i = nextValue(tokens, i)
		}

		switch name {
		case "--model":
			if cfg.Model == "" {
				cfg.Model = value
			}
		case "--served-model-name":
			if cfg.ServedModelName == "" {
				cfg.ServedModelName = value
			}
		case "--host":
			cfg.Host = value
		case "--port":
			setInt(&cfg.Port, value, &warnings, name)
		case "--tensor-parallel-size", "-tp":
			setInt(&cfg.TensorParallelSize, value, &warnings, name)
		case "--data-parallel-size", "-dp":
			setInt(&cfg.DataParallelSize, value, &warnings, name)
		case "--pipeline-parallel-size", "-pp":
			setInt(&cfg.PipelineParallelSize, value, &warnings, name)
		case "--max-model-len":
			setHumanInt(&cfg.MaxModelLen, value, &warnings, name, true)
		case "--max-num-batched-tokens":
			setHumanInt(&cfg.MaxNumBatchedTokens, value, &warnings, name, false)
		case "--max-num-seqs":
			setInt(&cfg.MaxNumSeqs, value, &warnings, name)
		case "--gpu-memory-utilization":
			setFloat(&cfg.GPUMemoryUtilization, value, &warnings, name)
		case "--kv-cache-dtype":
			cfg.KVCacheDType = value
		case "--enable-chunked-prefill":
			b := true
			cfg.ChunkedPrefill = &b
		case "--disable-chunked-prefill", "--no-enable-chunked-prefill":
			b := false
			cfg.ChunkedPrefill = &b
		case "--enable-prefix-caching":
			b := true
			cfg.PrefixCaching = &b
		case "--disable-prefix-caching", "--no-enable-prefix-caching":
			b := false
			cfg.PrefixCaching = &b
		case "--quantization", "-q":
			cfg.Quantization = value
		case "--dtype":
			cfg.DType = value
		case "--generation-config":
			cfg.GenerationConfig = value
		case "--api-key":
			cfg.APIKeyConfigured = true
		default:
			applyRuntimeHint(&cfg, name, value)
		}
	}

	applyEnvRuntimeHints(&cfg)
	sort.Strings(cfg.MultimodalFlags)
	sort.Strings(cfg.MultimodalCacheHints)

	return ParsedProcess{
		Matched:       true,
		EntryPoint:    entryPoint,
		Signature:     signature(snapshot, entryPoint),
		RuntimeConfig: cfg,
		Warnings:      warnings,
	}
}

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

func detectEntryPoint(args []string) (string, int, bool) {
	for idx := 0; idx < len(args); idx++ {
		base := filepath.Base(args[idx])
		if base == "vllm" && idx+1 < len(args) && args[idx+1] == "serve" {
			return "vllm serve", idx + 2, true
		}

		if args[idx] == "-m" && idx+1 < len(args) {
			if _, ok := legacyModules[args[idx+1]]; ok {
				return args[idx+1], idx + 2, true
			}
		}
	}

	return "", 0, false
}

func signature(snapshot process.Snapshot, entryPoint string) string {
	parts := []string{entryPoint}
	for _, arg := range snapshot.Args {
		if strings.HasPrefix(arg, "--port") || strings.HasPrefix(arg, "--host") {
			continue
		}
		parts = append(parts, arg)
	}

	return strings.Join(parts, "\x00")
}

func setInt(target *int, value string, warnings *[]string, flag string) {
	if value == "" {
		return
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		*warnings = append(*warnings, "could not parse "+flag+" value")
		return
	}

	*target = parsed
}

func setHumanInt(target *int, value string, warnings *[]string, flag string, allowAuto bool) {
	if value == "" {
		return
	}

	parsed, err := parseHumanInt(value, allowAuto)
	if err != nil {
		*warnings = append(*warnings, "could not parse "+flag+" value")
		return
	}

	*target = parsed
}

func setFloat(target *float64, value string, warnings *[]string, flag string) {
	if value == "" {
		return
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		*warnings = append(*warnings, "could not parse "+flag+" value")
		return
	}

	*target = parsed
}

func nextValue(tokens []string, idx int) (string, int) {
	if idx+1 >= len(tokens) {
		return "", idx
	}

	value := tokens[idx+1]
	if strings.HasPrefix(value, "-") {
		if _, err := parseHumanInt(value, true); err != nil {
			return "", idx
		}
	}

	return value, idx + 1
}

func parseHumanInt(value string, allowAuto bool) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, strconv.ErrSyntax
	}

	if allowAuto && strings.EqualFold(trimmed, "auto") {
		return -1, nil
	}

	if parsed, err := strconv.Atoi(trimmed); err == nil {
		return parsed, nil
	}

	suffix := trimmed[len(trimmed)-1]
	baseValue := trimmed[:len(trimmed)-1]

	multiplier := 0.0
	switch suffix {
	case 'k':
		multiplier = 1_000
	case 'K':
		multiplier = 1 << 10
	case 'm':
		multiplier = 1_000_000
	case 'M':
		multiplier = 1 << 20
	case 'g':
		multiplier = 1_000_000_000
	case 'G':
		multiplier = 1 << 30
	default:
		return 0, strconv.ErrSyntax
	}

	parsed, err := strconv.ParseFloat(baseValue, 64)
	if err != nil {
		return 0, err
	}

	return int(math.Round(parsed * multiplier)), nil
}

func copyEnvHints(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}
