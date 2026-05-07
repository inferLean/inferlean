package collect

import (
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

func parseVLLMArgs(rawCommandLine string) map[string]string {
	parsed := map[string]string{}
	tokens := shared.SplitCommandLine(rawCommandLine)
	for idx := 0; idx < len(tokens); idx++ {
		key, value, next := parseFlagToken(tokens, idx)
		if key == "" {
			continue
		}
		parsed[key] = value
		idx = next
	}
	if _, hasModel := parsed["model"]; !hasModel {
		if model := parseServeModelPositional(tokens); model != "" {
			parsed["model"] = model
		}
	}
	return parsed
}

func parseFlagToken(tokens []string, index int) (string, string, int) {
	token := strings.TrimSpace(tokens[index])
	if !strings.HasPrefix(token, "--") {
		return "", "", index
	}
	body := strings.TrimPrefix(token, "--")
	if body == "" {
		return "", "", index
	}
	if strings.Contains(body, "=") {
		parts := strings.SplitN(body, "=", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), index
	}
	if index+1 < len(tokens) && !strings.HasPrefix(strings.TrimSpace(tokens[index+1]), "-") {
		return body, strings.TrimSpace(tokens[index+1]), index + 1
	}
	return body, "true", index
}

func parseServeModelPositional(tokens []string) string {
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
