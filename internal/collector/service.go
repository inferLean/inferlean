package collector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/debug"
	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

const (
	defaultCollectDirMode = 0o700
	runIDSize             = 16
	httpTimeout           = 5 * time.Second
	healthTimeout         = 20 * time.Second
)

type Step string

const (
	StepConfig    Step = "config"
	StepTools     Step = "tools"
	StepExporters Step = "exporters"
	StepHealthy   Step = "healthy"
	StepCollect   Step = "collect"
	StepFallbacks Step = "fallbacks"
	StepValidate  Step = "validate"
	StepPersist   Step = "persist"
)

type StepUpdate struct {
	Step       Step
	Message    string
	CollectFor time.Duration
}

type Options struct {
	Target      discovery.CandidateGroup
	CollectFor  time.Duration
	ScrapeEvery time.Duration
	OutputPath  string
	Stepf       func(StepUpdate)
	Version     string
}

type Result struct {
	Artifact           contracts.RunArtifact
	ArtifactPath       string
	RunDir             string
	MinimumEvidenceMet bool
	Warnings           []string
}

type Service struct {
	client *http.Client
}

type toolPaths struct {
	Prometheus   string
	NodeExporter string
}

type sourceCapture struct {
	Status        string
	Reason        string
	Artifacts     []string
	MetricPayload map[string]any
}

type managedProcess struct {
	cmd    *exec.Cmd
	stdout *os.File
	stderr *os.File
}

type runtimeArtifacts struct {
	prometheusConfig string
	vllmRaw          string
	hostRaw          string
	dcgmRaw          string
	nvidiaRaw        string
	processRaw       string
	prometheusStdout string
	prometheusStderr string
	nodeStdout       string
	nodeStderr       string
}

type promVectorResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type promTargetsResponse struct {
	Status string `json:"status"`
	Data   struct {
		ActiveTargets []struct {
			Labels map[string]string `json:"labels"`
			Health string            `json:"health"`
		} `json:"activeTargets"`
	} `json:"data"`
}

type nvidiaGPU struct {
	Name           string `json:"name"`
	Utilization    string `json:"utilization_gpu,omitempty"`
	MemoryUsedMiB  string `json:"memory_used_mib,omitempty"`
	MemoryTotalMiB string `json:"memory_total_mib,omitempty"`
	PowerDrawW     string `json:"power_draw_w,omitempty"`
	TemperatureC   string `json:"temperature_c,omitempty"`
	SMClockMHz     string `json:"sm_clock_mhz,omitempty"`
	MemClockMHz    string `json:"mem_clock_mhz,omitempty"`
	DriverVersion  string `json:"driver_version,omitempty"`
}

type nvidiaProcess struct {
	PID          string `json:"pid"`
	ProcessName  string `json:"process_name,omitempty"`
	GPUMemoryMiB string `json:"gpu_memory_mib,omitempty"`
	GPUUUID      string `json:"gpu_uuid,omitempty"`
}

type nvidiaSnapshot struct {
	GPUs          []nvidiaGPU     `json:"gpus,omitempty"`
	Processes     []nvidiaProcess `json:"processes,omitempty"`
	DriverVersion string          `json:"driver_version,omitempty"`
}

