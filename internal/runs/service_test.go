package runs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/config"
)

func TestServiceListReturnsRuns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer access-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"runs":[{"run_id":"run-123","upload_id":"upload-123","installation_id":"inst-456","schema_version":"v2.5","collector_version":"0.2.0","received_at":"2023-11-14T22:13:20Z"}]}`))
	}))
	defer server.Close()

	service := NewService()
	runs, session, err := service.List(context.Background(), server.URL, config.AuthState{
		BackendURL:  server.URL,
		Issuer:      server.URL + "/dex",
		ClientID:    "inferlean-cli",
		TokenType:   "Bearer",
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if session.BackendURL != server.URL {
		t.Fatalf("session.BackendURL = %q, want %q", session.BackendURL, server.URL)
	}
	if len(runs) != 1 || runs[0].RunID != "run-123" {
		t.Fatalf("runs = %#v, want one run with run_id run-123", runs)
	}
}

func TestServiceGetReturnsArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/run-123" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/runs/run-123")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"run-123","upload_id":"upload-123","installation_id":"inst-456","schema_version":"v2.5","collector_version":"0.2.0","subject":"user-123","received_at":"2023-11-14T22:13:20Z","artifact":{"job":{"run_id":"run-123"},"metrics":{"gpu_utilization":0.8}}}`))
	}))
	defer server.Close()

	service := NewService()
	detail, _, err := service.Get(context.Background(), server.URL, "run-123", config.AuthState{
		BackendURL:  server.URL,
		Issuer:      server.URL + "/dex",
		ClientID:    "inferlean-cli",
		TokenType:   "Bearer",
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.RunID != "run-123" {
		t.Fatalf("RunID = %q, want %q", detail.RunID, "run-123")
	}
	if string(detail.Artifact) != `{"job":{"run_id":"run-123"},"metrics":{"gpu_utilization":0.8}}` {
		t.Fatalf("Artifact = %s, want artifact JSON", string(detail.Artifact))
	}
}

func TestServiceListRequiresLogin(t *testing.T) {
	service := NewService()
	_, _, err := service.List(context.Background(), "https://app.inferlean.com", config.AuthState{})
	if err == nil || err.Error() != "login required; run inferlean login first" {
		t.Fatalf("List() error = %v, want login required", err)
	}
}
