package publish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestPublishUsesBearerHeaderAndReturnsAck(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-id-token" {
			t.Fatalf("Authorization header = %q, want %q", got, "Bearer test-id-token")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/artifacts":
			w.WriteHeader(http.StatusAccepted)
			if err := json.NewEncoder(w).Encode(contracts.ArtifactUploadAck{
				UploadID:       "upload-123",
				RunID:          "run-123",
				InstallationID: "inst-456",
				Status:         "accepted",
				ReceivedAt:     time.Unix(1700000000, 0).UTC(),
				StatusURL:      server.URL + "/api/v1/runs/run-123",
				ReportURL:      server.URL + "/api/v1/runs/run-123/report",
			}); err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/runs/run-123/report":
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
					IneligibleReason:             "host metrics missing",
				},
			}); err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	service := NewService()
	result, err := service.Publish(context.Background(), Options{
		BaseURL: server.URL,
		Artifact: contracts.RunArtifact{
			SchemaVersion: contracts.SchemaVersion,
			Job: contracts.Job{
				RunID:            "run-123",
				InstallationID:   "inst-456",
				CollectorVersion: "0.2.0",
				SchemaVersion:    contracts.SchemaVersion,
				CollectedAt:      time.Unix(1700000000, 0).UTC(),
			},
		},
		Auth: config.AuthState{
			BackendURL: server.URL,
			Issuer:     "https://dex.example.com/dex",
			ClientID:   "inferlean-cli",
			TokenType:  "Bearer",
			IDToken:    "test-id-token",
			UseIDToken: true,
			ExpiresAt:  time.Now().Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.Ack.UploadID != "upload-123" {
		t.Fatalf("Ack.UploadID = %q, want %q", result.Ack.UploadID, "upload-123")
	}
	if result.Report == nil || result.Report.Job.RunID != "run-123" {
		t.Fatalf("Report = %+v, want canonical report for run-123", result.Report)
	}
}

func TestPublishReturnsSummaryPreviewWhenAnonymous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q, want empty", got)
		}
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(contracts.ArtifactUploadAck{
			UploadID:       "upload-123",
			RunID:          "run-123",
			InstallationID: "inst-456",
			Status:         "accepted",
			ReceivedAt:     time.Unix(1700000000, 0).UTC(),
			SummaryPreview: &contracts.SummaryPreview{
				Headline:              "Queue pressure is limiting throughput.",
				PrimaryRecommendation: "Widen scheduler posture",
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	service := NewService()
	result, err := service.Publish(context.Background(), Options{
		BaseURL: server.URL,
		Artifact: contracts.RunArtifact{
			SchemaVersion: contracts.SchemaVersion,
			Job: contracts.Job{
				RunID:            "run-123",
				InstallationID:   "inst-456",
				CollectorVersion: "0.2.0",
				SchemaVersion:    contracts.SchemaVersion,
				CollectedAt:      time.Unix(1700000000, 0).UTC(),
			},
		},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.Report != nil {
		t.Fatalf("Report = %+v, want nil for anonymous publish", result.Report)
	}
	if result.SummaryPreview == nil || result.SummaryPreview.Headline == "" {
		t.Fatalf("SummaryPreview = %+v, want populated anonymous preview", result.SummaryPreview)
	}
}
