package collect

import (
	"strings"
	"unicode"
)

func parseVLLMArgs(rawCommandLine string) map[string]string {
	parsed := map[string]string{}
	tokens := splitCommandLine(rawCommandLine)
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