func NewService() Service {
	return Service{
		client: &http.Client{Timeout: httpTimeout},
	}
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

	store, err := config.NewStore()
	if err != nil {
		return Result{}, err
	}

	emitStep(opts.Stepf, StepConfig, "Loading local installation state", 0)
	cfg, err := store.Ensure()
	if err != nil {
		return Result{}, err
	}

	runID, err := newRunID()
	if err != nil {
		return Result{}, err
	}
	runDir, artifactPath, rawPaths, err := prepareRunLayout(runID, opts.OutputPath)
	if err != nil {
		return Result{}, err
	}

	warnings := collectWarnings(opts.CollectFor, opts.ScrapeEvery)
	for _, warning := range warnings {
		debug.Debugf("collection warning: %s", warning)
	}

	emitStep(opts.Stepf, StepTools, "Resolving bundled collection tools", 0)
	tools, err := resolveToolPaths()
	if err != nil {
		return Result{}, err
	}

	nodePort, err := reservePort()
	if err != nil {
		return Result{}, err
	}
	promPort, err := reservePort()
	if err != nil {
		return Result{}, err
	}

	vllmTarget := buildVLLMTarget(opts.Target)
	nodeTarget := fmt.Sprintf("127.0.0.1:%d", nodePort)
	dcgmTarget := discoverDCGMTarget()

	if err := os.MkdirAll(filepath.Join(runDir, "raw"), defaultCollectDirMode); err != nil {
		return Result{}, fmt.Errorf("create raw evidence directory: %w", err)
	}

	if err := writePrometheusConfig(rawPaths.prometheusConfig, opts.ScrapeEvery, vllmTarget, nodeTarget, dcgmTarget); err != nil {
		return Result{}, err
	}

	emitStep(opts.Stepf, StepExporters, "Starting local exporters", 0)
	nodeProc, err := startProcess(ctx, tools.NodeExporter, []string{
		"--web.listen-address=" + nodeTarget,
		"--log.level=error",
	}, rawPaths.nodeStdout, rawPaths.nodeStderr)
	if err != nil {
		return Result{}, fmt.Errorf("start node exporter: %w", err)
	}
	defer nodeProc.Close()

	promProc, err := startProcess(ctx, tools.Prometheus, []string{
		"--config.file=" + rawPaths.prometheusConfig,
		"--storage.tsdb.path=" + filepath.Join(runDir, "prometheus-data"),
		"--web.listen-address=127.0.0.1:" + strconv.Itoa(promPort),
		"--log.level=error",
	}, rawPaths.prometheusStdout, rawPaths.prometheusStderr)
	if err != nil {
		return Result{}, fmt.Errorf("start prometheus: %w", err)
	}
	defer promProc.Close()

	promBase := fmt.Sprintf("http://127.0.0.1:%d", promPort)

	emitStep(opts.Stepf, StepHealthy, "Waiting for Prometheus and scrape targets to become healthy", 0)
	if err := s.waitForPrometheusReady(ctx, promBase); err != nil {
		return Result{}, err
	}
	healthyJobs := []string{"node_exporter"}
	if vllmTarget != "" {
		healthyJobs = append(healthyJobs, "vllm")
	}
	if dcgmTarget != "" {
		healthyJobs = append(healthyJobs, "dcgm")
	}
	if err := s.waitForPrometheusTargets(ctx, promBase, healthyJobs); err != nil {
		debug.Debugf("target readiness warning: %v", err)
		warnings = append(warnings, "some scrape targets did not report healthy before collection started")
	}

	emitStep(opts.Stepf, StepCollect, "Collecting local evidence", opts.CollectFor)
	debug.Debugf("collection window started: collect_for=%s scrape_every=%s", opts.CollectFor, opts.ScrapeEvery)
	timer := time.NewTimer(opts.CollectFor)
	select {
	case <-ctx.Done():
		timer.Stop()
		return Result{}, ctx.Err()
	case <-timer.C:
	}

	vllmCapture := sourceCapture{Status: "missing", Reason: "vLLM metrics endpoint was not configured"}
	if vllmTarget != "" {
		vllmCapture = s.captureMetricsSource(ctx, promBase, "vllm", "http://"+vllmTarget+"/metrics", rawPaths.vllmRaw)
	}
	hostCapture := s.captureMetricsSource(ctx, promBase, "node_exporter", "http://"+nodeTarget+"/metrics", rawPaths.hostRaw)
	gpuCapture := sourceCapture{Status: "missing", Reason: "DCGM exporter endpoint was not detected"}
	if dcgmTarget != "" {
		gpuCapture = s.captureMetricsSource(ctx, promBase, "dcgm", "http://"+dcgmTarget+"/metrics", rawPaths.dcgmRaw)
	}

	emitStep(opts.Stepf, StepFallbacks, "Capturing fallback and local process evidence", 0)
	nvidiaCapture, nvidiaSnapshot := captureNvidiaSMI(ctx, rawPaths.nvidiaRaw)
	processCapture, processMetrics := captureProcessInspection(rawPaths.processRaw, opts.Target)
	env, envMeasurements := collectEnvironment(ctx, nvidiaSnapshot)

	sourceStates := map[string]contracts.SourceState{
		"vllm_metrics":       toSourceState(vllmCapture),
		"host_metrics":       toSourceState(hostCapture),
		"gpu_telemetry":      toSourceState(gpuCapture),
		"nvidia_smi":         toSourceState(nvidiaCapture),
		"process_inspection": toSourceState(processCapture),
	}

	completeness := computeCompleteness(sourceStates)
	minimumEvidenceMet := hasMinimumEvidence(sourceStates)

	artifact := contracts.RunArtifact{
		SchemaVersion: contracts.SchemaVersion,
		Job: contracts.Job{
			RunID:            runID,
			InstallationID:   cfg.InstallationID,
			CollectorVersion: opts.Version,
			SchemaVersion:    contracts.SchemaVersion,
			CollectedAt:      time.Now().UTC(),
		},
		Environment:   env,
		RuntimeConfig: toRuntimeConfig(opts.Target.RuntimeConfig),
		Metrics: contracts.Metrics{
			VLLM:      vllmCapture.MetricPayload,
			Host:      mergeMaps(hostCapture.MetricPayload, envMeasurements),
			GPU:       gpuCapture.MetricPayload,
			NvidiaSmi: nvidiaCapture.MetricPayload,
		},
		ProcessInspection: contracts.ProcessInspection{
			TargetProcess: contracts.TargetProcess{
				PID:            opts.Target.PrimaryPID,
				Executable:     opts.Target.Executable,
				RawCommandLine: opts.Target.RawCommandLine,
				EntryPoint:     opts.Target.EntryPoint,
				StartedAt:      timePointer(opts.Target.StartedAt),
			},
			ParseWarnings: append([]string{}, opts.Target.ParseWarnings...),
		},
		WorkloadObservations: contracts.WorkloadObservations{
			Summary: fmt.Sprintf("Collected local evidence for %s over %s", opts.Target.DisplayModel(), opts.CollectFor),
			Hints: map[string]string{
				"target_model": opts.Target.DisplayModel(),
			},
			Measurements: map[string]any{
				"collect_for_seconds":           opts.CollectFor.Seconds(),
				"scrape_every_seconds":          opts.ScrapeEvery.Seconds(),
				"minimum_required_evidence_met": minimumEvidenceMet,
				"process_inspection":            processMetrics,
			},
		},
		CollectionQuality: contracts.CollectionQuality{
			SourceStates:     sourceStates,
			MissingEvidence:  missingEvidence(sourceStates),
			DegradedEvidence: degradedEvidence(sourceStates),
			Completeness:     completeness,
			Summary:          qualitySummary(sourceStates, minimumEvidenceMet),
		},
	}

	emitStep(opts.Stepf, StepValidate, "Validating the run artifact", 0)
	if err := artifact.Validate(); err != nil {
		return Result{}, err
	}

	emitStep(opts.Stepf, StepPersist, "Persisting artifact and sidecars", 0)
	if err := writeJSONFile(artifactPath, artifact); err != nil {
		return Result{}, err
	}

	return Result{
		Artifact:           artifact,
		ArtifactPath:       artifactPath,
		RunDir:             runDir,
		MinimumEvidenceMet: minimumEvidenceMet,
		Warnings:           warnings,
	}, nil
}

