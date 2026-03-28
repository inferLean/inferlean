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
		if r.Method != http.MethodPost {
			t.Fatalf("Method = %s, want %s", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(contracts.ArtifactUploadAck{
			UploadID:       "upload-123",
			RunID:          "run-123",
			InstallationID: "inst-456",
			Status:         "accepted",
			ReceivedAt:     time.Unix(1700000000, 0).UTC(),
			StatusURL:      server.URL + "/api/v1/runs/run-123",
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
}
