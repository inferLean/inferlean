package collector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
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
	for _, step := range []func(context.Context) error{run.startExporters, run.waitForReadiness, run.runCollectionWindow} {
		if err := step(ctx); err != nil {
			return Result{}, err
		}
	}

	run.captureMetrics(ctx)
	run.captureFallbacks(ctx)
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

func (r *collectionRun) configureTargets() error {
	nodePort, err := reservePort()
	if err != nil {
		return err
	}
	promPort, err := reservePort()
	if err != nil {
		return err
	}
	r.nodePort = nodePort
	r.promPort = promPort
	r.vllmTarget = buildVLLMTarget(r.opts.Target)
	r.nodeTarget = "127.0.0.1:" + strconv.Itoa(nodePort)
	r.dcgmTarget = discoverDCGMTarget()
	r.promBase = "http://127.0.0.1:" + strconv.Itoa(promPort)

	if err := os.MkdirAll(rawDir(r.rawPaths.prometheusConfig), defaultCollectDirMode); err != nil {
		return fmt.Errorf("create raw evidence directory: %w", err)
	}
	return writePrometheusConfig(r.rawPaths.prometheusConfig, r.opts.ScrapeEvery, r.vllmTarget, r.nodeTarget, r.dcgmTarget)
}

func (r *collectionRun) startExporters(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepExporters, "Starting local exporters", 0)

	nodeProc, err := startProcess(ctx, r.tools.NodeExporter, nodeExporterArgs(r.nodeTarget), r.rawPaths.nodeStdout, r.rawPaths.nodeStderr)
	if err != nil {
		return fmt.Errorf("start node exporter: %w", err)
	}
	promProc, err := startProcess(ctx, r.tools.Prometheus, prometheusArgs(r.rawPaths.prometheusConfig, r.runDir, r.promPort), r.rawPaths.prometheusStdout, r.rawPaths.prometheusStderr)
	if err != nil {
		nodeProc.Close()
		return fmt.Errorf("start prometheus: %w", err)
	}

	r.nodeProc = nodeProc
	r.promProc = promProc
	return nil
}

func (r *collectionRun) waitForReadiness(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepHealthy, "Waiting for Prometheus and scrape targets to become healthy", 0)
	if err := r.service.waitForPrometheusReady(ctx, r.promBase); err != nil {
		return err
	}
	if err := r.service.waitForPrometheusTargets(ctx, r.promBase, r.requiredJobs()); err != nil {
		debug.Debugf("target readiness warning: %v", err)
		r.warnings = append(r.warnings, "some scrape targets did not report healthy before collection started")
	}
	return nil
}

func (r *collectionRun) runCollectionWindow(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepCollect, "Collecting local evidence", r.opts.CollectFor)
	debug.Debugf("collection window started: collect_for=%s scrape_every=%s", r.opts.CollectFor, r.opts.ScrapeEvery)

	timer := time.NewTimer(r.opts.CollectFor)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r *collectionRun) captureMetrics(ctx context.Context) {
	r.vllmCapture = missingCapture("vLLM metrics endpoint was not configured")
	if r.vllmTarget != "" {
		r.vllmCapture = r.service.captureMetricsSource(ctx, r.promBase, "vllm", "http://"+r.vllmTarget+"/metrics", r.rawPaths.vllmRaw)
	}
	r.hostCapture = r.service.captureMetricsSource(ctx, r.promBase, "node_exporter", "http://"+r.nodeTarget+"/metrics", r.rawPaths.hostRaw)
	r.gpuCapture = missingCapture("DCGM exporter endpoint was not detected")
	if r.dcgmTarget != "" {
		r.gpuCapture = r.service.captureMetricsSource(ctx, r.promBase, "dcgm", "http://"+r.dcgmTarget+"/metrics", r.rawPaths.dcgmRaw)
	}
}

func (r *collectionRun) captureFallbacks(ctx context.Context) {
	emitStep(r.opts.Stepf, StepFallbacks, "Capturing fallback and local process evidence", 0)

	r.nvidiaCapture, r.nvidiaSnapshot = captureNvidiaSMI(ctx, r.rawPaths.nvidiaRaw)
	r.processCapture, r.processMetrics = captureProcessInspection(r.rawPaths.processRaw, r.opts.Target)
	r.env, r.envMetrics = collectEnvironment(ctx, r.nvidiaSnapshot)
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
	if r.nodeProc != nil {
		r.nodeProc.Close()
	}
	if r.promProc != nil {
		r.promProc.Close()
	}
}
