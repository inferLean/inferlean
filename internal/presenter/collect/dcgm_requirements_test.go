package collect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/exporters/dcgm"
)

func TestRequireDCGMSourceFailsWhenUnavailableByDefault(t *testing.T) {
	err := requireDCGMSource(Options{}, collectionSources{
		dcgm: dcgm.StartResult{Available: false, Reason: "dcgm-exporter not found"},
	})

	if err == nil {
		t.Fatal("expected missing dcgm-exporter error")
	}
	if !strings.Contains(err.Error(), "dcgm-exporter is required") {
		t.Fatalf("error = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--no-dcgm-use-estimation") {
		t.Fatalf("error does not mention estimation override: %q", err.Error())
	}
}

func TestRequireDCGMSourceAllowsExplicitEstimation(t *testing.T) {
	err := requireDCGMSource(Options{AllowDCGMEstimation: true}, collectionSources{
		dcgm: dcgm.StartResult{Available: false, Reason: "dcgm-exporter not found"},
	})

	if err != nil {
		t.Fatalf("expected estimation override to allow missing dcgm-exporter: %v", err)
	}
}

func TestRequireDCGMMetricsFailsWhenCriticalMetricsMissing(t *testing.T) {
	res := promcollector.Result{
		SourceStatus: map[string]string{"dcgm_exporter": "ok"},
		Samples: map[string][]promcollector.Sample{
			"dcgm_exporter": {{
				Timestamp: time.Unix(1, 0).UTC(),
				Metrics: []promcollector.MetricPoint{
					{Name: "DCGM_FI_DEV_GPU_UTIL", Value: 70},
					{Name: "DCGM_FI_DEV_FB_USED", Value: 1024},
					{Name: "DCGM_FI_DEV_FB_FREE", Value: 2048},
				},
			}},
		},
	}

	err := requireDCGMMetrics(Options{}, res)
	if err == nil {
		t.Fatal("expected missing profiler metrics error")
	}
	if !strings.Contains(err.Error(), "DCGM_FI_PROF_SM_ACTIVE") {
		t.Fatalf("error = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--no-dcgm-use-estimation") {
		t.Fatalf("error does not mention estimation override: %q", err.Error())
	}
}

func TestRequireDCGMMetricsAllowsExplicitEstimation(t *testing.T) {
	res := promcollector.Result{
		SourceStatus: map[string]string{"dcgm_exporter": "ok"},
		Samples:      map[string][]promcollector.Sample{},
	}

	if err := requireDCGMMetrics(Options{AllowDCGMEstimation: true}, res); err != nil {
		t.Fatalf("expected estimation override to allow missing profiler metrics: %v", err)
	}
}

func TestRequireDCGMPreflightFailsBeforeCollectionWhenCriticalMetricsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`
DCGM_FI_DEV_GPU_UTIL 70
DCGM_FI_DEV_FB_USED 1024
DCGM_FI_DEV_FB_FREE 2048
`))
	}))
	t.Cleanup(server.Close)

	err := requireDCGMPreflight(context.Background(), Options{}, collectionSources{
		dcgm: dcgm.StartResult{Available: true, Endpoint: server.URL},
	})
	if err == nil {
		t.Fatal("expected preflight missing profiler metrics error")
	}
	if !strings.Contains(err.Error(), "preflight failed before collection") {
		t.Fatalf("error does not mention preflight timing: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "DCGM_FI_PROF_SM_ACTIVE") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestRequireDCGMPreflightAcceptsCriticalMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for _, name := range requiredDCGMMetrics {
			_, _ = w.Write([]byte(name + " 1\n"))
		}
	}))
	t.Cleanup(server.Close)

	err := requireDCGMPreflight(context.Background(), Options{}, collectionSources{
		dcgm: dcgm.StartResult{Available: true, Endpoint: server.URL},
	})
	if err != nil {
		t.Fatalf("expected preflight to pass with critical metrics: %v", err)
	}
}

func TestRequireDCGMPreflightAllowsExplicitEstimation(t *testing.T) {
	err := requireDCGMPreflight(context.Background(), Options{AllowDCGMEstimation: true}, collectionSources{})
	if err != nil {
		t.Fatalf("expected estimation override to skip preflight: %v", err)
	}
}

func TestRequireDCGMMetricsAcceptsCriticalMetrics(t *testing.T) {
	points := make([]promcollector.MetricPoint, 0, len(requiredDCGMMetrics))
	for _, name := range requiredDCGMMetrics {
		points = append(points, promcollector.MetricPoint{Name: name, Value: 1})
	}
	res := promcollector.Result{
		SourceStatus: map[string]string{"dcgm_exporter": "ok"},
		Samples: map[string][]promcollector.Sample{
			"dcgm_exporter": {{
				Timestamp: time.Unix(1, 0).UTC(),
				Metrics:   points,
			}},
		},
	}

	if err := requireDCGMMetrics(Options{}, res); err != nil {
		t.Fatalf("expected critical dcgm metrics to pass: %v", err)
	}
}
