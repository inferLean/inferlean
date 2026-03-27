package collector

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type processSamplesResult struct {
	samples []processSample
	err     error
}

type nvmlSamplesResult struct {
	snapshot *nvmlSnapshot
	coverage contracts.SourceCoverage
	err      error
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
	r.dcgmTarget = discoverExternalDCGMTarget()
	r.dcgmManaged = r.dcgmTarget == "" && r.tools.DCGMExporter != ""
	if err := r.reserveManagedDCGMTarget(); err != nil {
		return err
	}
	r.promBase = "http://127.0.0.1:" + strconv.Itoa(promPort)
	return os.MkdirAll(rawDir(r.rawPaths.prometheusConfig), defaultCollectDirMode)
}

func (r *collectionRun) reserveManagedDCGMTarget() error {
	if !r.dcgmManaged {
		return nil
	}
	port, err := reservePort()
	if err != nil {
		return err
	}
	r.dcgmTarget = "127.0.0.1:" + strconv.Itoa(port)
	return nil
}

func (r *collectionRun) startExporters(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepExporters, "Starting local exporters", 0)

	nodeProc, err := startProcess(ctx, r.tools.NodeExporter, nodeExporterArgs(r.nodeTarget), r.rawPaths.nodeStdout, r.rawPaths.nodeStderr)
	if err != nil {
		return fmt.Errorf("start node exporter: %w", err)
	}
	r.nodeProc = nodeProc

	if err := r.startManagedDCGM(ctx); err != nil {
		r.warnf("bundled DCGM exporter could not be started; continuing with NVML GPU telemetry: %v", err)
		r.dcgmTarget = ""
		r.dcgmManaged = false
	}
	if err := writePrometheusConfig(r.rawPaths.prometheusConfig, r.opts.ScrapeEvery, r.vllmTarget, r.nodeTarget, r.dcgmTarget); err != nil {
		return err
	}
	return r.startPrometheus(ctx)
}

func (r *collectionRun) startManagedDCGM(ctx context.Context) error {
	if !r.dcgmManaged {
		return nil
	}
	proc, err := startProcess(
		ctx,
		r.tools.DCGMExporter,
		dcgmExporterArgs(r.dcgmTarget, r.opts.ScrapeEvery, r.tools.DCGMCollectors),
		r.rawPaths.dcgmStdout,
		r.rawPaths.dcgmStderr,
	)
	if err != nil {
		return err
	}
	r.dcgmProc = proc
	return nil
}

func (r *collectionRun) startPrometheus(ctx context.Context) error {
	promProc, err := startProcess(ctx, r.tools.Prometheus, prometheusArgs(r.rawPaths.prometheusConfig, r.runDir, r.promPort), r.rawPaths.prometheusStdout, r.rawPaths.prometheusStderr)
	if err != nil {
		r.close()
		return fmt.Errorf("start prometheus: %w", err)
	}
	r.promProc = promProc
	return nil
}

func (r *collectionRun) waitForReadiness(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepHealthy, "Waiting for Prometheus and scrape targets to become healthy", 0)
	if err := r.service.waitForPrometheusReady(ctx, r.promBase); err != nil {
		return err
	}
	if err := r.service.waitForPrometheusTargets(ctx, r.promBase, r.requiredJobs()); err != nil {
		r.warnf("some scrape targets did not report healthy before collection started: %v", err)
	}
	return nil
}

func (r *collectionRun) runCollectionWindow(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepCollect, "Collecting local evidence", r.opts.CollectFor)

	collectCtx, cancel := context.WithTimeout(ctx, r.opts.CollectFor)
	defer cancel()

	r.collectStarted = time.Now().UTC()
	processCh := r.startProcessSampler(collectCtx)
	nvmlCh := r.startNVMLSampler(collectCtx)
	<-collectCtx.Done()
	r.collectEnded = time.Now().UTC()
	r.finishSamplerResults(<-processCh, <-nvmlCh)
	return ctx.Err()
}

func (r *collectionRun) startProcessSampler(ctx context.Context) <-chan processSamplesResult {
	ch := make(chan processSamplesResult, 1)
	go func() {
		samples, err := collectProcessSamples(ctx, r.opts.Target.PIDs, r.opts.ScrapeEvery, r.rawPaths.processSamples)
		ch <- processSamplesResult{samples: samples, err: err}
	}()
	return ch
}

