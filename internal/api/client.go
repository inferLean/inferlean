package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Client struct {
	http *http.Client
}

func NewClient() Client {
	return Client{http: &http.Client{Timeout: 20 * time.Second}}
}

func (c Client) UploadArtifact(ctx context.Context, backendURL string, artifact contracts.RunArtifact, auth types.AuthState) (types.UploadAck, error) {
	payload, err := json.Marshal(artifact)
	if err != nil {
		return types.UploadAck{}, fmt.Errorf("encode artifact: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeBaseURL(backendURL)+"/api/v1/artifacts", bytes.NewReader(payload))
	if err != nil {
		return types.UploadAck{}, fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	attachAuth(req, auth)
	resp, err := c.http.Do(req)
	if err != nil {
		return types.UploadAck{}, fmt.Errorf("upload artifact: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return types.UploadAck{}, fmt.Errorf("upload artifact: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var ack types.UploadAck
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return types.UploadAck{}, fmt.Errorf("decode upload ack: %w", err)
	}
	return ack, nil
}

func (c Client) GetReport(ctx context.Context, reportURL string, auth types.AuthState) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(reportURL), nil)
	if err != nil {
		return nil, fmt.Errorf("build report request: %w", err)
	}
	attachAuth(req, auth)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("load report: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("load report: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var report map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("decode report: %w", err)
	}
	return report, nil
}

func (c Client) GetRunReport(ctx context.Context, backendURL string, runID string, auth types.AuthState) (map[string]any, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("run id is required")
	}
	reportURL := normalizeBaseURL(backendURL) + "/api/v1/runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/report"
	return c.GetReport(ctx, reportURL, auth)
}

func attachAuth(req *http.Request, auth types.AuthState) {
	token := strings.TrimSpace(auth.BearerToken())
	if token == "" {
		return
	}
	tokenType := strings.TrimSpace(auth.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	req.Header.Set("Authorization", tokenType+" "+token)
}

func normalizeBaseURL(url string) string {
	return strings.TrimRight(strings.TrimSpace(url), "/")
}
