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

	errs = append(errs, validateOverlay(r.Diagnosis.ScenarioOverlays.Latency, "latency")...)
	errs = append(errs, validateOverlay(r.Diagnosis.ScenarioOverlays.Balanced, "balanced")...)
	errs = append(errs, validateOverlay(r.Diagnosis.ScenarioOverlays.Throughput, "throughput")...)
	errs = append(errs, validateIssues(r.Issues)...)
	errs = append(errs, validateReportCoverage(r.DiagnosticCoverage)...)

	return errors.Join(errs...)
}

func validateOverlay(overlay ScenarioOverlay, want string) []error {
	if overlay.Target == "" || overlay.Target == want {
		return nil
	}
	return []error{fmt.Errorf("diagnosis.scenario_overlays.%s.target must be %s", want, want)}
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

func (p SummaryPreview) Validate() error {
	if strings.TrimSpace(p.Headline) != "" {
		return nil
	}
	if strings.TrimSpace(p.PrimaryRecommendation) != "" {
		return nil
	}
	return errors.New("headline or primary_recommendation is required")
}
