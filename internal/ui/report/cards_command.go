package report

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildVLLMCommandReplacementCard(replacement *contracts.VLLMCommandReplacement) (reportCardViewModel, bool) {
	if replacement == nil {
		return reportCardViewModel{}, false
	}
	current := strings.TrimSpace(replacement.CurrentCommand)
	recommended := strings.TrimSpace(replacement.RecommendedCommand)
	if current == "" && recommended == "" && len(replacement.Warnings) == 0 {
		return reportCardViewModel{}, false
	}
	card := reportCardViewModel{
		id:              "vllm-command-replacement",
		title:           "vLLM Command Replacement",
		summary:         commandReplacementSummary(*replacement),
		defaultExpanded: recommended != "",
	}
	card.sections = append(card.sections, commandReplacementSections(*replacement)...)
	return card, true
}

func commandReplacementSections(replacement contracts.VLLMCommandReplacement) []reportSectionViewModel {
	sections := make([]reportSectionViewModel, 0, 4)
	if current := strings.TrimSpace(replacement.CurrentCommand); current != "" {
		sections = append(sections, reportSectionViewModel{
			title: "Current Command",
			lines: []string{current},
		})
	}
	if recommended := strings.TrimSpace(replacement.RecommendedCommand); recommended != "" {
		sections = append(sections, reportSectionViewModel{
			title: "Recommended Command",
			lines: []string{recommended},
		})
	}
	if len(replacement.AppliedActionIDs)+len(replacement.SkippedActionIDs) > 0 {
		sections = append(sections, reportSectionViewModel{
			title: "Action Coverage",
			lines: commandReplacementActionLines(replacement),
		})
	}
	if len(replacement.Warnings) > 0 {
		sections = append(sections, reportSectionViewModel{
			title: "Warnings",
			lines: bulletLines(replacement.Warnings),
		})
	}
	return sections
}

func commandReplacementSummary(replacement contracts.VLLMCommandReplacement) string {
	if strings.TrimSpace(replacement.RecommendedCommand) == "" {
		return "No safe one-line replacement command was generated."
	}
	applied := len(replacement.AppliedActionIDs)
	if applied == 1 {
		return "Recommended command applies 1 command-safe action."
	}
	return fmt.Sprintf("Recommended command applies %d command-safe actions.", applied)
}

func commandReplacementActionLines(replacement contracts.VLLMCommandReplacement) []string {
	lines := make([]string, 0, 2)
	if len(replacement.AppliedActionIDs) > 0 {
		lines = append(lines, "Applied: "+strings.Join(replacement.AppliedActionIDs, ", "))
	}
	if len(replacement.SkippedActionIDs) > 0 {
		lines = append(lines, "Skipped: "+strings.Join(replacement.SkippedActionIDs, ", "))
	}
	return lines
}

func bulletLines(values []string) []string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			lines = append(lines, "- "+trimmed)
		}
	}
	return lines
}
