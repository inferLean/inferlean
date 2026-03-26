package contracts

import (
	"errors"
	"fmt"
	"strings"
)

func (a RunArtifact) Validate() error {
	var errs []error

	if strings.TrimSpace(a.SchemaVersion) == "" {
		errs = append(errs, errors.New("schema_version is required"))
	} else if a.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version must be %s", SchemaVersion))
	}

	if err := a.Job.validate(); err != nil {
		errs = append(errs, err)
	}

	if a.Job.SchemaVersion != "" && a.Job.SchemaVersion != a.SchemaVersion {
		errs = append(errs, errors.New("job.schema_version must match schema_version"))
	}

	if err := a.ProcessInspection.validate(); err != nil {
		errs = append(errs, err)
	}

	if err := a.CollectionQuality.validate(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (j Job) validate() error {
	var errs []error
	if strings.TrimSpace(j.RunID) == "" {
		errs = append(errs, errors.New("job.run_id is required"))
	}
	if strings.TrimSpace(j.InstallationID) == "" {
		errs = append(errs, errors.New("job.installation_id is required"))
	}
	if strings.TrimSpace(j.CollectorVersion) == "" {
		errs = append(errs, errors.New("job.collector_version is required"))
	}
	if strings.TrimSpace(j.SchemaVersion) == "" {
		errs = append(errs, errors.New("job.schema_version is required"))
	}
	if j.CollectedAt.IsZero() {
		errs = append(errs, errors.New("job.collected_at is required"))
	}

	return errors.Join(errs...)
}

func (p ProcessInspection) validate() error {
	if strings.TrimSpace(p.TargetProcess.RawCommandLine) == "" {
		return errors.New("process_inspection.target_process.raw_command_line is required")
	}
	return nil
}

func (q CollectionQuality) validate() error {
	var errs []error
	for name, source := range q.SourceStates {
		if strings.TrimSpace(source.Status) == "" {
			errs = append(errs, fmt.Errorf("collection_quality.source_states[%s].status is required", name))
			continue
		}
		switch source.Status {
		case "ok", "degraded", "missing":
		default:
			errs = append(errs, fmt.Errorf("collection_quality.source_states[%s].status must be ok, degraded, or missing", name))
		}
	}
	if q.Completeness < 0 || q.Completeness > 1 {
		errs = append(errs, errors.New("collection_quality.completeness must be between 0 and 1"))
	}

	return errors.Join(errs...)
}
