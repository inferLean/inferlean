package prometheus

import (
	"math"
	"strconv"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

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
