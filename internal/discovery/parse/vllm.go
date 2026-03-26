package parse

import (
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
	Model                string
	Host                 string
	Port                 int
	TensorParallelSize   int
	DataParallelSize     int
	PipelineParallelSize int
	MaxModelLen          int
	MaxNumBatchedTokens  int
	MaxNumSeqs           int
	GPUMemoryUtilization float64
	KVCacheDType         string
	ChunkedPrefill       *bool
	PrefixCaching        *bool
	Quantization         string
	MultimodalFlags      []string
	EnvHints             map[string]string
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
		if !strings.HasPrefix(token, "--") {
			continue
		}

		name, inlineValue, hasInlineValue := strings.Cut(token, "=")
		value := inlineValue
		if !hasInlineValue {
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				value = tokens[i+1]
				i++
			}
		}

		switch name {
		case "--model", "--served-model-name":
			if cfg.Model == "" {
				cfg.Model = value
			}
		case "--host":
			cfg.Host = value
		case "--port":
			setInt(&cfg.Port, value, &warnings, name)
		case "--tensor-parallel-size":
			setInt(&cfg.TensorParallelSize, value, &warnings, name)
		case "--data-parallel-size":
			setInt(&cfg.DataParallelSize, value, &warnings, name)
		case "--pipeline-parallel-size":
			setInt(&cfg.PipelineParallelSize, value, &warnings, name)
		case "--max-model-len":
			setInt(&cfg.MaxModelLen, value, &warnings, name)
		case "--max-num-batched-tokens":
			setInt(&cfg.MaxNumBatchedTokens, value, &warnings, name)
		case "--max-num-seqs":
			setInt(&cfg.MaxNumSeqs, value, &warnings, name)
		case "--gpu-memory-utilization":
			setFloat(&cfg.GPUMemoryUtilization, value, &warnings, name)
		case "--kv-cache-dtype":
			cfg.KVCacheDType = value
		case "--enable-chunked-prefill":
			b := true
			cfg.ChunkedPrefill = &b
		case "--disable-chunked-prefill":
			b := false
			cfg.ChunkedPrefill = &b
		case "--enable-prefix-caching":
			b := true
			cfg.PrefixCaching = &b
		case "--disable-prefix-caching":
			b := false
			cfg.PrefixCaching = &b
		case "--quantization":
			cfg.Quantization = value
		default:
			if strings.Contains(name, "multimodal") || strings.Contains(name, "mm-") {
				if value != "" {
					cfg.MultimodalFlags = append(cfg.MultimodalFlags, name+"="+value)
				} else {
					cfg.MultimodalFlags = append(cfg.MultimodalFlags, name)
				}
			}
		}
	}

	sort.Strings(cfg.MultimodalFlags)

	return ParsedProcess{
		Matched:       true,
		EntryPoint:    entryPoint,
		Signature:     signature(snapshot, entryPoint),
		RuntimeConfig: cfg,
		Warnings:      warnings,
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
