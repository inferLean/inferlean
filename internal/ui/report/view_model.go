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
	vm := reportViewModel{
		headerChips:       buildHeaderChips(report),
		footerChips:       buildFooterChips(report, now),
		cards:             buildReportCards(report),
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

func buildFooterChips(report contracts.FinalReport, now time.Time) []string {
	chips := []string{
		"collector " + fallback(report.Job.CollectorVersion, "-"),
		"artifact " + fallback(report.Job.ArtifactSchemaVersion, "-"),
		"rendered " + formatTime(now),
	}
	if installationID := strings.TrimSpace(report.Job.InstallationID); installationID != "" {
		chips = append(chips, "installation "+installationID)
	}
	return chips
}

func buildReportCards(report contracts.FinalReport) []reportCardViewModel {
	cards := []reportCardViewModel{
		buildVerdictCard(report),
		buildRecommendationCard(report),
		buildFrontierCard(report),
	}
	if card, ok := buildQuantizationCard(report); ok {
		cards = append(cards, card)
	}
	if card, ok := buildSecondaryOpportunityCard(report); ok {
		cards = append(cards, card)
	}
	cards = append(cards, buildIssuesCard(report))
	cards = append(cards, buildEvidenceCard(report))
	cards = append(cards, buildCollectionQualityCard(report))
	return cards
}
