package contracts

import (
	"errors"
	"fmt"
	"strings"
)

func (r FinalReport) Validate() error {
	var errs []error

	if strings.TrimSpace(r.SchemaVersion) == "" {
		errs = append(errs, errors.New("schema_version is required"))
	} else if r.SchemaVersion != ReportSchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version must be %s", ReportSchemaVersion))
	}
	if strings.TrimSpace(r.Job.RunID) == "" {
		errs = append(errs, errors.New("job.run_id is required"))
	}
	switch r.Entitlement.Tier {
	case "free", "paid":
	default:
		errs = append(errs, errors.New("entitlement.tier must be free or paid"))
	}

	errs = append(errs, validateIssues(r.Issues)...)
	errs = append(errs, validateOpportunities(r.Opportunities)...)
	errs = append(errs, validateSaturation(r.Saturation)...)
	errs = append(errs, validateReportCoverage(r.DiagnosticCoverage)...)
	errs = append(errs, validateVLLMCommandReplacement(r.VLLMCommandReplacement)...)

	return errors.Join(errs...)
}

func validateVLLMCommandReplacement(replacement *VLLMCommandReplacement) []error {
	if replacement == nil {
		return nil
	}

	var errs []error
	if strings.TrimSpace(replacement.CurrentCommand) == "" {
		errs = append(errs, errors.New("vllm_command_replacement.current_command is required"))
	}
	if strings.TrimSpace(replacement.RecommendedCommand) == "" && len(replacement.Warnings) == 0 {
		errs = append(errs, errors.New("vllm_command_replacement.recommended_command or warnings is required"))
	}
	return errs
}

func validateSaturation(s SaturationReport) []error {
	var errs []error
	if strings.TrimSpace(s.Version) == "" {
		errs = append(errs, errors.New("saturation.version is required"))
	}
	errs = append(errs, validateSaturationMetric("saturation.generic", s.Generic, true)...)
	seen := make(map[string]struct{}, len(s.Dimensions))
	for idx, dimension := range s.Dimensions {
		field := fmt.Sprintf("saturation.dimensions[%d]", idx)
		errs = append(errs, validateSaturationMetric(field, dimension, false)...)
		if strings.TrimSpace(dimension.ID) == "" {
			continue
		}
		if _, ok := seen[dimension.ID]; ok {
			errs = append(errs, fmt.Errorf("%s.id must be unique", field))
		}
		seen[dimension.ID] = struct{}{}
	}
	return errs
}

func validateSaturationMetric(field string, metric SaturationMetric, generic bool) []error {
	var errs []error
	if strings.TrimSpace(metric.ID) == "" {
		errs = append(errs, fmt.Errorf("%s.id is required", field))
	}
	switch metric.Status {
	case "ok", "partial", "not_evaluable":
	default:
		errs = append(errs, fmt.Errorf("%s.status must be ok, partial, or not_evaluable", field))
	}
	if metric.Status != "not_evaluable" && !metric.Score.HasData() {
		errs = append(errs, fmt.Errorf("%s.score is required when evaluable", field))
	}
	if metric.Status == "not_evaluable" && strings.TrimSpace(metric.Reason) == "" {
		errs = append(errs, fmt.Errorf("%s.reason is required when not evaluable", field))
	}
	if !generic && strings.TrimSpace(metric.BottleneckType) == "" {
		errs = append(errs, fmt.Errorf("%s.bottleneck_type is required", field))
	}
	return errs
}

func validateIssues(issues []Issue) []error {
	if len(issues) == 0 {
		return nil
	}

	rankOne := 0
	ranks := make(map[int]struct{}, len(issues))
	ids := make(map[string]struct{}, len(issues))
	var errs []error
	for idx, issue := range issues {
		if strings.TrimSpace(issue.ID) == "" {
			errs = append(errs, fmt.Errorf("issues[%d].id is required", idx))
		}
		if strings.TrimSpace(issue.DetectorID) == "" {
			errs = append(errs, fmt.Errorf("issues[%d].detector_id is required", idx))
		}
		if issue.Rank == 1 {
			rankOne++
		}
		if issue.Rank < 1 {
			errs = append(errs, fmt.Errorf("issues[%d].rank must be positive", idx))
		} else if _, ok := ranks[issue.Rank]; ok {
			errs = append(errs, fmt.Errorf("issues[%d].rank must be unique", idx))
		}
		ranks[issue.Rank] = struct{}{}
		if _, ok := ids[issue.ID]; ok {
			errs = append(errs, fmt.Errorf("issues[%d].id must be unique", idx))
		}
		ids[issue.ID] = struct{}{}
		if issue.Recommendation == nil {
			errs = append(errs, fmt.Errorf("issues[%d].recommendation is required", idx))
		} else {
			errs = append(errs, validateRecommendation(issue.Recommendation, fmt.Sprintf("issues[%d].recommendation", idx))...)
			if len(issue.Recommendation.Actions)+len(issue.Recommendation.FollowUpSteps) == 0 {
				errs = append(errs, fmt.Errorf("issues[%d].recommendation.actions or follow_up_steps is required", idx))
			}
		}
	}
	if rankOne != 1 {
		errs = append(errs, errors.New("exactly one issue must have rank 1"))
	}
	return errs
}

