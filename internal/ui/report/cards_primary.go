package report

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildVerdictCard(report contracts.FinalReport) reportCardViewModel {
	lines := []string{
		"Headline: " + diagnosisHeadline(report),
		"Top Issue: " + issueLabel(primaryIssue(report)),
		"Summary: " + diagnosisSummary(report),
	}
	return reportCardViewModel{
		id:              "verdict",
		title:           "Verdict",
		summary:         diagnosisHeadline(report),
		defaultExpanded: true,
		sections:        []reportSectionViewModel{{lines: lines}},
	}
}

func buildRecommendationCard(report contracts.FinalReport) reportCardViewModel {
	base := report.Diagnosis.BaseDiagnosis
	recommendation := primaryRecommendation(report)
	card := reportCardViewModel{
		id:              "primary-recommendation",
		title:           "Primary Recommendation",
		defaultExpanded: true,
	}
	if recommendation == nil {
		card.summary = "No safe recommendation"
		card.sections = []reportSectionViewModel{{lines: []string{
			"Decision: No safe recommendation",
			"Reason: " + fallback(base.NoSafeRecommendationReason, "-"),
		}}}
		return card
	}
	rec := *recommendation
	card.summary = fallback(rec.Title, rec.Decision)
	card.sections = append(card.sections, reportSectionViewModel{lines: []string{
		"Title: " + fallback(rec.Title, rec.Decision),
		"Rationale: " + fallback(rec.Rationale, "-"),
		"Projected Effect: " + fallback(rec.ProjectedEffect.Summary, "-"),
	}})
	card.sections = append(card.sections, reportSectionViewModel{title: "Projected Effect", lines: projectedEffectLines(rec.ProjectedEffect)})
	if section := recommendationActionsSection(rec.Actions); len(section.lines) > 0 {
		card.sections = append(card.sections, section)
	}
	return card
}

func primaryIssue(report contracts.FinalReport) *contracts.Issue {
	if len(report.Issues) == 0 {
		return nil
	}
	return &report.Issues[0]
}

func primaryRecommendation(report contracts.FinalReport) *contracts.Recommendation {
	issue := primaryIssue(report)
	if issue == nil {
		return nil
	}
	return issue.Recommendation
}

func issueLabel(issue *contracts.Issue) string {
	if issue == nil {
		return "-"
	}
	return fallback(firstNonEmpty(issue.Label, issue.DetectorID, issue.ID), "-")
}

func buildOpportunitiesCard(report contracts.FinalReport) (reportCardViewModel, bool) {
	if len(report.Opportunities) == 0 {
		return reportCardViewModel{}, false
	}
	opportunity := report.Opportunities[0]
	summary := fallback(opportunity.Title, opportunity.ID)
	if opportunity.Recommendation != nil {
		summary = fallback(opportunity.Recommendation.Title, opportunity.Recommendation.Decision)
	}
	card := reportCardViewModel{
		id:              "opportunities",
		title:           "Opportunities",
		summary:         summary,
		defaultExpanded: false,
	}
	rows := make([][]string, 0, len(report.Opportunities))
	for _, opportunity := range report.Opportunities {
		rows = append(rows, []string{
			fmt.Sprintf("%d", opportunity.Rank),
			fallback(opportunity.Title, opportunity.ID),
			fallback(opportunity.Category, "-"),
			fallback(opportunity.Confidence, "-"),
		})
	}
	card.sections = append(card.sections, reportSectionViewModel{table: &reportTableViewModel{
		headers: []string{"Rank", "Opportunity", "Category", "Confidence"},
		rows:    rows,
	}})
	if opportunity.Recommendation != nil {
		card.sections = append(card.sections, reportSectionViewModel{lines: []string{
			"Title: " + fallback(opportunity.Recommendation.Title, opportunity.Recommendation.Decision),
			"Rationale: " + fallback(opportunity.Recommendation.Rationale, "-"),
		}})
		card.sections = append(card.sections, reportSectionViewModel{title: "Projected Effect", lines: projectedEffectLines(opportunity.Recommendation.ProjectedEffect)})
		if section := recommendationActionsSection(opportunity.Recommendation.Actions); len(section.lines) > 0 {
			card.sections = append(card.sections, section)
		}
		if section := recommendationFollowUpsSection(opportunity.Recommendation.FollowUpSteps); len(section.lines) > 0 {
			card.sections = append(card.sections, section)
		}
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
			issueRecommendationLabel(issue),
		})
	}
	card.summary = fallback(report.Issues[0].Label, report.Issues[0].ID)
	card.sections = []reportSectionViewModel{{
		table: &reportTableViewModel{
			headers: []string{"Rank", "Issue", "Confidence", "Recommendation"},
			rows:    rows,
		},
	}}
	card.sections = append(card.sections, issueRecommendationSections(report.Issues)...)
	return card
}

func issueRecommendationSections(issues []contracts.Issue) []reportSectionViewModel {
	sections := make([]reportSectionViewModel, 0, 2)
	for _, issue := range issues {
		if issue.Recommendation == nil {
			continue
		}
		lines := []string{
			fmt.Sprintf("Issue %d: %s", issue.Rank, fallback(issue.Label, issue.ID)),
			"Recommendation: " + fallback(issue.Recommendation.Title, issue.Recommendation.Decision),
		}
		if rationale := fallback(issue.Recommendation.Rationale, ""); rationale != "" {
			lines = append(lines, "Rationale: "+rationale)
		}
		sections = append(sections, reportSectionViewModel{title: "Issue Recommendation", lines: lines})
		if len(sections) == 2 {
			return sections
		}
	}
	return sections
}

func issueRecommendationLabel(issue contracts.Issue) string {
	if issue.Recommendation == nil {
		return "-"
	}
	return fallback(issue.Recommendation.Title, issue.Recommendation.Decision)
}
