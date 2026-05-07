package report

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildVerdictCard(report contracts.FinalReport) reportCardViewModel {
	base := report.Diagnosis.BaseDiagnosis
	lines := []string{
		"Headline: " + fallback(base.Situation.Headline, "-"),
		"Limiter: " + limiterLabel(base.CurrentLimiter),
		"Summary: " + fallback(base.Situation.Summary, "-"),
	}
	return reportCardViewModel{
		id:              "verdict",
		title:           "Verdict",
		summary:         fallback(base.Situation.Headline, limiterLabel(base.CurrentLimiter)),
		defaultExpanded: true,
		sections:        []reportSectionViewModel{{lines: lines}},
	}
}

func buildRecommendationCard(report contracts.FinalReport) reportCardViewModel {
	base := report.Diagnosis.BaseDiagnosis
	card := reportCardViewModel{
		id:              "primary-recommendation",
		title:           "Primary Recommendation",
		defaultExpanded: true,
	}
	if base.Recommendation == nil {
		card.summary = "No safe recommendation"
		card.sections = []reportSectionViewModel{{lines: []string{
			"Decision: No safe recommendation",
			"Reason: " + fallback(base.NoSafeRecommendationReason, "-"),
		}}}
		return card
	}
	rec := *base.Recommendation
	card.summary = fallback(rec.Title, rec.Decision)
	card.sections = append(card.sections, reportSectionViewModel{lines: []string{
		"Title: " + fallback(rec.Title, rec.Decision),
		"Rationale: " + fallback(rec.Rationale, "-"),
		"Expected Gain: " + fallback(rec.ExpectedEffect.Summary, "-"),
	}})
	if section := recommendationActionsSection(rec.Actions); len(section.lines) > 0 {
		card.sections = append(card.sections, section)
	}
	return card
}

func buildFrontierCard(report contracts.FinalReport) reportCardViewModel {
	lines := buildTargetSummaryLines(report.Diagnosis.TargetOverlay)
	return reportCardViewModel{
		id:              "frontier",
		title:           "Frontier And Target Estimate",
		summary:         "Selected target: " + fallback(report.Diagnosis.TargetOverlay.Target, "unknown"),
		defaultExpanded: true,
		sections:        []reportSectionViewModel{{lines: lines}},
	}
}

func buildQuantizationCard(report contracts.FinalReport) (reportCardViewModel, bool) {
	lens := report.DiagnosticLenses.Quantization
	if lens == nil {
		return reportCardViewModel{}, false
	}
	lines := []string{
		"Candidate: " + quantizationCandidateSummary(lens.SelectedCandidate),
		"Expected Gain: " + quantizationOverlaySummary(lens.TargetOverlay),
	}
	if lens.Recommendation != nil {
		lines = append(lines, "Why: "+fallback(lens.Recommendation.Rationale, fallback(lens.Recommendation.Title, lens.Recommendation.Decision)))
	}
	card := reportCardViewModel{
		id:              "quantization",
		title:           "Quantization Opportunity",
		summary:         "Shortlist: " + fallback(firstNonEmpty(lens.SelectedCandidate.Repo, lens.SelectedCandidate.Family), "quantized candidate"),
		defaultExpanded: false,
		sections:        []reportSectionViewModel{{lines: lines}},
	}
	return card, true
}

func buildSecondaryOpportunityCard(report contracts.FinalReport) (reportCardViewModel, bool) {
	opportunity := report.UIHints.SecondaryOpportunity
	if opportunity == nil || opportunity.Recommendation == nil {
		return reportCardViewModel{}, false
	}
	card := reportCardViewModel{
		id:              "secondary-opportunity",
		title:           "Secondary Opportunity",
		summary:         fallback(opportunity.Recommendation.Title, opportunity.Recommendation.Decision),
		defaultExpanded: false,
		sections: []reportSectionViewModel{{lines: []string{
			"Issue: " + fallback(firstNonEmpty(opportunity.IssueID, opportunity.IssueFamily), "-"),
			"Priority: " + fallback(opportunity.PriorityNote, "-"),
			"Title: " + fallback(opportunity.Recommendation.Title, opportunity.Recommendation.Decision),
			"Rationale: " + fallback(opportunity.Recommendation.Rationale, "-"),
		}}},
	}
	if section := recommendationActionsSection(opportunity.Recommendation.Actions); len(section.lines) > 0 {
		card.sections = append(card.sections, section)
	}
	if section := recommendationFollowUpsSection(opportunity.Recommendation.FollowUpSteps); len(section.lines) > 0 {
		card.sections = append(card.sections, section)
	}
	return card, true
}

func buildIssuesCard(report contracts.FinalReport) reportCardViewModel {
	card := reportCardViewModel{
		id:              "issues",
		title:           "Issues",
		defaultExpanded: true,
	}
	if len(report.Issues) == 0 {
		card.summary = "No ranked issues"
		card.sections = []reportSectionViewModel{{lines: []string{"No ranked issues in this report."}}}
		return card
	}
	rows := make([][]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		rows = append(rows, []string{
			fmt.Sprintf("%d", issue.Rank),
			fallback(issue.Label, issue.ID),
			fallback(issue.Confidence, "-"),
			fallback(issue.Summary, fallback(issue.Impact.Summary, "-")),
		})
	}
	card.summary = fallback(report.Issues[0].Label, report.Issues[0].ID)
	card.sections = []reportSectionViewModel{{
		table: &reportTableViewModel{
			headers: []string{"Rank", "Issue", "Confidence", "Summary"},
			rows:    rows,
		},
	}}
	return card
}
