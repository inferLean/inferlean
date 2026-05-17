package artifactnormalize

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildMultimodalFlags(input Input) []string {
	flags := []string{}
	if input.UserIntent.Multimodal {
		flags = append(flags, "multimodal")
	}
	flags = append(flags, limitMMPerPromptFlags(input.Configurations.ParsedArgs["limit-mm-per-prompt"])...)
	return dedupeSortedStrings(flags)
}

func multimodalRuntimeHintsUnsupported(runtime contracts.RuntimeConfig, input Input) bool {
	if len(runtime.MultimodalFlags) > 0 {
		return false
	}
	_, disabled := parseLimitMMPerPrompt(input.Configurations.ParsedArgs["limit-mm-per-prompt"])
	return disabled
}

func limitMMPerPromptFlags(raw string) []string {
	limits, _ := parseLimitMMPerPrompt(raw)
	flags := make([]string, 0, len(limits))
	for modality, limit := range limits {
		if limit <= 0 {
			continue
		}
		flag := multimodalLimitFlag(modality)
		if flag == "" {
			continue
		}
		flags = append(flags, flag)
	}
	return dedupeSortedStrings(flags)
}

func parseLimitMMPerPrompt(raw string) (map[string]float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, false
	}
	limits := map[string]float64{}
	for modality, rawLimit := range payload {
		limit, ok := numericLimit(rawLimit)
		if !ok {
			continue
		}
		limits[modality] = limit
	}
	if len(limits) == 0 {
		return limits, false
	}
	for _, limit := range limits {
		if limit > 0 {
			return limits, false
		}
	}
	return limits, true
}

func numericLimit(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func multimodalLimitFlag(modality string) string {
	key := strings.ToLower(strings.TrimSpace(modality))
	key = strings.ReplaceAll(key, "_", "-")
	switch key {
	case "":
		return ""
	case "image", "images":
		return "image-inputs"
	case "video", "videos":
		return "video-inputs"
	case "audio", "audios":
		return "audio-inputs"
	default:
		return "multimodal"
	}
}

func dedupeSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) > 0 && out[len(out)-1] == value {
			continue
		}
		out = append(out, value)
	}
	return out
}
