package report

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestBuildReportViewModelIncludesDashboardParityCards(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	artifact := fullArtifactFixture()
	vm := buildReportViewModelWithArtifact(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, &artifact, time.Unix(1700000200, 0).UTC(), "")

	var ids []string
	for _, card := range vm.cards {
		ids = append(ids, card.id)
	}
	want := []string{
		"top-issue-1",
		"top-issue-2",
		"saturation",
		"top-opportunity-1",
		"top-opportunity-2",
		"ranked-opportunities",
		"ranked-issues",
		"evidence",
		"collection-quality",
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("card order = %v, want %v", ids, want)
	}
	if vm.browserURL == "" {
		t.Fatal("expected browser URL when identity is complete")
	}
	if got := vm.cards[6].sections[0].table; got == nil || len(got.rows) != len(report.Issues) {
		t.Fatal("ranked issues card should include all table rows")
	}
	if got := vm.cards[7].tabs; len(got) != 3 {
		t.Fatalf("evidence tabs = %d, want 3", len(got))
	}
}

func TestBuildReportViewModelOmitsOptionalCardsWhenDataMissing(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	report.Opportunities = nil

	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	for _, card := range vm.cards {
		if card.id == "ranked-opportunities" || strings.HasPrefix(card.id, "top-opportunity-") {
			t.Fatalf("unexpected optional card present: %s", card.id)
		}
	}
	if vm.browserURL != "" {
		t.Fatalf("browser URL should be absent when installation identity is incomplete, got %q", vm.browserURL)
	}
}

func TestBuildReportViewModelCarriesValidationWarning(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(
		report,
		reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID},
		defaults.AppBaseURL,
		time.Unix(1700000200, 0).UTC(),
		"Schema validation warning: invalid detector status",
	)
	if vm.validationWarning == "" {
		t.Fatal("expected validation warning to be preserved in the view model")
	}
}

func TestBuildReportViewModelKeepsTopIssueRecommendationFocused(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	lines := vm.cards[0].sections[1].lines
	want := []string{
		"Title: Reduce KV footprint",
		"Decision: reduce_kv_footprint",
		"Rationale: This unlocks safer scheduler headroom before any throughput tuning.",
		"Confidence: high",
		"Projected Effect: Likely improvement: +8% to +15% throughput.",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("top issue recommendation lines = %v, want %v", lines, want)
	}
}

func TestBuildReportViewModelKeepsRecommendationDetailsComplete(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	if len(vm.cards[0].sections) != 5 {
		t.Fatalf("top recommendation sections = %d, want 5", len(vm.cards[0].sections))
	}
	projectedLines := vm.cards[0].sections[3].lines
	if !slices.Contains(projectedLines, "Request Throughput: request_throughput 4.80 req/s -> 5.28 req/s (+10.0%)") {
		t.Fatalf("projected effect lines = %v, want request throughput projection", projectedLines)
	}
	actionLines := vm.cards[0].sections[2].lines
	if slices.Contains(actionLines, "   How: Keep prompt mix and concurrency stable.") {
		t.Fatalf("action lines should not include how text: %v", actionLines)
	}
	if !slices.Contains(actionLines, "   Risk: Shorter maximum context for some requests.") {
		t.Fatalf("action lines should include risk text: %v", actionLines)
	}
}

func TestBuildReportViewModelIncludesVLLMCommandReplacement(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	report.VLLMCommandReplacement = &contracts.VLLMCommandReplacement{
		CurrentCommand:     "vllm serve Qwen/Qwen3 --max-num-seqs=64",
		RecommendedCommand: "vllm serve Qwen/Qwen3 --max-num-seqs=128",
		AppliedActionIDs:   []string{"action:set-max-num-seqs"},
		SkippedActionIDs:   []string{"action:right-size-gpu-capacity"},
		Warnings:           []string{"Skipped non-command recommendation actions."},
	}

	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	card := vm.cards[2]
	if got, want := card.id, "vllm-command-replacement"; got != want {
		t.Fatalf("card id = %q, want %q", got, want)
	}
	if !sectionContains(card.sections, "vllm serve Qwen/Qwen3 --max-num-seqs=128") {
		t.Fatalf("command card missing recommended command: %#v", card.sections)
	}
	if !sectionContains(card.sections, "action:right-size-gpu-capacity") {
		t.Fatalf("command card missing skipped action ids: %#v", card.sections)
	}
}