func (r *collectionRun) startNVMLSampler(ctx context.Context) <-chan nvmlSamplesResult {
	ch := make(chan nvmlSamplesResult, 1)
	go func() {
		snapshot, coverage, err := collectNVMLSamples(ctx, r.opts.ScrapeEvery, r.rawPaths.nvmlRaw)
		ch <- nvmlSamplesResult{snapshot: snapshot, coverage: coverage, err: err}
	}()
	return ch
}

func (r *collectionRun) finishSamplerResults(processResult processSamplesResult, nvmlResult nvmlSamplesResult) {
	if processResult.err != nil {
		r.warnf("process sampler failed; per-process CPU and memory evidence may be incomplete: %v", processResult.err)
	} else {
		r.processSamples = processResult.samples
	}
	if nvmlResult.err != nil {
		r.nvmlCoverage = missingCoverage(gpuRequiredFields, relativeRawArtifact(r.rawPaths.nvmlRaw))
		r.warnf("NVML sampling failed; rich GPU telemetry may rely on DCGM only: %v", nvmlResult.err)
		return
	}
	r.nvmlSnapshot = nvmlResult.snapshot
	r.nvmlCoverage = nvmlResult.coverage
}

func (r *collectionRun) captureEvidence(ctx context.Context) error {
	emitStep(r.opts.Stepf, StepFallbacks, "Capturing fallback and local process evidence", 0)
	if err := r.capturePrometheusSources(ctx); err != nil {
		return err
	}

	r.nvidiaCapture, r.nvidiaMetrics, r.nvidiaSnapshot = captureNvidiaSMIMetrics(ctx, r.rawPaths.nvidiaRaw)
	r.processCapture, r.processInspection = captureProcessInspection(ctx, r.rawPaths.processRaw, r.opts.Target)
	r.env = collectEnvironment(ctx, r.nvidiaSnapshot, r.nvmlSnapshot)
	r.runtimeConfig = probeRuntimeConfig(ctx, r.opts.Target, r.rawPaths.runtimeProbeRaw, driverVersion(r.nvidiaSnapshot, r.nvmlSnapshot))
	r.processInspection.ProbeWarnings = append([]string{}, r.runtimeConfig.ProbeWarnings...)
	r.processInspection.ProbeEvidenceRef = r.runtimeConfig.ProbeEvidenceRef
	return writeJSONFile(r.rawPaths.processRaw, r.processInspection)
}

func (r *collectionRun) capturePrometheusSources(ctx context.Context) error {
	for _, source := range []struct {
		target string
		path   string
		label  string
	}{
		{target: r.vllmTarget, path: r.rawPaths.vllmRaw, label: "vLLM metrics"},
		{target: r.nodeTarget, path: r.rawPaths.hostRaw, label: "node exporter metrics"},
		{target: r.dcgmTarget, path: r.rawPaths.dcgmRaw, label: "dcgm metrics"},
	} {
		if err := r.captureRawMetrics(ctx, source.target, source.path, source.label); err != nil {
			r.warnf("could not capture %s: %v", source.label, err)
		}
	}
	for _, step := range []func(context.Context) error{r.captureVLLMMetrics, r.captureHostMetrics, r.captureGPUTelemetry} {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *collectionRun) captureRawMetrics(ctx context.Context, target, path, label string) error {
	if target == "" {
		return nil
	}
	err := fetchToFile(ctx, r.service.client, "http://"+target+"/metrics", path)
	if err == nil {
		return nil
	}
	_ = os.MkdirAll(rawDir(path), defaultCollectDirMode)
	_ = os.WriteFile(path, []byte("# "+label+" capture error: "+err.Error()+"\n"), 0o600)
	return err
}

func driverVersion(nvidia *nvidiaSnapshot, nvml *nvmlSnapshot) string {
	if nvml != nil && nvml.DriverVersion != "" {
		return nvml.DriverVersion
	}
	if nvidia != nil {
		return nvidia.DriverVersion
	}
	return ""
}