func collectWarnings(collectFor, scrapeEvery time.Duration) []string {
	if collectFor/scrapeEvery >= 2 {
		return nil
	}
	return []string{"collection window may produce fewer than two scrapes; results may be less stable"}
}

func emitStep(stepf func(StepUpdate), step Step, message string, collectFor time.Duration) {
	if stepf == nil {
		return
	}
	stepf(StepUpdate{Step: step, Message: message, CollectFor: collectFor})
}

func newRunID() (string, error) {
	var buf [runIDSize]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func prepareRunLayout(runID, outputPath string) (string, string, runtimeArtifacts, error) {
	if outputPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", runtimeArtifacts{}, fmt.Errorf("resolve home directory: %w", err)
		}
		runDir := filepath.Join(home, ".inferlean", "runs", runID)
		outputPath = filepath.Join(runDir, "artifact.json")
	}

	artifactPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", "", runtimeArtifacts{}, fmt.Errorf("resolve artifact path: %w", err)
	}
	runDir := filepath.Dir(artifactPath)
	if err := os.MkdirAll(runDir, defaultCollectDirMode); err != nil {
		return "", "", runtimeArtifacts{}, fmt.Errorf("create run directory: %w", err)
	}

	rawDir := filepath.Join(runDir, "raw")
	return runDir, artifactPath, runtimeArtifacts{
		prometheusConfig: filepath.Join(rawDir, "prometheus.yml"),
		vllmRaw:          filepath.Join(rawDir, "vllm.metrics"),
		hostRaw:          filepath.Join(rawDir, "node_exporter.metrics"),
		dcgmRaw:          filepath.Join(rawDir, "dcgm.metrics"),
		nvidiaRaw:        filepath.Join(rawDir, "nvidia-smi.txt"),
		processRaw:       filepath.Join(rawDir, "process-inspection.json"),
		prometheusStdout: filepath.Join(rawDir, "prometheus.stdout.log"),
		prometheusStderr: filepath.Join(rawDir, "prometheus.stderr.log"),
		nodeStdout:       filepath.Join(rawDir, "node_exporter.stdout.log"),
		nodeStderr:       filepath.Join(rawDir, "node_exporter.stderr.log"),
	}, nil
}

