package prometheus

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

type Target struct {
	Name     string
	Endpoint string
	Required bool
}

type Collector struct {
	client *http.Client
}

type MetricPoint struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

type Sample struct {
	Timestamp time.Time     `json:"timestamp"`
	Metrics   []MetricPoint `json:"metrics"`
}

type Result struct {
	RawText        string
	RawByTarget    map[string]string
	Samples        map[string][]Sample
	SourceStatus   map[string]string
	StartedAt      time.Time
	FinishedAt     time.Time
	ScrapeInterval time.Duration
}

func NewCollector() Collector {
	return Collector{client: &http.Client{Timeout: 5 * time.Second}}
}

func (c Collector) Collect(ctx context.Context, endpoint string, collectFor, every time.Duration) Result {
	targets := []Target{{Name: "vllm", Endpoint: endpoint, Required: true}}
	return c.CollectTargets(ctx, targets, collectFor, every)
}

func (c Collector) CollectTargets(ctx context.Context, targets []Target, collectFor, every time.Duration) Result {
	cleanTargets := sanitizeTargets(targets)
	result := initResult(cleanTargets)
	result.ScrapeInterval = every
	if len(cleanTargets) == 0 || collectFor <= 0 || every <= 0 {
		return result
	}
	runtime, runtimeReason := tryStartRuntime(ctx, cleanTargets, every)
	if runtime != nil {
		result.SourceStatus["prometheus_runtime"] = "ok"
		defer runtime.Stop(context.Background())
	} else if strings.TrimSpace(runtimeReason) != "" {
		result.SourceStatus["prometheus_runtime"] = "degraded: " + runtimeReason
	}
	result.StartedAt = time.Now().UTC()
	deadline := result.StartedAt.Add(collectFor)
	defer func() {
		result.FinishedAt = time.Now().UTC()
	}()
	for {
		c.scrapeOnce(ctx, cleanTargets, &result)
		if time.Now().After(deadline) {
			break
		}
		if !waitForNext(ctx, every) {
			break
		}
	}
	return result
}

func sanitizeTargets(targets []Target) []Target {
	out := make([]Target, 0, len(targets))
	for _, target := range targets {
		name := strings.TrimSpace(target.Name)
		endpoint := strings.TrimSpace(target.Endpoint)
		if name == "" || endpoint == "" {
			continue
		}
		out = append(out, Target{Name: name, Endpoint: endpoint, Required: target.Required})
	}
	return out
}

func initResult(targets []Target) Result {
	result := Result{
		RawByTarget:  map[string]string{},
		Samples:      map[string][]Sample{},
		SourceStatus: map[string]string{},
	}
	for _, target := range targets {
		result.SourceStatus[target.Name] = "missing"
	}
	return result
}

func (c Collector) scrapeOnce(ctx context.Context, targets []Target, result *Result) {
	for _, target := range targets {
		scrapeTS := time.Now().UTC()
		metricText, parsed, err := c.fetch(ctx, target.Endpoint)
		if err != nil {
			continue
		}
		appendRaw(result, target.Name, scrapeTS, metricText)
		if len(parsed) == 0 {
			markDegraded(result, target.Name)
			continue
		}
		result.Samples[target.Name] = append(result.Samples[target.Name], Sample{
			Timestamp: scrapeTS,
			Metrics:   parsed,
		})
		result.SourceStatus[target.Name] = "ok"
	}
}

func appendRaw(result *Result, targetName string, timestamp time.Time, metricText string) {
	block := fmt.Sprintf("# scrape_ts=%s target=%s\n%s\n", timestamp.Format(time.RFC3339Nano), targetName, metricText)
	result.RawText += block
	result.RawByTarget[targetName] += block
}

func markDegraded(result *Result, targetName string) {
	if result.SourceStatus[targetName] == "ok" {
		return
	}
	result.SourceStatus[targetName] = "degraded"
}

func waitForNext(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (c Collector) fetch(ctx context.Context, endpoint string) (string, []MetricPoint, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("status=%s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	parsed, err := parseMetrics(string(body))
	if err != nil {
		return string(body), nil, err
	}
	return string(body), parsed, nil
}

func parseMetrics(text string) ([]MetricPoint, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	points := make([]MetricPoint, 0)
	for name, family := range families {
		for _, metric := range family.GetMetric() {
			points = append(points, flattenMetric(name, metric)...)
		}
	}
	return points, nil
}

func flattenMetric(name string, metric *dto.Metric) []MetricPoint {
	switch {
	case metric.Histogram != nil:
		return flattenHistogram(name, metric)
	case metric.Summary != nil:
		return flattenSummary(name, metric)
	default:
		value, ok := scalarMetricValue(metric)
		if !ok {
			return nil
		}
		return []MetricPoint{{
			Name:   name,
			Labels: metricLabels(metric),
			Value:  value,
		}}
	}
}

func flattenHistogram(name string, metric *dto.Metric) []MetricPoint {
	histogram := metric.GetHistogram()
	labels := metricLabels(metric)
	points := make([]MetricPoint, 0, len(histogram.GetBucket())+2)
	for _, bucket := range histogram.GetBucket() {
		points = append(points, MetricPoint{
			Name:   name + "_bucket",
			Labels: labelsWith(labels, "le", formatPrometheusBound(bucket.GetUpperBound())),
			Value:  float64(bucket.GetCumulativeCount()),
		})
	}
	points = append(points, MetricPoint{
		Name:   name + "_count",
		Labels: copyLabels(labels),
		Value:  float64(histogram.GetSampleCount()),
	})
	points = append(points, MetricPoint{
		Name:   name + "_sum",
		Labels: copyLabels(labels),
		Value:  histogram.GetSampleSum(),
	})
	return points
}

func flattenSummary(name string, metric *dto.Metric) []MetricPoint {
	summary := metric.GetSummary()
	labels := metricLabels(metric)
	points := make([]MetricPoint, 0, len(summary.GetQuantile())+2)
	for _, quantile := range summary.GetQuantile() {
		points = append(points, MetricPoint{
			Name:   name,
			Labels: labelsWith(labels, "quantile", strconv.FormatFloat(quantile.GetQuantile(), 'g', -1, 64)),
			Value:  quantile.GetValue(),
		})
	}
	points = append(points, MetricPoint{
		Name:   name + "_count",
		Labels: copyLabels(labels),
		Value:  float64(summary.GetSampleCount()),
	})
	points = append(points, MetricPoint{
		Name:   name + "_sum",
		Labels: copyLabels(labels),
		Value:  summary.GetSampleSum(),
	})
	return points
}

func metricLabels(metric *dto.Metric) map[string]string {
	if len(metric.GetLabel()) == 0 {
		return nil
	}
	labels := map[string]string{}
	for _, label := range metric.GetLabel() {
		if strings.TrimSpace(label.GetName()) != "" {
			labels[label.GetName()] = label.GetValue()
		}
	}
	return labels
}

func labelsWith(labels map[string]string, key, value string) map[string]string {
	out := copyLabels(labels)
	if out == nil {
		out = map[string]string{}
	}
	out[key] = value
	return out
}

func copyLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}

func formatPrometheusBound(value float64) string {
	if math.IsInf(value, 1) {
		return "+Inf"
	}
	if math.IsInf(value, -1) {
		return "-Inf"
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func scalarMetricValue(metric *dto.Metric) (float64, bool) {
	switch {
	case metric.Gauge != nil:
		return metric.GetGauge().GetValue(), true
	case metric.Counter != nil:
		return metric.GetCounter().GetValue(), true
	case metric.Untyped != nil:
		return metric.GetUntyped().GetValue(), true
	default:
		return 0, false
	}
}