func TestBuildReportViewModelUsesRecommendationProjectedEffect(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	lines := vm.cards[0].sections[3].lines
	want := []string{
		"Latency: latency_e2e_seconds 1.40 s -> 1.40 s (+0.0%)",
		"Request Throughput: request_throughput 4.80 req/s -> 5.28 req/s (+10.0%)",
		"Output Token Throughput: generation_tokens_per_second 256.00 tok/s -> 281.60 tok/s (+10.0%)",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("projected effect lines = %v, want %v", lines, want)
	}
}

func TestBuildReportViewModelKeepsAllRankedRecommendationDetails(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	rankedOpportunities := vm.cards[5]
	if !sectionContains(rankedOpportunities.sections, "Validate prefix caching") {
		t.Fatalf("ranked opportunities missing secondary recommendation detail: %#v", rankedOpportunities.sections)
	}
	rankedIssues := vm.cards[6]
	if !sectionContains(rankedIssues.sections, "Increase batching posture") {
		t.Fatalf("ranked issues missing secondary recommendation detail: %#v", rankedIssues.sections)
	}
	if !sectionContains(rankedIssues.sections, "runtime_config.max_model_len") {
		t.Fatalf("ranked issues missing evidence references: %#v", rankedIssues.sections)
	}
}

func TestBuildReportViewModelKeepsQuantizationAsOpportunity(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	card := vm.cards[3]
	if got, want := card.id, "top-opportunity-1"; got != want {
		t.Fatalf("card id = %q, want %q", got, want)
	}
	if len(card.sections) < 3 {
		t.Fatalf("opportunity sections = %d, want recommendation detail", len(card.sections))
	}
	want := []string{
		"Title: Evaluate quantization next",
		"Rationale: Treat quantization as a ranked opportunity.",
	}
	got := []string{card.sections[1].lines[0], card.sections[1].lines[2]}
	if !slices.Equal(got, want) {
		t.Fatalf("opportunity recommendation lines = %v, want %v", got, want)
	}
}

func TestBuildReportViewModelSaturationShowsMissingEvidence(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	card := vm.cards[2]
	if got, want := card.id, "saturation"; got != want {
		t.Fatalf("card id = %q, want %q", got, want)
	}
	rows := card.sections[1].table.rows
	if !slices.Contains(rows[1], "metrics.gpu.sm_active") {
		t.Fatalf("saturation rows should show missing evidence: %v", rows)
	}
}

func TestBuildReportViewModelUsesArtifactEvidenceWithoutTelemetryTabs(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	artifact := fullArtifactFixture()
	vm := buildReportViewModelWithArtifact(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, &artifact, time.Unix(1700000200, 0).UTC(), "")

	evidence := vm.cards[7]
	var tabTitles []string
	for _, tab := range evidence.tabs {
		tabTitles = append(tabTitles, tab.title)
	}
	if !slices.Equal(tabTitles, []string{"config", "environment", "process"}) {
		t.Fatalf("evidence tabs = %v, want config/environment/process only", tabTitles)
	}
	configRows := evidence.tabs[0].sections[0].table.rows
	if !rowWith(configRows, "max_model_len", "8192", "cited") {
		t.Fatalf("runtime config rows should include cited max_model_len: %v", configRows)
	}
	processLines := strings.Join(evidence.tabs[2].sections[0].lines, "\n")
	if !strings.Contains(processLines, "python -m vllm.entrypoints.openai.api_server") {
		t.Fatalf("process tab missing command line: %s", processLines)
	}
}

func TestBuildReportViewModelFallsBackWhenArtifactMissing(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	evidence := vm.cards[7]
	if got := evidence.tabs[0].sections[0].lines[0]; !strings.Contains(got, "Runtime config evidence is unavailable") {
		t.Fatalf("missing report-only fallback message: %q", got)
	}
}

func rowWith(rows [][]string, values ...string) bool {
	for _, row := range rows {
		ok := true
		for i, value := range values {
			if i >= len(row) || row[i] != value {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func sectionContains(sections []reportSectionViewModel, needle string) bool {
	for _, section := range sections {
		for _, line := range section.lines {
			if strings.Contains(line, needle) {
				return true
			}
		}
	}
	return false
}
