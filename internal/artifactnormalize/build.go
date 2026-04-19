package artifactnormalize

import (
	"fmt"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func Build(input Input) (contracts.RunArtifact, error) {
	artifact := normalizeInput(input)
	if err := artifact.Validate(); err != nil {
		return contracts.RunArtifact{}, fmt.Errorf("validate run artifact: %w", err)
	}
	return artifact, nil
}

func normalizeInput(input Input) contracts.RunArtifact {
	collectedAt := input.Job.FinishedAt
	if collectedAt.IsZero() {
		collectedAt = input.Job.StartedAt
	}
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}
	sourceStates := toSourceStates(input.CollectionQuality.SourceStatus)
	missingEvidence := make([]string, 0)
	degradedEvidence := make([]string, 0)
	appendMissing(sourceStates, &missingEvidence, "missing")
	appendMissing(sourceStates, &degradedEvidence, "degraded")
	return contracts.RunArtifact{
		SchemaVersion: contracts.SchemaVersion,
		Job: contracts.Job{
			RunID:            strings.TrimSpace(input.Job.RunID),
			InstallationID:   strings.TrimSpace(input.Job.InstallationID),
			CollectorVersion: firstNonEmpty(input.Job.CollectorVersion, "dev"),
			SchemaVersion:    contracts.SchemaVersion,
			CollectedAt:      collectedAt,
		},
		Environment:          normalizeEnvironment(input),
		RuntimeConfig:        normalizeRuntimeConfig(input),
		Metrics:              normalizeMetrics(input),
		ProcessInspection:    normalizeProcessInspection(input),
		WorkloadObservations: normalizeWorkload(input),
		CollectionQuality: contracts.CollectionQuality{
			SourceStates:     sourceStates,
			MissingEvidence:  missingEvidence,
			DegradedEvidence: degradedEvidence,
			Completeness:     completeness(sourceStates),
			Summary:          firstNonEmpty(input.CollectionQuality.TelemetryMode, "normalized-from-cli"),
		},
	}
}
