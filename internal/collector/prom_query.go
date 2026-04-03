package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (s Service) queryRange(ctx context.Context, promBase, expr string, start, end time.Time, step time.Duration) ([]labeledSeries, error) {
	queryURL := fmt.Sprintf(
		"%s/api/v1/query_range?query=%s&start=%s&end=%s&step=%s",
		promBase,
		url.QueryEscape(expr),
		url.QueryEscape(start.UTC().Format(time.RFC3339)),
		url.QueryEscape(end.UTC().Format(time.RFC3339)),
		url.QueryEscape(step.String()),
	)
	return s.fetchRangeSeries(ctx, queryURL)
}

func (s Service) queryInstant(ctx context.Context, promBase, expr string, ts time.Time) ([]labeledSeries, error) {
	queryURL := fmt.Sprintf(
		"%s/api/v1/query?query=%s&time=%s",
		promBase,
		url.QueryEscape(expr),
		url.QueryEscape(ts.UTC().Format(time.RFC3339)),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
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
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return vectorSeries(body), nil
}

func (s Service) fetchRangeSeries(ctx context.Context, queryURL string) ([]labeledSeries, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
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

	var body promRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return matrixSeries(body), nil
}

func vectorSeries(body promVectorResponse) []labeledSeries {
	series := make([]labeledSeries, 0, len(body.Data.Result))
	for _, result := range body.Data.Result {
		points := parsePoints([][]any{result.Value})
		series = append(series, labeledSeries{
			Labels: cloneLabels(result.Metric),
			Points: points,
		})
	}
	return series
}

func matrixSeries(body promRangeResponse) []labeledSeries {
	series := make([]labeledSeries, 0, len(body.Data.Result))
	for _, result := range body.Data.Result {
		series = append(series, labeledSeries{
			Labels: cloneLabels(result.Metric),
			Points: parsePoints(result.Values),
		})
	}
	return series
}

func parsePoints(values [][]any) []metricPoint {
	points := make([]metricPoint, 0, len(values))
	for _, value := range values {
		if len(value) != 2 {
			continue
		}
		timestamp, ok := value[0].(float64)
		if !ok {
			continue
		}
		number, err := parsePromFloat(value[1])
		if err != nil {
			continue
		}
		points = append(points, metricPoint{
			Timestamp: time.Unix(int64(timestamp), 0).UTC(),
			Value:     number,
		})
	}
	return points
}

func parsePromFloat(value any) (float64, error) {
	raw, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("unexpected prometheus value type %T", value)
	}
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("non-finite prometheus value %q", raw)
	}
	return number, nil
}

func cloneLabels(labels map[string]string) map[string]string {
	clone := make(map[string]string, len(labels))
	for key, value := range labels {
		clone[key] = value
	}
	return clone
}
