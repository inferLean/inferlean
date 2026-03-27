package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

func writePrometheusConfig(path string, scrapeEvery time.Duration, vllmTarget, nodeTarget, dcgmTarget string) error {
	var b strings.Builder
	b.WriteString("global:\n")
	b.WriteString(fmt.Sprintf("  scrape_interval: %s\n", scrapeEvery))
	b.WriteString(fmt.Sprintf("  evaluation_interval: %s\n", scrapeEvery))
	b.WriteString("scrape_configs:\n")
	for _, job := range buildScrapeJobs(vllmTarget, nodeTarget, dcgmTarget) {
		writeScrapeJob(&b, job[0], job[1])
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write prometheus config: %w", err)
	}
	return nil
}

func buildScrapeJobs(vllmTarget, nodeTarget, dcgmTarget string) [][2]string {
	jobs := [][2]string{{"node_exporter", nodeTarget}}
	if vllmTarget != "" {
		jobs = append(jobs, [2]string{"vllm", vllmTarget})
	}
	if dcgmTarget != "" {
		jobs = append(jobs, [2]string{"dcgm", dcgmTarget})
	}
	return jobs
}

func writeScrapeJob(b *strings.Builder, name, target string) {
	b.WriteString(fmt.Sprintf("  - job_name: %q\n", name))
	b.WriteString("    static_configs:\n")
	b.WriteString("      - targets:\n")
	b.WriteString(fmt.Sprintf("          - %q\n", target))
}

func (s Service) waitForPrometheusReady(ctx context.Context, baseURL string) error {
	return waitForDeadline(healthTimeout, 250*time.Millisecond, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/-/ready", nil)
		if err != nil {
			return false
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, "prometheus did not become ready in time")
}

func (s Service) waitForPrometheusTargets(ctx context.Context, baseURL string, jobs []string) error {
	if len(jobs) == 0 {
		return nil
	}
	required := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		required[job] = struct{}{}
	}

	return waitForDeadline(healthTimeout, 500*time.Millisecond, func() bool {
		healthy, err := s.healthyPrometheusJobs(ctx, baseURL, required)
		return err == nil && len(healthy) == len(required)
	}, "not all scrape targets became healthy")
}

func waitForDeadline(timeout, interval time.Duration, ready func() bool, message string) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return nil
		}
		time.Sleep(interval)
	}
	return errors.New(message)
}

func (s Service) healthyPrometheusJobs(ctx context.Context, baseURL string, required map[string]struct{}) (map[string]bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/targets", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body promTargetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Status != "success" {
		return nil, err
	}

	healthy := map[string]bool{}
	for _, target := range body.Data.ActiveTargets {
		job := target.Labels["job"]
		if _, ok := required[job]; ok && target.Health == "up" {
			healthy[job] = true
		}
	}
	return healthy, nil
}

func (s Service) captureMetricsSource(ctx context.Context, promBase, job, rawURL, rawPath string) sourceCapture {
	rel := relativeRawArtifact(rawPath)
	capture := sourceCapture{Status: "ok", Artifacts: []string{rel}}

	if err := fetchToFile(ctx, s.client, rawURL, rawPath); err != nil {
		capture.Status = "degraded"
		capture.Reason = fmt.Sprintf("could not capture raw metrics: %v", err)
	}

	series, err := s.queryJobVector(ctx, promBase, job)
	if err != nil {
		capture.Status = "degraded"
		capture.Reason = fmt.Sprintf("could not query Prometheus for %s: %v", job, err)
		return capture
	}
	if len(series) == 0 {
		capture.Status = "missing"
		capture.Reason = fmt.Sprintf("Prometheus did not return any series for job %s", job)
		return capture
	}

	capture.MetricPayload = map[string]any{
		"job":              job,
		"raw_evidence_ref": rel,
		"metric_names":     sortedKeys(series),
		"series":           series,
		"series_count":     len(series),
	}
	return capture
}

func fetchToFile(ctx context.Context, client *http.Client, rawURL, path string) error {
	if rawURL == "" {
		return errors.New("raw metrics URL was empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return writeResponse(path, resp.Body)
}

func writeResponse(path string, body io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, body)
	return err
}

func (s Service) queryJobVector(ctx context.Context, promBase, job string) (map[string]any, error) {
	query := url.QueryEscape(fmt.Sprintf("{job=%q}", job))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, promBase+"/api/v1/query?query="+query, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var body promVectorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Status != "success" {
		return nil, err
	}
	return buildSeriesMap(body.Data.Result), nil
}

func buildSeriesMap(results []struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}) map[string]any {
	seriesMap := map[string]any{}
	for _, series := range results {
		name := series.Metric["__name__"]
		if name == "" {
			name = "unknown"
		}
		entry := metricSample(series.Metric, series.Value)
		current, _ := seriesMap[name].([]map[string]any)
		seriesMap[name] = append(current, entry)
	}
	return seriesMap
}

func metricSample(labels map[string]string, value []any) map[string]any {
	sample := map[string]any{"labels": stripMetricName(labels)}
	if len(value) == 2 {
		if timestamp, ok := value[0].(float64); ok {
			sample["timestamp"] = time.Unix(int64(timestamp), 0).UTC().Format(time.RFC3339)
		}
		if raw, ok := value[1].(string); ok {
			sample["value"] = raw
		}
	}
	return sample
}

func stripMetricName(labels map[string]string) map[string]string {
	clean := map[string]string{}
	for key, value := range labels {
		if key != "__name__" {
			clean[key] = value
		}
	}
	return clean
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
