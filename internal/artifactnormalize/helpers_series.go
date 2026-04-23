package artifactnormalize

import (
	"math"
	"strings"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func windowFromMetric(samples []promcollector.Sample, metricName string) contracts.MetricWindow {
	points := make([]contracts.MetricSample, 0, len(samples))
	for _, sample := range samples {
		value, ok := metricValue(sample.Metrics, metricName)
		if !ok {
			continue
		}
		points = append(points, contracts.MetricSample{Timestamp: sample.Timestamp, Value: value})
	}
	return withSamples(points)
}

func histogramMeanWindow(samples []promcollector.Sample, prefix string) contracts.MetricWindow {
	sumMetric := prefix + "_sum"
	countMetric := prefix + "_count"
	points := make([]contracts.MetricSample, 0, len(samples))
	for _, sample := range samples {
		sum, okSum := metricValue(sample.Metrics, sumMetric)
		count, okCount := metricValue(sample.Metrics, countMetric)
		if !okSum || !okCount || count <= 0 {
			continue
		}
		points = append(points, contracts.MetricSample{Timestamp: sample.Timestamp, Value: sum / count})
	}
	return withSamples(points)
}

func histogramDistribution(samples []promcollector.Sample, prefix string) contracts.DistributionSnapshot {
	if len(samples) == 0 {
		return contracts.DistributionSnapshot{}
	}
	latest := samples[len(samples)-1]
	count, _ := metricValue(latest.Metrics, prefix+"_count")
	sum, _ := metricValue(latest.Metrics, prefix+"_sum")
	buckets := make([]contracts.DistributionBucket, 0)
	for _, metric := range latest.Metrics {
		if metric.Name != prefix+"_bucket" {
			continue
		}
		upperBound := strings.TrimSpace(metric.Labels["le"])
		if upperBound == "" {
			continue
		}
		buckets = append(buckets, contracts.DistributionBucket{UpperBound: upperBound, Value: metric.Value})
	}
	return contracts.DistributionSnapshot{
		Count:   floatPtr(count),
		Sum:     floatPtr(sum),
		Buckets: buckets,
	}
}

func cacheSnapshot(samples []promcollector.Sample, hitMetric, queryMetric string) contracts.CacheSnapshot {
	if len(samples) == 0 {
		return contracts.CacheSnapshot{}
	}
	latest := samples[len(samples)-1]
	hits, okHits := metricValue(latest.Metrics, hitMetric)
	queries, okQueries := metricValue(latest.Metrics, queryMetric)
	if !okHits && !okQueries {
		return contracts.CacheSnapshot{}
	}
	cache := contracts.CacheSnapshot{}
	if okHits {
		cache.Hits = floatPtr(hits)
	}
	if okQueries {
		cache.Queries = floatPtr(queries)
	}
	if okHits && okQueries && queries > 0 && !math.IsNaN(hits/queries) {
		cache.HitRate = floatPtr(hits / queries)
	}
	return cache
}

func metricValue(metrics []promcollector.MetricPoint, name string) (float64, bool) {
	value := 0.0
	found := false
	for _, metric := range metrics {
		if metric.Name != name {
			continue
		}
		value += metric.Value
		found = true
	}
	return value, found
}

func metricValueWithLabel(metrics []promcollector.MetricPoint, name, label, want string) (float64, bool) {
	value := 0.0
	found := false
	for _, metric := range metrics {
		if metric.Name != name {
			continue
		}
		if metric.Labels[label] != want {
			continue
		}
		value += metric.Value
		found = true
	}
	return value, found
}

func latestMetricValue(samples []promcollector.Sample, name string) (float64, bool) {
	if len(samples) == 0 {
		return 0, false
	}
	return metricValue(samples[len(samples)-1].Metrics, name)
}

func deltaRateWindow(samples []promcollector.Sample, name string, scale float64) contracts.MetricWindow {
	points := make([]contracts.MetricSample, 0, len(samples))
	for idx := 1; idx < len(samples); idx++ {
		current, okCurrent := metricValue(samples[idx].Metrics, name)
		previous, okPrevious := metricValue(samples[idx-1].Metrics, name)
		if !okCurrent || !okPrevious {
			continue
		}
		deltaTime := samples[idx].Timestamp.Sub(samples[idx-1].Timestamp).Seconds()
		if deltaTime <= 0 {
			continue
		}
		if current < previous {
			continue
		}
		points = append(points, contracts.MetricSample{
			Timestamp: samples[idx].Timestamp,
			Value:     ((current - previous) / deltaTime) * scale,
		})
	}
	return withSamples(points)
}

func memoryWindows(usedBytes, totalBytes contracts.MetricWindow) contracts.MemoryMetrics {
	memory := contracts.MemoryMetrics{Used: usedBytes, Total: totalBytes}
	if usedBytes.Latest != nil && totalBytes.Latest != nil {
		free := *totalBytes.Latest - *usedBytes.Latest
		memory.Free = withSamples([]contracts.MetricSample{{Timestamp: time.Now().UTC(), Value: free}})
	}
	return memory
}
