package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCollectTargets(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("test_metric 1\n"))
	}))
	defer server.Close()
	collector := NewCollector()
	result := collector.CollectTargets(context.Background(), []Target{
		{Name: "good", Endpoint: server.URL, Required: true},
		{Name: "missing", Endpoint: "http://127.0.0.1:1/metrics"},
	}, 250*time.Millisecond, 100*time.Millisecond)
	if result.SourceStatus["good"] != "ok" {
		t.Fatalf("good source status = %q", result.SourceStatus["good"])
	}
	if result.SourceStatus["missing"] != "missing" {
		t.Fatalf("missing source status = %q", result.SourceStatus["missing"])
	}
	if len(result.Samples["good"]) == 0 {
		t.Fatal("expected good source samples")
	}
	if result.Samples["good"][0].Timestamp.IsZero() {
		t.Fatal("expected timestamped samples")
	}
	if len(result.Samples["good"][0].Metrics) == 0 {
		t.Fatal("expected parsed metrics in first sample")
	}
	if !strings.Contains(result.RawText, "target=good") {
		t.Fatal("expected raw text to include good target")
	}
	if !strings.Contains(result.RawByTarget["good"], "test_metric") {
		t.Fatal("expected per-target raw data")
	}
}
