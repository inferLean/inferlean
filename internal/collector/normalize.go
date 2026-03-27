package collector

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type metricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type labeledSeries struct {
	Labels map[string]string `json:"labels"`
	Points []metricPoint     `json:"points"`
}

func windowFromPoints(points []metricPoint) contracts.MetricWindow {
	if len(points) == 0 {
		return contracts.MetricWindow{}
	}

	minValue, maxValue := points[0].Value, points[0].Value
	sum := 0.0
	samples := make([]contracts.MetricSample, 0, len(points))
	for _, point := range points {
		if point.Value < minValue {
			minValue = point.Value
		}
		if point.Value > maxValue {
			maxValue = point.Value
		}
		sum += point.Value
		samples = append(samples, contracts.MetricSample{
			Timestamp: point.Timestamp.UTC(),
			Value:     point.Value,
		})
	}

	latest := points[len(points)-1].Value
	avg := sum / float64(len(points))
	return contracts.MetricWindow{
		Latest:  floatPointer(latest),
		Min:     floatPointer(minValue),
		Max:     floatPointer(maxValue),
		Avg:     floatPointer(avg),
		Samples: samples,
	}
}

func latestHistogram(series []labeledSeries) contracts.DistributionSnapshot {
	if len(series) == 0 {
		return contracts.DistributionSnapshot{}
	}

	count := seriesValue(series, "_count")
	sum := seriesValue(series, "_sum")
	buckets := histogramBuckets(series)
	return contracts.DistributionSnapshot{
		Count:   count,
		Sum:     sum,
		Buckets: buckets,
	}
}

func seriesValue(series []labeledSeries, suffix string) *float64 {
	for _, entry := range series {
		if name := entry.Labels["__name__"]; len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix {
			if len(entry.Points) == 0 {
				return nil
			}
			return floatPointer(entry.Points[len(entry.Points)-1].Value)
		}
	}
	return nil
}

func histogramBuckets(series []labeledSeries) []contracts.DistributionBucket {
	var buckets []contracts.DistributionBucket
	for _, entry := range series {
		if entry.Labels["le"] == "" || len(entry.Points) == 0 {
			continue
		}
		buckets = append(buckets, contracts.DistributionBucket{
			UpperBound: entry.Labels["le"],
			Value:      entry.Points[len(entry.Points)-1].Value,
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return bucketOrder(buckets[i].UpperBound) < bucketOrder(buckets[j].UpperBound)
	})
	return buckets
}

func bucketOrder(value string) float64 {
	if value == "+Inf" {
		return math.Inf(1)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return math.MaxFloat64
	}
	return parsed
}

func cacheSnapshot(hits, queries contracts.MetricWindow) contracts.CacheSnapshot {
	cache := contracts.CacheSnapshot{Hits: hits.Latest, Queries: queries.Latest}
	if hits.Latest != nil && queries.Latest != nil && *queries.Latest > 0 {
		cache.HitRate = floatPointer(*hits.Latest / *queries.Latest)
	}
	return cache
}

func floatPointer(value float64) *float64 {
	clone := value
	return &clone
}
