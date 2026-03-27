package collector

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/debug"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func NewService() Service {
	return Service{client: newHTTPClient(httpTimeout)}
}

func ValidateDurations(collectFor, scrapeEvery time.Duration) error {
	switch {
	case collectFor <= 0:
		return errors.New("collect duration must be greater than zero")
	case scrapeEvery <= 0:
		return errors.New("scrape interval must be greater than zero")
	case scrapeEvery > collectFor:
		return errors.New("scrape interval must not exceed collect duration")
	default:
		return nil
	}
}

func (s Service) Collect(ctx context.Context, opts Options) (Result, error) {
	if runtime.GOOS != "linux" {
		return Result{}, errors.New("collection is only supported on Linux in Phase 2")
	}
	if err := ValidateDurations(opts.CollectFor, opts.ScrapeEvery); err != nil {
		return Result{}, err
	}

	run, err := newCollectionRun(s, opts)
	if err != nil {
		return Result{}, err
	}
	defer run.close()

	for _, step := range []func() error{run.loadConfig, run.prepareLayout, run.prepareTools, run.configureTargets} {
		if err := step(); err != nil {
			return Result{}, err
		}
	}
	for _, step := range []func(context.Context) error{run.startExporters, run.waitForReadiness, run.runCollectionWindow, run.captureEvidence} {
		if err := step(ctx); err != nil {
			return Result{}, err
		}
	}
	return run.finalize()
}

func newCollectionRun(service Service, opts Options) (*collectionRun, error) {
	if opts.Version == "" {
		return nil, errors.New("collector version is required")
	}
	return &collectionRun{service: service, opts: opts}, nil
}

func (r *collectionRun) loadConfig() error {
	emitStep(r.opts.Stepf, StepConfig, "Loading local installation state", 0)

	store, err := config.NewStore()
	if err != nil {
		return err
	}
	cfg, err := store.Ensure()
	if err != nil {
		return err
	}
	runID, err := newRunID()
	if err != nil {
		return err
	}

	r.cfg = contracts.Job{
		RunID:            runID,
		InstallationID:   cfg.InstallationID,
		CollectorVersion: r.opts.Version,
		SchemaVersion:    contracts.SchemaVersion,
	}
	return nil
}

func (r *collectionRun) prepareLayout() error {
	runDir, artifactPath, rawPaths, err := prepareRunLayout(r.cfg.RunID, r.opts.OutputPath)
	if err != nil {
		return err
	}
	r.runDir = runDir
	r.artifactPath = artifactPath
	r.rawPaths = rawPaths
	r.warnings = collectWarnings(r.opts.CollectFor, r.opts.ScrapeEvery)
	for _, warning := range r.warnings {
		debug.Debugf("collection warning: %s", warning)
	}
	return nil
}

func (r *collectionRun) prepareTools() error {
	emitStep(r.opts.Stepf, StepTools, "Resolving bundled collection tools", 0)
	tools, err := resolveToolPaths()
	if err != nil {
		return err
	}
	r.tools = tools
	return nil
}

func (r *collectionRun) finalize() (Result, error) {
	artifact, minimumEvidenceMet := r.buildArtifact()
	emitStep(r.opts.Stepf, StepValidate, "Validating the run artifact", 0)
	if err := artifact.Validate(); err != nil {
		return Result{}, err
	}

	emitStep(r.opts.Stepf, StepPersist, "Persisting artifact and sidecars", 0)
	if err := writeJSONFile(r.artifactPath, artifact); err != nil {
		return Result{}, err
	}
	return Result{
		Artifact:           artifact,
		ArtifactPath:       r.artifactPath,
		RunDir:             r.runDir,
		MinimumEvidenceMet: minimumEvidenceMet,
		Warnings:           r.warnings,
	}, nil
}

func (r *collectionRun) requiredJobs() []string {
	jobs := []string{"node_exporter"}
	if r.vllmTarget != "" {
		jobs = append(jobs, "vllm")
	}
	if r.dcgmTarget != "" {
		jobs = append(jobs, "dcgm")
	}
	return jobs
}

func (r *collectionRun) close() {
	for _, proc := range []*managedProcess{r.dcgmProc, r.nodeProc, r.promProc} {
		if proc != nil {
			proc.Close()
		}
	}
}

func (r *collectionRun) warnf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	debug.Debugf("collector warning: %s", message)
	r.warnings = append(r.warnings, message)
}