func resolveToolPaths() (toolPaths, error) {
	root, err := resolveToolsRoot()
	if err != nil {
		return toolPaths{}, err
	}

	archDir := "linux_" + runtime.GOARCH
	prometheus, err := findToolExecutable(filepath.Join(root, archDir, "prometheus"), "prometheus")
	if err != nil {
		return toolPaths{}, fmt.Errorf("resolve prometheus binary: %w", err)
	}
	nodeExporter, err := findToolExecutable(filepath.Join(root, archDir, "node_exporter"), "node_exporter")
	if err != nil {
		return toolPaths{}, fmt.Errorf("resolve node exporter binary: %w", err)
	}

	return toolPaths{
		Prometheus:   prometheus,
		NodeExporter: nodeExporter,
	}, nil
}

func resolveToolsRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv("INFERLEAN_TOOLS_DIR")); override != "" {
		return override, nil
	}

	exePath, err := os.Executable()
	if err == nil {
		root := filepath.Join(filepath.Dir(exePath), "tools")
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			return root, nil
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		root := filepath.Join(cwd, "dist", "tools")
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			return root, nil
		}
	}

	return "", errors.New("could not locate bundled tools; install InferLean from a Linux release bundle or set INFERLEAN_TOOLS_DIR")
}

func findToolExecutable(root, name string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != name {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found under %s", name, root)
	}
	return found, nil
}

func reservePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func buildVLLMTarget(target discovery.CandidateGroup) string {
	if target.RuntimeConfig.Port == 0 {
		return ""
	}

	host := strings.TrimSpace(target.RuntimeConfig.Host)
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}

	return net.JoinHostPort(host, strconv.Itoa(target.RuntimeConfig.Port))
}

