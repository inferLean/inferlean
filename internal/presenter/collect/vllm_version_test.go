package collect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestFetchVLLMVersionUsesVersionEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.9.2"}`))
	}))
	defer server.Close()

	version, err := fetchVLLMVersion(context.Background(), server.URL+"/metrics")
	if err != nil {
		t.Fatalf("fetchVLLMVersion() error = %v", err)
	}
	if gotPath != "/version" {
		t.Fatalf("path = %q, want /version", gotPath)
	}
	if version != "0.9.2" {
		t.Fatalf("version = %q, want 0.9.2", version)
	}
}

func TestApplyLiveVLLMVersionHintRecordsSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"0.10.0"}`))
	}))
	defer server.Close()

	cfg := types.Configurations{}
	version := applyLiveVLLMVersionHint(context.Background(), &cfg, server.URL+"/metrics")
	if version != "0.10.0" {
		t.Fatalf("version = %q, want 0.10.0", version)
	}
	if got := cfg.EnvironmentHints["vllm_version_hint"]; got != "0.10.0" {
		t.Fatalf("vllm_version_hint = %q, want 0.10.0", got)
	}
	if got := cfg.EnvironmentHints["vllm_version_source"]; got != "vllm_version_api" {
		t.Fatalf("vllm_version_source = %q, want vllm_version_api", got)
	}
}
