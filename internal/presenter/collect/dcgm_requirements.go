package collect

import (
	"context"
	"fmt"
	"strings"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
)

var requiredDCGMMetrics = []string{
	"DCGM_FI_DEV_GPU_UTIL",
	"DCGM_FI_DEV_FB_USED",
	"DCGM_FI_DEV_FB_FREE",
	"DCGM_FI_PROF_SM_ACTIVE",
	"DCGM_FI_PROF_SM_OCCUPANCY",
	"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE",
	"DCGM_FI_PROF_DRAM_ACTIVE",
}

func requireDCGMSource(opts Options, sources collectionSources) error {
	if opts.AllowDCGMEstimation || sources.dcgm.Available {
		return nil
	}
	message := "dcgm-exporter is required for collection"
	if reason := strings.TrimSpace(sources.dcgm.Reason); reason != "" {
		message += ": " + reason
	}
	return fmt.Errorf("%s. Install dcgm-exporter, pass --dcgm-endpoint <url>, or pass --no-dcgm-use-estimation to allow estimated/fallback GPU telemetry", message)
}

func requireDCGMPreflight(ctx context.Context, opts Options, sources collectionSources) error {
	if opts.AllowDCGMEstimation {
		return nil
	}
	res := promcollector.NewCollector().ScrapeTargetsOnce(ctx, []promcollector.Target{
		{Name: "dcgm_exporter", Endpoint: sources.dcgm.Endpoint, Required: true},
	})
	if err := requireDCGMMetrics(opts, res); err != nil {
		return fmt.Errorf("\n\ndcgm-exporter preflight failed before collection: %w", err)
	}
	return nil
}

func requireDCGMMetricsStatus(promRes promcollector.Result) error {
	status := strings.TrimSpace(promRes.SourceStatus["dcgm_exporter"])
	if status == "" || status == "ok" {
		return nil
	}
	return fmt.Errorf("dcgm-exporter scrape did not complete successfully: %s", status)
}

func requireDCGMMetrics(opts Options, promRes promcollector.Result) error {
	if opts.AllowDCGMEstimation {
		return nil
	}
	if err := requireDCGMMetricsStatus(promRes); err != nil {
		return dcgmMetricsError(err.Error())
	}
	missing := missingDCGMMetrics(promRes.Samples["dcgm_exporter"], requiredDCGMMetrics)
	if len(missing) == 0 {
		return nil
	}
	return dcgmMetricsError("missing required DCGM metrics: " + strings.Join(missing, ", "))
}

func missingDCGMMetrics(samples []promcollector.Sample, required []string) []string {
	seen := map[string]bool{}
	for _, sample := range samples {
		for _, metric := range sample.Metrics {
			seen[metric.Name] = true
		}
	}
	missing := make([]string, 0, len(required))
	for _, name := range required {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

func dcgmMetricsError(reason string) error {
	return fmt.Errorf("%s. Enable DCGM profiling metrics or pass --no-dcgm-use-estimation to allow estimated/fallback GPU telemetry", reason)
}
