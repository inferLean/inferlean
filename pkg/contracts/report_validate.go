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

	errs = append(errs, validateRecommendation(r.Diagnosis.BaseDiagnosis.Recommendation, "diagnosis.base_diagnosis.recommendation")...)

	errs = append(errs, validateOverlay(r.Diagnosis.TargetOverlay, "diagnosis.target_overlay")...)
	errs = append(errs, validateQuantizationLens(r.DiagnosticLenses.Quantization)...)
	errs = append(errs, validateIssues(r.Issues)...)
	errs = append(errs, validateReportCoverage(r.DiagnosticCoverage)...)
	errs = append(errs, validateSecondaryOpportunity(r.UIHints.SecondaryOpportunity)...)

	return errors.Join(errs...)
}

func validateOverlay(overlay ScenarioOverlay, field string) []error {
	var errs []error
	switch overlay.Target {
	case "latency", "throughput":
	case "":
		errs = append(errs, fmt.Errorf("%s.target is required", field))
	default:
		errs = append(errs, fmt.Errorf("%s.target must be latency or throughput", field))
	}
	errs = append(errs, validateRecommendation(overlay.Recommendation, field+".recommendation")...)
	return errs
}

func validateQuantizationLens(lens *QuantizationLens) []error {
	if lens == nil {
		return []error{errors.New("diagnostic_lenses.quantization is required")}
	}
	var errs []error
	if strings.TrimSpace(lens.SelectedCandidate.Family) == "" {
		errs = append(errs, errors.New("diagnostic_lenses.quantization.selected_candidate.family is required"))
	}
	errs = append(errs, validateRecommendation(lens.Recommendation, "diagnostic_lenses.quantization.recommendation")...)
	errs = append(errs, validateQuantizationOverlay(lens.TargetOverlay)...)
	return errs
}

func validateQuantizationOverlay(overlay QuantizationScenarioOverlay) []error {
	switch overlay.Target {
	case "latency", "throughput":
		return nil
	case "":
		return []error{errors.New("diagnostic_lenses.quantization.target_overlay.target is required")}
	default:
		return []error{errors.New("diagnostic_lenses.quantization.target_overlay.target must be latency or throughput")}
	}
}

func validateIssues(issues []Issue) []error {
	if len(issues) == 0 {
		return nil
	}

	rankOne := 0
	ids := make(map[string]struct{}, len(issues))
	var errs []error
	for idx, issue := range issues {
		if strings.TrimSpace(issue.ID) == "" {
			errs = append(errs, fmt.Errorf("issues[%d].id is required", idx))
		}
		if issue.Rank == 1 {
			rankOne++
		}
		if _, ok := ids[issue.ID]; ok {
			errs = append(errs, fmt.Errorf("issues[%d].id must be unique", idx))
		}
		ids[issue.ID] = struct{}{}
	}
	if rankOne != 1 {
		errs = append(errs, errors.New("exactly one issue must have rank 1"))
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
	var errs []error
	for idx, result := range coverage.DetectorResults {
		if strings.TrimSpace(result.DetectorID) == "" {
			errs = append(errs, fmt.Errorf("diagnostic_coverage.detector_results[%d].detector_id is required", idx))
		}
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
	for idx, action := range recommendation.Actions {
		if strings.TrimSpace(action.CurrentValue) == "" || strings.TrimSpace(action.ProposedValue) == "" {
			errs = append(errs, fmt.Errorf("%s.actions[%d] must include current_value and proposed_value", field, idx))
		}
	}
	return errs
}

func validateSecondaryOpportunity(opportunity *SecondaryOpportunity) []error {
	if opportunity == nil {
		return nil
	}

	var errs []error
	if strings.TrimSpace(opportunity.IssueID) == "" {
		errs = append(errs, errors.New("ui_hints.secondary_opportunity.issue_id is required"))
	}
	if strings.TrimSpace(opportunity.IssueFamily) == "" {
		errs = append(errs, errors.New("ui_hints.secondary_opportunity.issue_family is required"))
	}
	if strings.TrimSpace(opportunity.PriorityNote) == "" {
		errs = append(errs, errors.New("ui_hints.secondary_opportunity.priority_note is required"))
	}
	if opportunity.Recommendation == nil {
		errs = append(errs, errors.New("ui_hints.secondary_opportunity.recommendation is required"))
	}
	errs = append(errs, validateRecommendation(opportunity.Recommendation, "ui_hints.secondary_opportunity.recommendation")...)
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
