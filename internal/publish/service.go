package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/auth"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
)

const httpTimeout = 20 * time.Second

type Step string

const (
	StepAuth   Step = "auth"
	StepUpload Step = "upload"
	StepWait   Step = "wait"
)

type StepUpdate struct {
	Step    Step
	Message string
}

type Options struct {
	BaseURL  string
	Artifact contracts.RunArtifact
	Auth     config.AuthState
	Stepf    func(StepUpdate)
}

type Result struct {
	Ack  contracts.ArtifactUploadAck
	Auth config.AuthState
}

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

func (s Service) Publish(ctx context.Context, opts Options) (Result, error) {
	baseURL := auth.NormalizeBaseURL(opts.BaseURL)
	if baseURL == "" {
		baseURL = auth.NormalizeBaseURL(opts.Auth.BackendURL)
	}
	if baseURL == "" {
		return Result{}, fmt.Errorf("backend URL is required for publish")
	}

	session := opts.Auth
	authenticated := false
	if session.HasSession() {
		emitStep(opts.Stepf, StepAuth, "Refreshing saved login when available")
		session.BackendURL = baseURL

		updatedAuth, err := s.authManager.EnsureValid(ctx, session)
		if err == nil {
			session = updatedAuth
			authenticated = true
		}
	}

	payload, err := json.Marshal(opts.Artifact)
	if err != nil {
		return Result{}, fmt.Errorf("encode artifact upload: %w", err)
	}

	emitStep(opts.Stepf, StepUpload, "Uploading run artifact to the backend")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/artifacts", bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("build artifact upload request: %w", err)
	}
	if authenticated {
		tokenType := session.TokenType
		if strings.TrimSpace(tokenType) == "" {
			tokenType = "Bearer"
		}
		req.Header.Set("Authorization", tokenType+" "+session.BearerToken())
	}
	req.Header.Set("Content-Type", "application/json")

	emitStep(opts.Stepf, StepWait, "Waiting for durable backend acknowledgement")
	resp, err := s.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("upload artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Result{}, fmt.Errorf("upload artifact: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var ack contracts.ArtifactUploadAck
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return Result{}, fmt.Errorf("decode artifact upload acknowledgement: %w", err)
	}
	if err := ack.Validate(); err != nil {
		return Result{}, fmt.Errorf("invalid artifact upload acknowledgement: %w", err)
	}

	return Result{Ack: ack, Auth: session}, nil
}

func emitStep(stepf func(StepUpdate), step Step, message string) {
	if stepf == nil {
		return
	}
	stepf(StepUpdate{Step: step, Message: message})
}
