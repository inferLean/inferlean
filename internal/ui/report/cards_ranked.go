package report

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildRankedOpportunitiesCard(opportunities []contracts.Opportunity, artifact *contracts.RunArtifact) (reportCardViewModel, bool) {
	if len(opportunities) == 0 {
		return reportCardViewModel{}, false
	}
	card := reportCardViewModel{
		id:              "ranked-opportunities",
		title:           "Ranked Opportunities",
		summary:         fmt.Sprintf("%d backend-ranked opportunities", len(opportunities)),
		defaultExpanded: true,
		sections: []reportSectionViewModel{{
			table: &reportTableViewModel{
				headers: []string{"Rank", "Opportunity", "Category", "Confidence", "Recommendation"},
				rows:    opportunityRows(opportunities),
			},
		}},
	}
	for _, opportunity := range opportunities {
		card.sections = append(card.sections, opportunityContextSections(opportunity, artifact)...)
		if opportunity.Recommendation != nil {
			card.sections = append(card.sections, recommendationDetailSections(*opportunity.Recommendation)...)
		}
	}
	return card, true
}

func buildRankedIssuesCard(issues []contracts.Issue, artifact *contracts.RunArtifact) (reportCardViewModel, bool) {
	if len(issues) == 0 {
		return reportCardViewModel{}, false
	}
	card := reportCardViewModel{
		id:              "ranked-issues",
		title:           "Ranked Issues",
		summary:         fmt.Sprintf("%d backend-ranked issues", len(issues)),
		defaultExpanded: true,
		sections: []reportSectionViewModel{{
			table: &reportTableViewModel{
				headers: []string{"Rank", "Detector", "Issue", "Confidence", "Recommendation"},
				rows:    issueRows(issues),
			},
		}},
	}
	for _, issue := range issues {
		card.sections = append(card.sections, issueContextSections(issue, artifact)...)
		if issue.Recommendation != nil {
			card.sections = append(card.sections, recommendationDetailSections(*issue.Recommendation)...)
		}
	}
	return card, true
}

func issueRows(issues []contracts.Issue) [][]string {
	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, []string{
			formatRank(issue.Rank),
			fallback(issue.DetectorID, issue.ID),
			fallback(issue.Label, issue.ID),
			fallback(issue.Confidence, "-"),
			issueRecommendationLabel(issue),
		})
	}
	return rows
}

func opportunityRows(opportunities []contracts.Opportunity) [][]string {
	rows := make([][]string, 0, len(opportunities))
	for _, opportunity := range opportunities {
		rows = append(rows, []string{
			formatRank(opportunity.Rank),
			fallback(opportunity.Title, opportunity.ID),
			fallback(opportunity.Category, "-"),
			fallback(opportunity.Confidence, "-"),
			opportunityRecommendationLabel(opportunity),
		})
	}
	return rows
}

func issueContextSections(issue contracts.Issue, artifact *contracts.RunArtifact) []reportSectionViewModel {
	lines := []string{
		"Rank: " + formatRank(issue.Rank),
		"Detector: " + fallback(issue.DetectorID, issue.ID),
		"Family: " + fallback(issue.Family, "-"),
		"Label: " + fallback(issue.Label, issue.ID),
		"Confidence: " + fallback(issue.Confidence, "-"),
	}
	lines = append(lines, evidenceReferenceLines(issue.EvidenceRefs, artifact)...)
	return []reportSectionViewModel{{
		title: fmt.Sprintf("Issue %s Detail", formatRank(issue.Rank)),
		lines: lines,
	}}
}

func opportunityContextSections(opportunity contracts.Opportunity, artifact *contracts.RunArtifact) []reportSectionViewModel {
	lines := []string{
		"Rank: " + formatRank(opportunity.Rank),
		"Detector: " + fallback(opportunity.DetectorID, opportunity.ID),
		"Category: " + fallback(opportunity.Category, "-"),
		"Title: " + fallback(opportunity.Title, opportunity.ID),
		"Confidence: " + fallback(opportunity.Confidence, "-"),
	}
	lines = append(lines, evidenceReferenceLines(opportunity.EvidenceRefs, artifact)...)
	return []reportSectionViewModel{{
		title: fmt.Sprintf("Opportunity %s Detail", formatRank(opportunity.Rank)),
		lines: lines,
	}}
}

func formatRank(rank int) string {
	if rank <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", rank)
}
