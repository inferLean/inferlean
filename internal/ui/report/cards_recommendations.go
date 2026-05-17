package report

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildTopIssueRecommendationCards(issues []contracts.Issue) []reportCardViewModel {
	cards := make([]reportCardViewModel, 0, 2)
	for _, issue := range issues {
		if issue.Recommendation == nil {
			continue
		}
		position := len(cards)
		title := "Top Issue Recommendation"
		if position == 1 {
			title = "Secondary Issue Recommendation"
		}
		cards = append(cards, buildIssueRecommendationCard(issue, title, fmt.Sprintf("top-issue-%d", position+1), true))
		if len(cards) == 2 {
			return cards
		}
	}
	if len(cards) > 0 || len(issues) == 0 {
		return cards
	}
	return []reportCardViewModel{{
		id:              "top-issue-empty",
		title:           "Top Issue Recommendation",
		summary:         "No safe recommendation",
		defaultExpanded: true,
		sections: []reportSectionViewModel{{lines: []string{
			"No ranked issue recommendation is available in this report.",
		}}},
	}}
}

func buildTopOpportunityRecommendationCards(opportunities []contracts.Opportunity) []reportCardViewModel {
	cards := make([]reportCardViewModel, 0, 2)
	for _, opportunity := range opportunities {
		if opportunity.Recommendation == nil {
			continue
		}
		position := len(cards)
		title := "Top Opportunity Recommendation"
		if position == 1 {
			title = "Secondary Opportunity Recommendation"
		}
		cards = append(cards, buildOpportunityRecommendationCard(opportunity, title, fmt.Sprintf("top-opportunity-%d", position+1), false))
		if len(cards) == 2 {
			return cards
		}
	}
	return cards
}

func buildIssueRecommendationCard(issue contracts.Issue, title, id string, expanded bool) reportCardViewModel {
	card := reportCardViewModel{
		id:              id,
		title:           title,
		summary:         fallback(issueRecommendationLabel(issue), fallback(issue.Label, issue.ID)),
		defaultExpanded: expanded,
		sections:        issueContextSections(issue, nil),
	}
	if issue.Recommendation != nil {
		card.sections = append(card.sections, recommendationDetailSections(*issue.Recommendation)...)
	}
	return card
}

func buildOpportunityRecommendationCard(opportunity contracts.Opportunity, title, id string, expanded bool) reportCardViewModel {
	card := reportCardViewModel{
		id:              id,
		title:           title,
		summary:         fallback(opportunityRecommendationLabel(opportunity), fallback(opportunity.Title, opportunity.ID)),
		defaultExpanded: expanded,
		sections:        opportunityContextSections(opportunity, nil),
	}
	if opportunity.Recommendation != nil {
		card.sections = append(card.sections, recommendationDetailSections(*opportunity.Recommendation)...)
	}
	return card
}

func recommendationDetailSections(rec contracts.Recommendation) []reportSectionViewModel {
	sections := []reportSectionViewModel{{
		title: "Recommendation",
		lines: []string{
			"Title: " + fallback(rec.Title, rec.Decision),
			"Decision: " + fallback(rec.Decision, "-"),
			"Rationale: " + fallback(rec.Rationale, "-"),
			"Confidence: " + fallback(rec.Confidence, "-"),
			"Projected Effect: " + fallback(rec.ProjectedEffect.Summary, "-"),
		},
	}}
	if section := recommendationActionsSection(rec.Actions); len(section.lines) > 0 {
		sections = append(sections, section)
	}
	sections = append(sections, reportSectionViewModel{
		title: "Projected Effect",
		lines: projectedEffectLines(rec.ProjectedEffect),
	})
	if section := recommendationFollowUpsSection(rec.FollowUpSteps); len(section.lines) > 0 {
		sections = append(sections, section)
	}
	return sections
}

func primaryIssue(report contracts.FinalReport) *contracts.Issue {
	issues := displayIssues(report)
	if len(issues) == 0 {
		return nil
	}
	return &issues[0]
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

func issueRecommendationLabel(issue contracts.Issue) string {
	if issue.Recommendation == nil {
		return "-"
	}
	return fallback(issue.Recommendation.Title, issue.Recommendation.Decision)
}

func opportunityRecommendationLabel(opportunity contracts.Opportunity) string {
	if opportunity.Recommendation == nil {
		return "-"
	}
	return fallback(opportunity.Recommendation.Title, opportunity.Recommendation.Decision)
}
