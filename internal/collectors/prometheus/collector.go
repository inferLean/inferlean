package prometheus

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strconv"
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
	RawText      string
	RawByTarget  map[string]string
	Samples      map[string][]Sample
	SourceStatus map[string]string
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
	if len(cleanTargets) == 0 || collectFor <= 0 || every <= 0 {
		return result
	}
	runtime := tryStartRuntime(ctx, cleanTargets, every)
	if runtime != nil {
		defer runtime.Stop(context.Background())
	}
	deadline := time.Now().Add(collectFor)
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
	scan := bufio.NewScanner(resp.Body)
	parsed := make([]MetricPoint, 0)
	lines := make([]string, 0)
	for scan.Scan() {
		line := scan.Text()
		lines = append(lines, line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		point, ok := parseMetricLine(line)
		if ok {
			parsed = append(parsed, point)
		}
	}
	return strings.Join(lines, "\n"), parsed, nil
}

func parseMetricLine(line string) (MetricPoint, bool) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return MetricPoint{}, false
	}
	value, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return MetricPoint{}, false
	}
	name, labels := parseMetricToken(parts[0])
	if strings.TrimSpace(name) == "" {
		return MetricPoint{}, false
	}
	return MetricPoint{Name: name, Labels: labels, Value: value}, true
}

func parseMetricToken(token string) (string, map[string]string) {
	openIdx := strings.Index(token, "{")
	if openIdx < 0 {
		return token, nil
	}
	closeIdx := strings.LastIndex(token, "}")
	if closeIdx <= openIdx {
		return token, nil
	}
	name := strings.TrimSpace(token[:openIdx])
	if name == "" {
		return "", nil
	}
	labelsPart := token[openIdx+1 : closeIdx]
	return name, parseLabels(labelsPart)
}

func parseLabels(text string) map[string]string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	labels := map[string]string{}
	for _, pair := range splitLabelPairs(text) {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, "\"")
		}
		labels[key] = value
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func splitLabelPairs(text string) []string {
	pairs := make([]string, 0)
	var current strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range text {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			inQuotes = !inQuotes
			current.WriteRune(r)
			continue
		}
		if r == ',' && !inQuotes {
			if strings.TrimSpace(current.String()) != "" {
				pairs = append(pairs, strings.TrimSpace(current.String()))
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if strings.TrimSpace(current.String()) != "" {
		pairs = append(pairs, strings.TrimSpace(current.String()))
	}
	return pairs
}
