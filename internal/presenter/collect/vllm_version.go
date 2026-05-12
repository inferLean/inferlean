package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

const vllmVersionTimeout = 2 * time.Second

var (
	pinnedVLLMPattern = regexp.MustCompile(`(?i)vllm(?:==|~=|>=|<=|>|<)\s*v?(\d+\.\d+(?:\.\d+)?(?:rc\d+|post\d+)?)`)
	imageVLLMPattern  = regexp.MustCompile(`(?i)(?:^|[/:_-])vllm(?:[-_a-z]*)?:v?(\d+\.\d+(?:\.\d+)?(?:rc\d+|post\d+)?)`)
)

type vllmVersionResponse struct {
	Version string `json:"version"`
}

func applyLiveVLLMVersionHint(ctx context.Context, cfg *types.Configurations, endpoint string) string {
	version, err := fetchVLLMVersion(ctx, endpoint)
	if err != nil {
		cfg.EnvironmentHints = withHint(cfg.EnvironmentHints, "vllm_version_api_error", err.Error())
		return ""
	}
	cfg.EnvironmentHints = withHint(cfg.EnvironmentHints, "vllm_version_source", "vllm_version_api")
	cfg.EnvironmentHints = withHint(cfg.EnvironmentHints, "vllm_version_hint", version)
	return version
}

func fetchVLLMVersion(ctx context.Context, endpoint string) (string, error) {
	versionURL, err := vllmVersionURL(endpoint)
	if err != nil {
		return "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, vllmVersionTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, versionURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("query vLLM /version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query vLLM /version: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read vLLM /version: %w", err)
	}
	version := parseVLLMVersionResponse(body)
	if version == "" {
		return "", fmt.Errorf("query vLLM /version: missing version")
	}
	return version, nil
}

func vllmVersionURL(endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid vLLM endpoint %q", endpoint)
	}
	parsed.Path = "/version"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func parseVLLMVersionResponse(body []byte) string {
	var payload vllmVersionResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		return strings.TrimSpace(payload.Version)
	}
	return strings.TrimSpace(string(body))
}

func inferVLLMVersionHint(ctx context.Context, target vllmdiscovery.Candidate) string {
	if version := parseVLLMVersionText(target.RawCommandLine); version != "" {
		return version
	}
	containerID := strings.TrimSpace(target.ContainerID)
	if containerID == "" {
		return ""
	}
	image, err := inspectDockerImage(ctx, containerID)
	if err != nil {
		return ""
	}
	return parseVLLMVersionText(image)
}

func inspectDockerImage(ctx context.Context, containerID string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}", containerID)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	image := strings.TrimSpace(string(out))
	if image == "" {
		return "", fmt.Errorf("empty docker image for container %s", containerID)
	}
	return image, nil
}

func parseVLLMVersionText(text string) string {
	if matches := pinnedVLLMPattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	if matches := imageVLLMPattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}
