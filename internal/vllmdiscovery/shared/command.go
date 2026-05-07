package shared

import (
	"strings"
	"unicode"
)

func IsServeCommand(command string) bool {
	low := strings.ToLower(strings.TrimSpace(command))
	if low == "" {
		return false
	}
	if strings.Contains(low, "vllm bench") {
		return false
	}
	serveMarkers := []string{
		"vllm serve",
		"entrypoints.openai.api_server",
		"openai.api_server",
		"api_server.py",
		"api_server",
	}
	for _, marker := range serveMarkers {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func SplitCommandLine(raw string) []string {
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
