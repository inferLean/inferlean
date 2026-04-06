package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/auth"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
)

const httpTimeout = 20 * time.Second

type Service struct {
	client      *http.Client
	authManager *auth.Manager
}

func NewService() Service {
	client := &http.Client{Timeout: httpTimeout}
	return Service{
		client:      client,
		authManager: auth.NewManagerWithClient(client),
	}
}

func (s Service) List(ctx context.Context, baseURL string, session config.AuthState) ([]contracts.RunSummary, config.AuthState, error) {
	updated, err := s.ensureSession(ctx, baseURL, session)
	if err != nil {
		return nil, config.AuthState{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, auth.NormalizeBaseURL(updated.BackendURL)+"/api/v1/runs", nil)
	if err != nil {
		return nil, config.AuthState{}, fmt.Errorf("build runs request: %w", err)
	}
	req.Header.Set("Authorization", tokenHeader(updated))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, config.AuthState{}, fmt.Errorf("list runs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, config.AuthState{}, fmt.Errorf("list runs: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var response contracts.RunListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, config.AuthState{}, fmt.Errorf("decode runs response: %w", err)
	}

	return response.Runs, updated, nil
}

func (s Service) Get(ctx context.Context, baseURL, runID string, session config.AuthState) (contracts.RunDetailResponse, config.AuthState, error) {
	updated, err := s.ensureSession(ctx, baseURL, session)
	if err != nil {
		return contracts.RunDetailResponse{}, config.AuthState{}, err
	}

	pathRunID := url.PathEscape(strings.TrimSpace(runID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, auth.NormalizeBaseURL(updated.BackendURL)+"/api/v1/runs/"+pathRunID, nil)
	if err != nil {
		return contracts.RunDetailResponse{}, config.AuthState{}, fmt.Errorf("build run detail request: %w", err)
	}
	req.Header.Set("Authorization", tokenHeader(updated))

	resp, err := s.client.Do(req)
	if err != nil {
		return contracts.RunDetailResponse{}, config.AuthState{}, fmt.Errorf("load run detail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return contracts.RunDetailResponse{}, config.AuthState{}, fmt.Errorf("load run detail: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var response contracts.RunDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return contracts.RunDetailResponse{}, config.AuthState{}, fmt.Errorf("decode run detail response: %w", err)
	}

	return response, updated, nil
}

func (s Service) GetReport(ctx context.Context, reportURL string, session config.AuthState) (contracts.FinalReport, config.AuthState, error) {
	updated, err := s.ensureSession(ctx, reportBaseURL(reportURL, session.BackendURL), session)
	if err != nil {
		return contracts.FinalReport{}, config.AuthState{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(reportURL), nil)
	if err != nil {
		return contracts.FinalReport{}, config.AuthState{}, fmt.Errorf("build report request: %w", err)
	}
	req.Header.Set("Authorization", tokenHeader(updated))

	resp, err := s.client.Do(req)
	if err != nil {
		return contracts.FinalReport{}, config.AuthState{}, fmt.Errorf("load report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return contracts.FinalReport{}, config.AuthState{}, fmt.Errorf("load report: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var report contracts.FinalReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return contracts.FinalReport{}, config.AuthState{}, fmt.Errorf("decode report response: %w", err)
	}
	if err := report.Validate(); err != nil {
		return contracts.FinalReport{}, config.AuthState{}, fmt.Errorf("invalid report response: %w", err)
	}

	return report, updated, nil
}

func (s Service) ensureSession(ctx context.Context, baseURL string, session config.AuthState) (config.AuthState, error) {
	if !session.HasSession() {
		return config.AuthState{}, fmt.Errorf("login required; run inferlean login first")
	}

	session.BackendURL = auth.NormalizeBaseURL(firstNonEmpty(baseURL, session.BackendURL))
	if session.BackendURL == "" {
		return config.AuthState{}, fmt.Errorf("backend URL is required")
	}

	updated, err := s.authManager.EnsureValid(ctx, session)
	if err != nil {
		return config.AuthState{}, err
	}
	return updated, nil
}

func tokenHeader(session config.AuthState) string {
	tokenType := strings.TrimSpace(session.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return tokenType + " " + session.BearerToken()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func reportBaseURL(reportURL, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}

	parsed, err := url.Parse(strings.TrimSpace(reportURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return reportURL
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