func discoverDCGMTarget() string {
	if endpoint := strings.TrimSpace(os.Getenv("INFERLEAN_DCGM_ENDPOINT")); endpoint != "" {
		return endpoint
	}

	for _, candidate := range []string{"127.0.0.1:9400", "localhost:9400"} {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://" + candidate + "/metrics")
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return candidate
		}
	}

	return ""
}

func writePrometheusConfig(path string, scrapeEvery time.Duration, vllmTarget, nodeTarget, dcgmTarget string) error {
	var b strings.Builder
	b.WriteString("global:\n")
	b.WriteString(fmt.Sprintf("  scrape_interval: %s\n", scrapeEvery))
	b.WriteString(fmt.Sprintf("  evaluation_interval: %s\n", scrapeEvery))
	b.WriteString("scrape_configs:\n")
	writeScrapeJob(&b, "node_exporter", nodeTarget)
	if vllmTarget != "" {
		writeScrapeJob(&b, "vllm", vllmTarget)
	}
	if dcgmTarget != "" {
		writeScrapeJob(&b, "dcgm", dcgmTarget)
	}

	if err := os.MkdirAll(filepath.Dir(path), defaultCollectDirMode); err != nil {
		return fmt.Errorf("create prometheus config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write prometheus config: %w", err)
	}
	return nil
}

func writeScrapeJob(b *strings.Builder, name, target string) {
	b.WriteString(fmt.Sprintf("  - job_name: %q\n", name))
	b.WriteString("    static_configs:\n")
	b.WriteString("      - targets:\n")
	b.WriteString(fmt.Sprintf("          - %q\n", target))
}

func startProcess(ctx context.Context, binary string, args []string, stdoutPath, stderrPath string) (*managedProcess, error) {
	if err := os.MkdirAll(filepath.Dir(stdoutPath), defaultCollectDirMode); err != nil {
		return nil, err
	}
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return nil, err
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		stdout.Close()
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		stdout.Close()
		stderr.Close()
		return nil, err
	}
	debug.Debugf("started process %s pid=%d", binary, cmd.Process.Pid)
	return &managedProcess{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

func (p *managedProcess) Close() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_, _ = p.cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
}

