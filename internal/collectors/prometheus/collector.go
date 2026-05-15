package prometheus

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

func (c Collector) ScrapeTargetsOnce(ctx context.Context, targets []Target) Result {
	cleanTargets := sanitizeTargets(targets)
	result := initResult(cleanTargets)
	if len(cleanTargets) == 0 {
		return result
	}
	result.StartedAt = time.Now().UTC()
	c.scrapeOnce(ctx, cleanTargets, &result)
	result.FinishedAt = time.Now().UTC()
	return result
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
	for {
		c.scrapeOnce(ctx, cleanTargets, &result)
		if time.Now().After(deadline) {
			break
		}
		if !waitForNext(ctx, every) {
			break
		}
	}
	result.FinishedAt = time.Now().UTC()
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
