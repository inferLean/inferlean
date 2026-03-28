package events

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/inferLean/inferlean/internal/config"
)

func TestFlushSendsQueuedEventsAndClearsPendingFiles(t *testing.T) {
	dir := t.TempDir()
	pendingDir := filepath.Join(dir, "events", "pending")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q, want empty for anonymous flush", got)
		}
		var payload struct {
			Events []map[string]any `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(payload.Events) != 1 {
			t.Fatalf("event count = %d, want %d", len(payload.Events), 1)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	emitter := NewEmitterAt(pendingDir, server.Client())
	if err := emitter.EmitAsync("", config.AuthState{}, NewWorkflowEvent("inst-456", "run-123", "collect", "publish", "success", nil)); err != nil {
		t.Fatalf("EmitAsync() error = %v", err)
	}

	if err := emitter.Flush(context.Background(), server.URL, config.AuthState{
		BackendURL: server.URL,
	}); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("pending event files = %d, want %d", len(entries), 0)
	}
}
