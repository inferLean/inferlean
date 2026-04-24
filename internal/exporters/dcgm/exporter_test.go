package dcgm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "hostport", in: "127.0.0.1:9400", want: "http://127.0.0.1:9400/metrics"},
		{name: "http no path", in: "http://127.0.0.1:9400", want: "http://127.0.0.1:9400/metrics"},
		{name: "custom path", in: "http://127.0.0.1:9400/custom", want: "http://127.0.0.1:9400/custom"},
		{name: "invalid", in: "://", want: ""},
		{name: "empty", in: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeEndpoint(tc.in); got != tc.want {
				t.Fatalf("normalizeEndpoint(%q)=%q want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStartUsesExistingEndpointFromEnv(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP test test\n"))
	}))
	t.Cleanup(server.Close)

	t.Setenv("INFERLEAN_DCGM_EXPORTER_ENDPOINT", server.URL)

	res := Start(context.Background())
	if !res.Available {
		t.Fatalf("expected available=true, got false with reason=%q", res.Reason)
	}
	if res.Endpoint == "" {
		t.Fatalf("expected endpoint to be set")
	}
	if res.Session != nil {
		t.Fatalf("expected session=nil when attaching to existing exporter")
	}
}