func (s Service) waitForPrometheusReady(ctx context.Context, baseURL string) error {
	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/-/ready", nil)
		if err == nil {
			resp, err := s.client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("prometheus did not become ready in time")
}

func (s Service) waitForPrometheusTargets(ctx context.Context, baseURL string, jobs []string) error {
	if len(jobs) == 0 {
		return nil
	}
	required := map[string]struct{}{}
	for _, job := range jobs {
		required[job] = struct{}{}
	}

	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/targets", nil)
		if err == nil {
			resp, err := s.client.Do(req)
			if err == nil {
				var body promTargetsResponse
				decodeErr := json.NewDecoder(resp.Body).Decode(&body)
				resp.Body.Close()
				if decodeErr == nil && body.Status == "success" {
					healthy := map[string]bool{}
					for _, target := range body.Data.ActiveTargets {
						job := target.Labels["job"]
						if _, ok := required[job]; !ok {
							continue
						}
						if target.Health == "up" {
							healthy[job] = true
						}
					}
					if len(healthy) == len(required) {
						return nil
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("not all scrape targets became healthy")
}

func (s Service) captureMetricsSource(ctx context.Context, promBase, job, rawURL, rawPath string) sourceCapture {
	rel := relativeArtifact(rawPath, filepath.Dir(filepath.Dir(rawPath)))
	capture := sourceCapture{
		Status:    "ok",
		Artifacts: []string{rel},
	}

	if err := fetchToFile(ctx, s.client, rawURL, rawPath); err != nil {
		capture.Status = "degraded"
		capture.Reason = fmt.Sprintf("could not capture raw metrics: %v", err)
	}

	series, err := s.queryJobVector(ctx, promBase, job)
	if err != nil {
		if capture.Status == "ok" {
			capture.Status = "degraded"
		}
		capture.Reason = fmt.Sprintf("could not query Prometheus for %s: %v", job, err)
		return capture
	}
	if len(series) == 0 {
		capture.Status = "missing"
		if capture.Reason == "" {
			capture.Reason = fmt.Sprintf("Prometheus did not return any series for job %s", job)
		}
		return capture
	}

	metricNames := make([]string, 0, len(series))
	for name := range series {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	capture.MetricPayload = map[string]any{
		"job":              job,
		"raw_evidence_ref": rel,
		"metric_names":     metricNames,
		"series":           series,
		"series_count":     len(series),
	}
	return capture
}

func fetchToFile(ctx context.Context, client *http.Client, rawURL, path string) error {
	if rawURL == "" {
		return errors.New("raw metrics URL was empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), defaultCollectDirMode); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func (s Service) queryJobVector(ctx context.Context, promBase, job string) (map[string]any, error) {
	query := url.QueryEscape(fmt.Sprintf("{job=%q}", job))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, promBase+"/api/v1/query?query="+query, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var body promVectorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("prometheus query status %q", body.Status)
	}

	byMetric := map[string]any{}
	for _, series := range body.Data.Result {
		name := series.Metric["__name__"]
		if name == "" {
			name = "unknown"
		}

		sample := map[string]any{
			"labels": stripMetricName(series.Metric),
		}
		if len(series.Value) == 2 {
			if timestamp, ok := series.Value[0].(float64); ok {
				sample["timestamp"] = time.Unix(int64(timestamp), 0).UTC().Format(time.RFC3339)
			}
			if value, ok := series.Value[1].(string); ok {
				sample["value"] = value
			}
		}

		current, _ := byMetric[name].([]map[string]any)
		current = append(current, sample)
		byMetric[name] = current
	}
	return byMetric, nil
}

func stripMetricName(labels map[string]string) map[string]string {
	clean := map[string]string{}
	for key, value := range labels {
		if key == "__name__" {
			continue
		}
		clean[key] = value
	}
	return clean
}

func captureNvidiaSMI(ctx context.Context, rawPath string) (sourceCapture, *nvidiaSnapshot) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return sourceCapture{
			Status: "missing",
			Reason: "nvidia-smi was not found in PATH",
		}, nil
	}

	queryOutput, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,driver_version,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return sourceCapture{
			Status: "degraded",
			Reason: fmt.Sprintf("nvidia-smi gpu query failed: %v", err),
		}, nil
	}

	processOutput, processErr := exec.CommandContext(ctx, "nvidia-smi",
		"--query-compute-apps=pid,process_name,used_gpu_memory,gpu_uuid",
		"--format=csv,noheader,nounits",
	).Output()
	if processErr != nil {
		debug.Debugf("nvidia-smi process query failed: %v", processErr)
	}

	rawContent := string(queryOutput)
	if len(processOutput) > 0 {
		rawContent += "\n# compute_apps\n" + string(processOutput)
	}
	if err := os.WriteFile(rawPath, []byte(rawContent), 0o600); err != nil {
		return sourceCapture{
			Status: "degraded",
			Reason: fmt.Sprintf("write nvidia-smi raw output: %v", err),
		}, nil
	}

	snapshot := parseNvidiaSMIOutput(queryOutput, processOutput)
	payload := map[string]any{
		"raw_evidence_ref": relativeArtifact(rawPath, filepath.Dir(filepath.Dir(rawPath))),
	}
	if snapshot != nil {
		payload["gpus"] = snapshot.GPUs
		payload["processes"] = snapshot.Processes
		payload["gpu_count"] = len(snapshot.GPUs)
		if snapshot.DriverVersion != "" {
			payload["driver_version"] = snapshot.DriverVersion
		}
	}

	status := "ok"
	reason := ""
	if processErr != nil {
		status = "degraded"
		reason = "gpu metrics were captured but per-process GPU memory was unavailable"
	}
	return sourceCapture{
		Status:        status,
		Reason:        reason,
		Artifacts:     []string{relativeArtifact(rawPath, filepath.Dir(filepath.Dir(rawPath)))},
		MetricPayload: payload,
	}, snapshot
}

func parseNvidiaSMIOutput(gpuOutput, processOutput []byte) *nvidiaSnapshot {
	snapshot := &nvidiaSnapshot{}
	lines := strings.Split(strings.TrimSpace(string(gpuOutput)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := splitCSVLine(line)
		gpu := nvidiaGPU{}
		if len(parts) > 0 {
			gpu.Name = parts[0]
		}
		if len(parts) > 1 {
			gpu.DriverVersion = parts[1]
			snapshot.DriverVersion = parts[1]
		}
		if len(parts) > 2 {
			gpu.Utilization = parts[2]
		}
		if len(parts) > 3 {
			gpu.MemoryUsedMiB = parts[3]
		}
		if len(parts) > 4 {
			gpu.MemoryTotalMiB = parts[4]
		}
		if len(parts) > 5 {
			gpu.PowerDrawW = parts[5]
		}
		if len(parts) > 6 {
			gpu.TemperatureC = parts[6]
		}
		if len(parts) > 7 {
			gpu.SMClockMHz = parts[7]
		}
		if len(parts) > 8 {
			gpu.MemClockMHz = parts[8]
		}
		snapshot.GPUs = append(snapshot.GPUs, gpu)
	}

	processLines := strings.Split(strings.TrimSpace(string(processOutput)), "\n")
	for _, line := range processLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(strings.ToLower(line), "no running processes found") {
			continue
		}
		parts := splitCSVLine(line)
		proc := nvidiaProcess{}
		if len(parts) > 0 {
			proc.PID = parts[0]
		}
		if len(parts) > 1 {
			proc.ProcessName = parts[1]
		}
		if len(parts) > 2 {
			proc.GPUMemoryMiB = parts[2]
		}
		if len(parts) > 3 {
			proc.GPUUUID = parts[3]
		}
		snapshot.Processes = append(snapshot.Processes, proc)
	}

	return snapshot
}

func splitCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	for idx := range parts {
		parts[idx] = strings.TrimSpace(parts[idx])
	}
	return parts
}

func captureProcessInspection(rawPath string, target discovery.CandidateGroup) (sourceCapture, map[string]any) {
	payload := map[string]any{
		"primary_pid":      target.PrimaryPID,
		"related_pids":     target.PIDs,
		"entry_point":      target.EntryPoint,
		"raw_command_line": target.RawCommandLine,
		"parse_warnings":   target.ParseWarnings,
	}
	if target.Executable != "" {
		payload["executable"] = target.Executable
	}
	if !target.StartedAt.IsZero() {
		payload["started_at"] = target.StartedAt.UTC().Format(time.RFC3339)
	}
	if err := writeJSONFile(rawPath, payload); err != nil {
		return sourceCapture{
			Status: "degraded",
			Reason: fmt.Sprintf("could not persist process inspection: %v", err),
		}, payload
	}

	return sourceCapture{
		Status:        "ok",
		Artifacts:     []string{relativeArtifact(rawPath, filepath.Dir(filepath.Dir(rawPath)))},
		MetricPayload: map[string]any{"raw_evidence_ref": relativeArtifact(rawPath, filepath.Dir(filepath.Dir(rawPath)))},
	}, payload
}

func collectEnvironment(ctx context.Context, snapshot *nvidiaSnapshot) (contracts.Environment, map[string]any) {
	env := contracts.Environment{
		OS: runtime.GOOS + "/" + runtime.GOARCH,
	}
	measurements := map[string]any{}

	if info, err := host.InfoWithContext(ctx); err == nil {
		env.Kernel = info.KernelVersion
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		env.CPUModel = infos[0].ModelName
	}
	if cores, err := cpu.CountsWithContext(ctx, true); err == nil {
		env.CPUCores = cores
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		env.MemoryBytes = int64(vm.Total)
		measurements["memory_used_bytes"] = vm.Used
		measurements["memory_available_bytes"] = vm.Available
	}
	if snapshot != nil {
		env.GPUCount = len(snapshot.GPUs)
		if len(snapshot.GPUs) > 0 {
			env.GPUModel = snapshot.GPUs[0].Name
			env.DriverVersion = snapshot.GPUs[0].DriverVersion
		}
	}

	return env, measurements
}

func toSourceState(capture sourceCapture) contracts.SourceState {
	return contracts.SourceState{
		Status:    capture.Status,
		Reason:    capture.Reason,
		Artifacts: capture.Artifacts,
	}
}

func computeCompleteness(states map[string]contracts.SourceState) float64 {
	weights := map[string]float64{
		"vllm_metrics":       0.30,
		"host_metrics":       0.20,
		"gpu_telemetry":      0.20,
		"nvidia_smi":         0.10,
		"process_inspection": 0.20,
	}
	score := 0.0
	for name, weight := range weights {
		state := states[name]
		switch state.Status {
		case "ok":
			score += weight
		case "degraded":
			score += weight * 0.5
		}
	}
	return score
}

func hasMinimumEvidence(states map[string]contracts.SourceState) bool {
	required := []string{"vllm_metrics", "host_metrics", "gpu_telemetry", "nvidia_smi", "process_inspection"}
	for _, key := range required {
		if states[key].Status != "ok" {
			return false
		}
	}
	return true
}

func missingEvidence(states map[string]contracts.SourceState) []string {
	var missing []string
	for name, state := range states {
		if state.Status == "missing" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func degradedEvidence(states map[string]contracts.SourceState) []string {
	var degraded []string
	for name, state := range states {
		if state.Status == "degraded" {
			degraded = append(degraded, name)
		}
	}
	sort.Strings(degraded)
	return degraded
}

func qualitySummary(states map[string]contracts.SourceState, minimumEvidenceMet bool) string {
	if minimumEvidenceMet {
		return "all required evidence sources were captured successfully"
	}

	var parts []string
	for name, state := range states {
		if state.Status == "ok" {
			continue
		}
		if state.Reason != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", name, state.Reason))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", name, state.Status))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func toRuntimeConfig(cfg discovery.RuntimeConfig) contracts.RuntimeConfig {
	return contracts.RuntimeConfig{
		Model:                cfg.Model,
		ServedModelName:      cfg.ServedModelName,
		Host:                 cfg.Host,
		Port:                 cfg.Port,
		TensorParallelSize:   cfg.TensorParallelSize,
		DataParallelSize:     cfg.DataParallelSize,
		PipelineParallelSize: cfg.PipelineParallelSize,
		MaxModelLen:          cfg.MaxModelLen,
		MaxNumBatchedTokens:  cfg.MaxNumBatchedTokens,
		MaxNumSeqs:           cfg.MaxNumSeqs,
		GPUMemoryUtilization: cfg.GPUMemoryUtilization,
		KVCacheDType:         cfg.KVCacheDType,
		ChunkedPrefill:       cfg.ChunkedPrefill,
		PrefixCaching:        cfg.PrefixCaching,
		Quantization:         cfg.Quantization,
		DType:                cfg.DType,
		GenerationConfig:     cfg.GenerationConfig,
		APIKeyConfigured:     cfg.APIKeyConfigured,
		MultimodalFlags:      append([]string{}, cfg.MultimodalFlags...),
		EnvHints:             cfg.EnvHints,
	}
}

func mergeMaps(parts ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, part := range parts {
		for key, value := range part {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), defaultCollectDirMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func relativeArtifact(path, runDir string) string {
	rel, err := filepath.Rel(runDir, path)
	if err != nil {
		return path
	}
	return rel
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	clone := value.UTC()
	return &clone
}
