package dcgm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

	res := Start(context.Background(), "")
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

func TestStartUsesExplicitEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP test test\n"))
	}))
	t.Cleanup(server.Close)

	t.Setenv("INFERLEAN_DCGM_EXPORTER_ENDPOINT", "http://127.0.0.1:1/metrics")

	res := Start(context.Background(), server.URL)
	if !res.Available {
		t.Fatalf("expected available=true, got false with reason=%q", res.Reason)
	}
	if res.Endpoint != server.URL+"/metrics" {
		t.Fatalf("Endpoint = %q, want %q", res.Endpoint, server.URL+"/metrics")
	}
	if res.Session != nil {
		t.Fatalf("expected session=nil when attaching to explicit exporter")
	}
}

func TestStartRejectsInvalidExplicitEndpoint(t *testing.T) {
	res := Start(context.Background(), "://")
	if res.Available {
		t.Fatal("expected available=false")
	}
	if res.Reason != "invalid dcgm-exporter endpoint" {
		t.Fatalf("Reason = %q", res.Reason)
	}
}

func TestDefaultCollectorsCSVIncludesCriticalMetrics(t *testing.T) {
	csv := defaultCollectorsCSV()
	for _, metric := range []string{
		"DCGM_FI_DEV_GPU_UTIL",
		"DCGM_FI_DEV_FB_USED",
		"DCGM_FI_DEV_FB_FREE",
		"DCGM_FI_PROF_SM_ACTIVE",
		"DCGM_FI_PROF_SM_OCCUPANCY",
		"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE",
		"DCGM_FI_PROF_DRAM_ACTIVE",
		"DCGM_FI_PROF_PCIE_RX_BYTES",
		"DCGM_FI_PROF_PCIE_TX_BYTES",
		"DCGM_FI_PROF_NVLINK_RX_BYTES",
		"DCGM_FI_PROF_NVLINK_TX_BYTES",
		"DCGM_FI_DEV_POWER_USAGE",
		"DCGM_FI_DEV_GPU_TEMP",
		"DCGM_FI_DEV_SM_CLOCK",
		"DCGM_FI_DEV_MEM_CLOCK",
		"DCGM_FI_DEV_CLOCK_THROTTLE_REASONS",
	} {
		if !strings.Contains(csv, metric) {
			t.Fatalf("collectors CSV missing %s:\n%s", metric, csv)
		}
	}
}

func TestDCGMArgsPreferCollectorsWhenAvailable(t *testing.T) {
	args := dcgmArgs("127.0.0.1:9400", "/tmp/collectors.csv")
	if got, want := strings.Join(args[0], " "), "-a 127.0.0.1:9400 -f /tmp/collectors.csv"; got != want {
		t.Fatalf("first args = %q, want %q", got, want)
	}
	if got, want := strings.Join(args[len(args)-1], " "), "--web.listen-address=127.0.0.1:9400"; got != want {
		t.Fatalf("fallback args = %q, want %q", got, want)
	}
}
