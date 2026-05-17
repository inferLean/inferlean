package report

import (
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type reportViewModel struct {
	headerChips       []string
	footerChips       []string
	cards             []reportCardViewModel
	browserURL        string
	validationWarning string
}

type reportCardViewModel struct {
	id              string
	title           string
	summary         string
	defaultExpanded bool
	sections        []reportSectionViewModel
	tabs            []reportTabViewModel
}

type reportSectionViewModel struct {
	title string
	lines []string
	table *reportTableViewModel
}

type reportTabViewModel struct {
	title    string
	summary  string
	sections []reportSectionViewModel
}

type reportTableViewModel struct {
	headers []string
	rows    [][]string
}

func buildReportViewModel(report contracts.FinalReport, identity reportIdentity, backendURL string, now time.Time, validationWarning string) reportViewModel {
	return buildReportViewModelWithArtifact(report, identity, backendURL, nil, now, validationWarning)
}

func buildReportViewModelWithArtifact(report contracts.FinalReport, identity reportIdentity, backendURL string, artifact *contracts.RunArtifact, now time.Time, validationWarning string) reportViewModel {
	vm := reportViewModel{
		headerChips:       buildHeaderChips(report),
		footerChips:       buildFooterChips(report, artifact, now),
		cards:             buildReportCards(report, artifact),
		validationWarning: strings.TrimSpace(validationWarning),
	}
	if url, ok := ReportURL(backendURL, identity.installationID, identity.runID); ok {
		vm.browserURL = url
	}
	return vm
}

func buildHeaderChips(report contracts.FinalReport) []string {
	chips := []string{
		"run " + fallback(report.Job.RunID, "-"),
		"model " + fallback(firstNonEmpty(report.Environment.ServedModelName, report.Environment.Model), "-"),
		"gpu " + environmentGPU(report.Environment),
		"collected " + formatTime(report.Job.CollectedAt),
		"reported " + formatTime(report.Job.ReportedAt),
		"schema " + fallback(report.SchemaVersion, "unknown"),
	}
	if tier := strings.TrimSpace(report.Entitlement.Tier); tier != "" {
		chips = append(chips, "tier "+tier)
	}
	return chips
}

func buildFooterChips(report contracts.FinalReport, artifact *contracts.RunArtifact, now time.Time) []string {
	artifactSchema := report.Job.ArtifactSchemaVersion
	if artifact != nil && strings.TrimSpace(artifact.SchemaVersion) != "" {
		artifactSchema = artifact.SchemaVersion
	}
	chips := []string{
		"report schema " + fallback(report.SchemaVersion, "unknown"),
		"artifact schema " + fallback(artifactSchema, "-"),
		"rendered " + formatTime(now),
	}
	installationID := strings.TrimSpace(report.Job.InstallationID)
	if artifact != nil && installationID == "" {
		installationID = strings.TrimSpace(artifact.Job.InstallationID)
	}
	if installationID != "" {
		chips = append(chips, "installation "+installationID)
	}
	return chips
}

func buildReportCards(report contracts.FinalReport, artifact *contracts.RunArtifact) []reportCardViewModel {
	issues := displayIssues(report)
	opportunities := displayOpportunities(report)

	cards := buildTopIssueRecommendationCards(issues)
	if card, ok := buildVLLMCommandReplacementCard(report.VLLMCommandReplacement); ok {
		cards = append(cards, card)
	}
	if card, ok := buildSaturationCard(report); ok {
		cards = append(cards, card)
	}
	cards = append(cards, buildTopOpportunityRecommendationCards(opportunities)...)
	if card, ok := buildRankedOpportunitiesCard(opportunities, artifact); ok {
		cards = append(cards, card)
	}
	if card, ok := buildRankedIssuesCard(issues, artifact); ok {
		cards = append(cards, card)
	}
	cards = append(cards, buildEvidenceCard(report, artifact))
	cards = append(cards, buildCollectionQualityCard(report, artifact))
	return cards
}
