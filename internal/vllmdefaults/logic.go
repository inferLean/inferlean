package vllmdefaults

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	pinnedPattern = regexp.MustCompile(`(?i)vllm(?:==|~=|>=|<=|>|<)\s*v?(\d+\.\d+(?:\.\d+)?(?:rc\d+|post\d+)?)`)
	imagePattern  = regexp.MustCompile(`(?i)(?:^|[/:_-])vllm(?:[-_a-z]*)?:v?(\d+\.\d+(?:\.\d+)?(?:rc\d+|post\d+)?)`)
)

type versionCandidate struct {
	tag string
	ver semanticVersion
}

func normalizeArgs(raw map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range raw {
		canonical := normalizeKey(key)
		if canonical == "" {
			continue
		}
		out[canonical] = strings.TrimSpace(value)
	}
	return out
}

func normalizeKey(raw string) string {
	key := strings.TrimSpace(raw)
	key = strings.TrimLeft(key, "-")
	key = strings.ReplaceAll(key, "_", "-")
	key = strings.ToLower(key)
	return strings.TrimSpace(key)
}

func inferModel(explicit map[string]string, rawCommandLine string) string {
	if model := strings.TrimSpace(explicit["model"]); model != "" {
		return model
	}
	tokens := splitCommandLine(rawCommandLine)
	for idx := 0; idx < len(tokens); idx++ {
		if tokens[idx] != "serve" {
			continue
		}
		if idx+1 >= len(tokens) {
			return ""
		}
		candidate := strings.TrimSpace(tokens[idx+1])
		if strings.HasPrefix(candidate, "-") {
			return ""
		}
		return candidate
	}
	return ""
}

func inferRequestedVersion(input Input, explicit map[string]string) string {
	if raw := strings.TrimSpace(input.VLLMVersion); raw != "" {
		return raw
	}
	if raw := strings.TrimSpace(explicit["vllm-version"]); raw != "" {
		return raw
	}
	if matches := pinnedPattern.FindStringSubmatch(input.RawCommandLine); len(matches) == 2 {
		return matches[1]
	}
	if matches := imagePattern.FindStringSubmatch(input.RawCommandLine); len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func selectTag(tags []string, requested string) string {
	parseable := make([]versionCandidate, 0, len(tags))
	normalizedToOriginal := map[string]string{}
	for _, tag := range tags {
		normalized := normalizeTag(tag)
		if normalized != "" {
			normalizedToOriginal[normalized] = tag
		}
		if parsed, ok := parseSemanticVersion(tag); ok {
			parseable = append(parseable, versionCandidate{tag: tag, ver: parsed})
		}
	}
	if len(parseable) == 0 {
		return tags[len(tags)-1]
	}
	if normalized := normalizeTag(requested); normalized != "" {
		if exact, ok := normalizedToOriginal[normalized]; ok {
			return exact
		}
		if reqVersion, ok := parseSemanticVersion(normalized); ok {
			if best := selectBestNotNewer(parseable, reqVersion); best != "" {
				return best
			}
		}
	}
	sort.Slice(parseable, func(i, j int) bool {
		return compareSemanticVersion(parseable[i].ver, parseable[j].ver) > 0
	})
	return parseable[0].tag
}

func selectBestNotNewer(candidates []versionCandidate, requested semanticVersion) string {
	best := ""
	var bestVersion semanticVersion
	for _, item := range candidates {
		if compareSemanticVersion(item.ver, requested) > 0 {
			continue
		}
		if best == "" || compareSemanticVersion(item.ver, bestVersion) > 0 {
			best = item.tag
			bestVersion = item.ver
		}
	}
	return best
}

func normalizeTag(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "v") {
		return "v" + strings.TrimPrefix(strings.TrimPrefix(trimmed, "v"), "V")
	}
	return "v" + trimmed
}

func selectProfile(profiles map[string]profileDefaults, input Input) string {
	if len(profiles) == 0 {
		return ""
	}
	if len(profiles) == 1 {
		for name := range profiles {
			return name
		}
	}
	if high, ok := profiles["cuda_high_memory_non_a100"]; ok {
		if shouldUseHighMemoryProfile(input) && high.Resolved != nil {
			return "cuda_high_memory_non_a100"
		}
	}
	if _, ok := profiles["cuda_default"]; ok {
		return "cuda_default"
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[0]
}

func shouldUseHighMemoryProfile(input Input) bool {
	if input.GPUMemoryMiB < 70000 {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(input.GPUModel))
	if model == "" {
		return false
	}
	return !strings.Contains(model, "a100")
}

func applyDefaults(target map[string]string, profile profileDefaults, model string) int {
	merged := map[string]any{}
	for key, value := range profile.Resolved {
		merged[key] = value
	}
	if modelDefaults, ok := profile.Models[model]; ok {
		for key, value := range modelDefaults.Resolved {
			merged[key] = value
		}
	}
	applied := 0
	for rawKey, rawValue := range merged {
		key := normalizeKey(rawKey)
		if key == "" {
			continue
		}
		if key == "model" && strings.TrimSpace(model) == "" {
			continue
		}
		if _, exists := target[key]; exists {
			continue
		}
		value := stringifyValue(rawValue)
		if strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
		applied++
	}
	return applied
}

func stringifyValue(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case bool:
		return strconv.FormatBool(value)
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case nil:
		return ""
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return strings.TrimSpace(fmt.Sprint(value))
		}
		return string(payload)
	}
}

func copyStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func splitCommandLine(raw string) []string {
	out := []string{}
	var token strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if token.Len() == 0 {
			return
		}
		out = append(out, token.String())
		token.Reset()
	}
	for _, r := range raw {
		if escaped {
			token.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			token.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		token.WriteRune(r)
	}
	if escaped {
		token.WriteRune('\\')
	}
	flush()
	return out
}
