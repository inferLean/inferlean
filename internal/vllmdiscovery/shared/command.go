package shared

import "strings"

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