func validateOpportunities(opportunities []Opportunity) []error {
	if len(opportunities) == 0 {
		return nil
	}

	rankOne := 0
	ranks := make(map[int]struct{}, len(opportunities))
	ids := make(map[string]struct{}, len(opportunities))
	var errs []error
	for idx, opportunity := range opportunities {
		if strings.TrimSpace(opportunity.ID) == "" {
			errs = append(errs, fmt.Errorf("opportunities[%d].id is required", idx))
		}
		if strings.TrimSpace(opportunity.DetectorID) == "" {
			errs = append(errs, fmt.Errorf("opportunities[%d].detector_id is required", idx))
		}
		if opportunity.Rank == 1 {
			rankOne++
		}
		if opportunity.Rank < 1 {
			errs = append(errs, fmt.Errorf("opportunities[%d].rank must be positive", idx))
		} else if _, ok := ranks[opportunity.Rank]; ok {
			errs = append(errs, fmt.Errorf("opportunities[%d].rank must be unique", idx))
		}
		ranks[opportunity.Rank] = struct{}{}
		if _, ok := ids[opportunity.ID]; ok {
			errs = append(errs, fmt.Errorf("opportunities[%d].id must be unique", idx))
		}
		ids[opportunity.ID] = struct{}{}
		if opportunity.Recommendation == nil {
			errs = append(errs, fmt.Errorf("opportunities[%d].recommendation is required", idx))
		} else {
			errs = append(errs, validateRecommendation(opportunity.Recommendation, fmt.Sprintf("opportunities[%d].recommendation", idx))...)
			if len(opportunity.Recommendation.Actions)+len(opportunity.Recommendation.FollowUpSteps) == 0 {
				errs = append(errs, fmt.Errorf("opportunities[%d].recommendation.actions or follow_up_steps is required", idx))
			}
		}
	}
	if rankOne != 1 {
		errs = append(errs, errors.New("exactly one opportunity must have rank 1"))
	}
	return errs
}

func validateReportCoverage(coverage DiagnosticCoverage) []error {
	if !coverage.EligibleForRequiredDetectors {
		if strings.TrimSpace(coverage.IneligibleReason) == "" {
			return []error{errors.New("diagnostic_coverage.ineligible_reason is required when coverage is ineligible")}
		}
		return nil
	}
	if len(coverage.DetectorResults) == 0 {
		return []error{errors.New("diagnostic_coverage.detector_results is required when coverage is eligible")}
	}

	seen := make(map[string]struct{}, len(coverage.DetectorResults))
	ranks := make(map[int]struct{}, len(coverage.DetectorResults))
	var errs []error
	for idx, result := range coverage.DetectorResults {
		if strings.TrimSpace(result.DetectorID) == "" {
			errs = append(errs, fmt.Errorf("diagnostic_coverage.detector_results[%d].detector_id is required", idx))
		}
		if result.Rank < 1 {
			errs = append(errs, fmt.Errorf("diagnostic_coverage.detector_results[%d].rank must be positive", idx))
		} else if _, ok := ranks[result.Rank]; ok {
			errs = append(errs, fmt.Errorf("diagnostic_coverage.detector_results[%d].rank must be unique", idx))
		}
		ranks[result.Rank] = struct{}{}
		switch result.Status {
		case "detected", "ruled_out", "not_evaluable":
		default:
			errs = append(errs, fmt.Errorf("diagnostic_coverage.detector_results[%d].status must be detected, ruled_out, or not_evaluable", idx))
		}
		if _, ok := seen[result.DetectorID]; ok {
			errs = append(errs, fmt.Errorf("diagnostic_coverage.detector_results[%d].detector_id must be unique", idx))
		}
		seen[result.DetectorID] = struct{}{}
	}
	return errs
}

func validateRecommendation(recommendation *Recommendation, field string) []error {
	if recommendation == nil {
		return nil
	}

	var errs []error
	errs = append(errs, validateProjectedEffect(recommendation.ProjectedEffect, field+".projected_effect")...)
	for idx, action := range recommendation.Actions {
		if !action.ValueRequired {
			continue
		}
		if strings.TrimSpace(action.CurrentValue) == "" || strings.TrimSpace(action.ProposedValue) == "" {
			errs = append(errs, fmt.Errorf("%s.actions[%d] must include current_value and proposed_value", field, idx))
		}
	}
	return errs
}

func validateProjectedEffect(effect ProjectedEffect, field string) []error {
	var errs []error
	errs = append(errs, validateProjectedMetric(effect.Latency, field+".latency")...)
	errs = append(errs, validateProjectedMetric(effect.Throughput.Requests, field+".throughput.requests")...)
	errs = append(errs, validateProjectedMetric(effect.Throughput.OutputTokens, field+".throughput.output_tokens")...)
	return errs
}

func validateProjectedMetric(metric ProjectedMetricEffect, field string) []error {
	var errs []error
	if strings.TrimSpace(metric.Metric) == "" {
		errs = append(errs, fmt.Errorf("%s.metric is required", field))
	}
	if strings.TrimSpace(metric.Unit) == "" {
		errs = append(errs, fmt.Errorf("%s.unit is required", field))
	}
	switch metric.Direction {
	case "lower_is_better", "higher_is_better":
	default:
		errs = append(errs, fmt.Errorf("%s.direction must be lower_is_better or higher_is_better", field))
	}
	hasEstimate := metric.Current != nil && metric.Projected != nil && metric.Delta != nil && metric.PercentDelta != nil
	if !hasEstimate && strings.TrimSpace(metric.Reason) == "" {
		errs = append(errs, fmt.Errorf("%s.reason is required when estimates are missing", field))
	}
	return errs
}

func (p SummaryPreview) Validate() error {
	if strings.TrimSpace(p.Headline) != "" {
		return nil
	}
	if strings.TrimSpace(p.PrimaryRecommendation) != "" {
		return nil
	}
	return errors.New("headline or primary_recommendation is required")
}

func validOptimizationTarget(target string) bool {
	switch strings.TrimSpace(target) {
	case "extreme_latency", "latency", "throughput", "extreme_throughput":
		return true
	default:
		return false
	}
}
