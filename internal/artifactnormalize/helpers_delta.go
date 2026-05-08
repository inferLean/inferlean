package artifactnormalize

import (
	"sort"
	"strings"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func windowsByLabel(samples []promcollector.Sample, metricName, label string) map[string]contracts.MetricWindow {
	pointsByLabel := map[string][]contracts.MetricSample{}
	for _, sample := range samples {
		for _, metric := range sample.Metrics {
			if metric.Name != metricName {
				continue
			}
			key := strings.TrimSpace(metric.Labels[label])
			if key == "" {
				continue
			}
			pointsByLabel[key] = append(pointsByLabel[key], contracts.MetricSample{
				Timestamp: sample.Timestamp,
				Value:     metric.Value,
			})
		}
	}
	if len(pointsByLabel) == 0 {
		return nil
	}
	windows := make(map[string]contracts.MetricWindow, len(pointsByLabel))
	for key, points := range pointsByLabel {
		windows[key] = withSamples(points)
	}
	return windows
}

func deltaSnapshot(samples []promcollector.Sample, metricName string) contracts.DeltaSnapshot {
	first, last, ok := boundarySeries(samples, metricName)
	if !ok {
		return contracts.DeltaSnapshot{}
	}
	snapshot := contracts.DeltaSnapshot{}
	total := 0.0
	for key, current := range last {
		previous, ok := first[key]
		if !ok || current.Value < previous.Value {
			continue
		}
		delta := current.Value - previous.Value
		total += delta
		snapshot.Series = append(snapshot.Series, contracts.LabeledDelta{
			Labels: copyMetricLabels(current.Labels),
			Value:  delta,
		})
	}
	if len(snapshot.Series) == 0 {
		return contracts.DeltaSnapshot{}
	}
	sort.Slice(snapshot.Series, func(i, j int) bool {
		return labelKey(snapshot.Series[i].Labels) < labelKey(snapshot.Series[j].Labels)
	})
	snapshot.Total = floatPtr(total)
	return snapshot
}

func boundarySeries(samples []promcollector.Sample, metricName string) (map[string]promcollector.MetricPoint, map[string]promcollector.MetricPoint, bool) {
	firstIdx := -1
	lastIdx := -1
	for idx, sample := range samples {
		if sampleHasMetric(sample, metricName) {
			if firstIdx == -1 {
				firstIdx = idx
			}
			lastIdx = idx
		}
	}
	if firstIdx < 0 || lastIdx < 0 || firstIdx == lastIdx {
		return nil, nil, false
	}
	return metricSeries(samples[firstIdx], metricName), metricSeries(samples[lastIdx], metricName), true
}

func sampleHasMetric(sample promcollector.Sample, metricName string) bool {
	for _, metric := range sample.Metrics {
		if metric.Name == metricName {
			return true
		}
	}
	return false
}

func metricSeries(sample promcollector.Sample, metricName string) map[string]promcollector.MetricPoint {
	series := map[string]promcollector.MetricPoint{}
	for _, metric := range sample.Metrics {
		if metric.Name != metricName {
			continue
		}
		series[labelKey(metric.Labels)] = metric
	}
	return series
}

func labelKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, "\xff")
}

func copyMetricLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}
