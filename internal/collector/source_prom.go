package collector

import (
	"context"
	"sort"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type windowResult struct {
	Window     contracts.MetricWindow `json:"window"`
	Expression string                 `json:"expression,omitempty"`
}

type distributionResult struct {
	Distribution contracts.DistributionSnapshot `json:"distribution"`
	BaseMetric   string                         `json:"base_metric,omitempty"`
}

func (s Service) firstWindow(ctx context.Context, promBase string, start, end time.Time, step time.Duration, exprs []string) (windowResult, bool, error) {
	for _, expr := range exprs {
		series, err := s.queryRange(ctx, promBase, expr, start, end, step)
		if err != nil {
			return windowResult{}, false, err
		}
		points := mergeSeriesPoints(series)
		if len(points) == 0 {
			continue
		}
		return windowResult{Window: windowFromPoints(points), Expression: expr}, true, nil
	}
	return windowResult{}, false, nil
}

func (s Service) firstHistogram(ctx context.Context, promBase string, end time.Time, bases []string) (distributionResult, bool, error) {
	for _, base := range bases {
		hist, ok, err := s.loadHistogram(ctx, promBase, end, base)
		if err != nil {
			return distributionResult{}, false, err
		}
		if ok {
			return distributionResult{Distribution: hist, BaseMetric: base}, true, nil
		}
	}
	return distributionResult{}, false, nil
}

func (s Service) loadHistogram(ctx context.Context, promBase string, end time.Time, base string) (contracts.DistributionSnapshot, bool, error) {
	buckets, err := s.queryInstant(ctx, promBase, "sum("+base+"_bucket) by (le)", end)
	if err != nil {
		return contracts.DistributionSnapshot{}, false, err
	}
	count, err := s.queryInstant(ctx, promBase, "sum("+base+"_count)", end)
	if err != nil {
		return contracts.DistributionSnapshot{}, false, err
	}
	sum, err := s.queryInstant(ctx, promBase, "sum("+base+"_sum)", end)
	if err != nil {
		return contracts.DistributionSnapshot{}, false, err
	}
	if len(buckets) == 0 && len(count) == 0 && len(sum) == 0 {
		return contracts.DistributionSnapshot{}, false, nil
	}

	return contracts.DistributionSnapshot{
		Count:   seriesLatest(count),
		Sum:     seriesLatest(sum),
		Buckets: distributionBuckets(buckets),
	}, true, nil
}

func mergeSeriesPoints(series []labeledSeries) []metricPoint {
	byTimestamp := map[time.Time]float64{}
	for _, entry := range series {
		for _, point := range entry.Points {
			byTimestamp[point.Timestamp] += point.Value
		}
	}

	points := make([]metricPoint, 0, len(byTimestamp))
	for timestamp, value := range byTimestamp {
		points = append(points, metricPoint{Timestamp: timestamp, Value: value})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	return points
}

func seriesLatest(series []labeledSeries) *float64 {
	points := mergeSeriesPoints(series)
	if len(points) == 0 {
		return nil
	}
	return floatPointer(points[len(points)-1].Value)
}

func distributionBuckets(series []labeledSeries) []contracts.DistributionBucket {
	buckets := make([]contracts.DistributionBucket, 0, len(series))
	for _, entry := range series {
		if len(entry.Points) == 0 {
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
