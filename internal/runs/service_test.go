package runs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
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

func TestServiceGetReportReturnsCanonicalReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/run-123/report" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/runs/run-123/report")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer access-token")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(contracts.FinalReport{
			SchemaVersion: contracts.ReportSchemaVersion,
			Job: contracts.ReportJob{
				RunID: "run-123",
			},
			Entitlement: contracts.ReportEntitlement{Tier: "free"},
			Diagnosis: contracts.DiagnosisSection{
				ScenarioOverlays: contracts.ScenarioOverlays{
					Latency:    contracts.ScenarioOverlay{Target: "latency"},
					Balanced:   contracts.ScenarioOverlay{Target: "balanced"},
					Throughput: contracts.ScenarioOverlay{Target: "throughput"},
				},
			},
			DiagnosticCoverage: contracts.DiagnosticCoverage{
				EligibleForRequiredDetectors: false,
				IneligibleReason:             "missing gpu telemetry",
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	service := NewService()
	report, _, err := service.GetReport(context.Background(), server.URL+"/api/v1/runs/run-123/report", config.AuthState{
		BackendURL:  server.URL,
		Issuer:      server.URL + "/dex",
		ClientID:    "inferlean-cli",
		TokenType:   "Bearer",
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("GetReport() error = %v", err)
	}
	if report.Job.RunID != "run-123" {
		t.Fatalf("Job.RunID = %q, want %q", report.Job.RunID, "run-123")
	}
}

func TestServiceGetReportRetriesServerErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary backend error", http.StatusBadGateway)
			return
		}
		if err := json.NewEncoder(w).Encode(contracts.FinalReport{
			SchemaVersion: contracts.ReportSchemaVersion,
			Job:           contracts.ReportJob{RunID: "run-123"},
			Entitlement:   contracts.ReportEntitlement{Tier: "free"},
			Diagnosis: contracts.DiagnosisSection{
				ScenarioOverlays: contracts.ScenarioOverlays{
					Latency:    contracts.ScenarioOverlay{Target: "latency"},
					Balanced:   contracts.ScenarioOverlay{Target: "balanced"},
					Throughput: contracts.ScenarioOverlay{Target: "throughput"},
				},
			},
			DiagnosticCoverage: contracts.DiagnosticCoverage{
				EligibleForRequiredDetectors: false,
				IneligibleReason:             "missing gpu telemetry",
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	service := NewService()
	report, _, err := service.GetReport(context.Background(), server.URL+"/api/v1/runs/run-123/report", config.AuthState{
		BackendURL:  server.URL,
		Issuer:      server.URL + "/dex",
		ClientID:    "inferlean-cli",
		TokenType:   "Bearer",
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("GetReport() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want %d", attempts, 2)
	}
	if report.Job.RunID != "run-123" {
		t.Fatalf("Job.RunID = %q, want %q", report.Job.RunID, "run-123")
	}
}

func TestServiceGetReportDoesNotRetryClientErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "report not found", http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService()
	_, _, err := service.GetReport(context.Background(), server.URL+"/api/v1/runs/run-123/report", config.AuthState{
		BackendURL:  server.URL,
		Issuer:      server.URL + "/dex",
		ClientID:    "inferlean-cli",
		TokenType:   "Bearer",
		AccessToken: "access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("GetReport() error = %v, want 404 failure", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want %d", attempts, 1)
	}
}
