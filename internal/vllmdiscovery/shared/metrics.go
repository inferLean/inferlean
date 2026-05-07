package shared

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const DefaultMetricsPort = 8000

func MetricsEndpoint(host string, port int) string {
	if port <= 0 {
		port = DefaultMetricsPort
	}
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/metrics",
	}).String()
}

func InferMetricsPort(rawCommandLine string, env []string) int {
	if port := portFromCommand(rawCommandLine); port > 0 {
		return port
	}
	if port := portFromEnv(env); port > 0 {
		return port
	}
	return DefaultMetricsPort
}

func InferMetricsHost(rawCommandLine string, env []string) string {
	if host := hostFromCommand(rawCommandLine); host != "" {
		return normalizeMetricsHost(host)
	}
	if host := hostFromEnv(env); host != "" {
		return normalizeMetricsHost(host)
	}
	return "127.0.0.1"
}

func InferMetricsEndpoint(rawCommandLine string, env []string) string {
	return MetricsEndpoint(
		InferMetricsHost(rawCommandLine, env),
		InferMetricsPort(rawCommandLine, env),
	)
}

func hostFromCommand(rawCommandLine string) string {
	tokens := SplitCommandLine(rawCommandLine)
	for idx := 0; idx < len(tokens); idx++ {
		token := strings.TrimSpace(tokens[idx])
		if strings.HasPrefix(token, "--host=") {
			return strings.TrimSpace(strings.TrimPrefix(token, "--host="))
		}
		if token == "--host" && idx+1 < len(tokens) {
			return strings.TrimSpace(tokens[idx+1])
		}
	}
	return ""
}

func hostFromEnv(env []string) string {
	for _, key := range []string{"VLLM_HOST", "HOST"} {
		if value, ok := envValue(env, key); ok {
			return value
		}
	}
	return ""
}

func normalizeMetricsHost(raw string) string {
	host := strings.Trim(strings.TrimSpace(raw), "[]")
	switch host {
	case "", "0.0.0.0":
		return "127.0.0.1"
	case "::":
		return "::1"
	default:
		return host
	}
}

func portFromCommand(rawCommandLine string) int {
	tokens := SplitCommandLine(rawCommandLine)
	for idx := 0; idx < len(tokens); idx++ {
		token := strings.TrimSpace(tokens[idx])
		if strings.HasPrefix(token, "--port=") {
			return parsePort(strings.TrimPrefix(token, "--port="))
		}
		if token == "--port" && idx+1 < len(tokens) {
			return parsePort(tokens[idx+1])
		}
	}
	return 0
}

func portFromEnv(env []string) int {
	for _, key := range []string{"VLLM_PORT", "PORT"} {
		if value, ok := envValue(env, key); ok {
			return parsePort(value)
		}
	}
	return 0
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(strings.TrimSpace(item), prefix) {
			return strings.TrimSpace(strings.TrimPrefix(item, prefix)), true
		}
	}
	return "", false
}

func parsePort(raw string) int {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}

func MissingPublishedPortError(containerName string, port int) error {
	target := strings.TrimSpace(containerName)
	if target == "" {
		target = "the vLLM container"
	}
	return fmt.Errorf("%s runs vLLM on port %d but that port is not published; expose it with docker -p %d:%d and run collection again", target, port, port, port)
}
