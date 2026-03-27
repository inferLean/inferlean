package collector

import (
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

const (
	defaultCollectDirMode = 0o700
	runIDSuffixSize       = 4
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

type collectionRun struct {
	service Service
	opts    Options

	cfg          contracts.Job
	runDir       string
	artifactPath string
	rawPaths     runtimeArtifacts
	warnings     []string

	tools          toolPaths
	nodePort       int
	promPort       int
	vllmTarget     string
	nodeTarget     string
	dcgmTarget     string
	dcgmManaged    bool
	promBase       string
	collectStarted time.Time
	collectEnded   time.Time

	nodeProc *managedProcess
	promProc *managedProcess
	dcgmProc *managedProcess

	vllmCapture    sourceCapture
	hostCapture    sourceCapture
	gpuCapture     sourceCapture
	nvidiaCapture  sourceCapture
	processCapture sourceCapture

	vllmMetrics       contracts.VLLMMetrics
	hostMetrics       contracts.HostMetrics
	gpuMetrics        contracts.GPUTelemetry
	nvidiaMetrics     contracts.NvidiaSMIMetrics
	runtimeConfig     contracts.RuntimeConfig
	processInspection contracts.ProcessInspection
	nvidiaSnapshot    *nvidiaSnapshot
	nvmlSnapshot      *nvmlSnapshot
	nvmlCoverage      contracts.SourceCoverage
	processSamples    []processSample
	env               contracts.Environment
}

type toolPaths struct {
	Prometheus     string
	NodeExporter   string
	DCGMExporter   string
	DCGMCollectors string
}

type sourceCapture struct {
	Status    string
	Reason    string
	Artifacts []string
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
	vllmNormalized   string
	hostNormalized   string
	gpuNormalized    string
	nvidiaRaw        string
	processRaw       string
	processSamples   string
	nvmlRaw          string
	runtimeProbeRaw  string
	prometheusStdout string
	prometheusStderr string
	nodeStdout       string
	nodeStderr       string
	dcgmStdout       string
	dcgmStderr       string
}

type promVectorResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
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

type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []promRangeSeries `json:"result"`
	} `json:"data"`
}

type promRangeSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
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

type processSample struct {
	Timestamp  time.Time `json:"timestamp"`
	CPUPercent float64   `json:"cpu_percent"`
	RSSBytes   float64   `json:"rss_bytes"`
}

type nvmlGPU struct {
	Name             string  `json:"name"`
	Utilization      float64 `json:"utilization,omitempty"`
	MemoryUsedBytes  float64 `json:"memory_used_bytes,omitempty"`
	MemoryFreeBytes  float64 `json:"memory_free_bytes,omitempty"`
	MemoryTotalBytes float64 `json:"memory_total_bytes,omitempty"`
	SMClockMHz       float64 `json:"sm_clock_mhz,omitempty"`
	MemClockMHz      float64 `json:"mem_clock_mhz,omitempty"`
	PowerDrawWatts   float64 `json:"power_draw_watts,omitempty"`
	TemperatureC     float64 `json:"temperature_c,omitempty"`
	PCIeRxKBs        float64 `json:"pcie_rx_kbs,omitempty"`
	PCIeTxKBs        float64 `json:"pcie_tx_kbs,omitempty"`
}

type nvmlSample struct {
	Timestamp time.Time `json:"timestamp"`
	Driver    string    `json:"driver,omitempty"`
	GPUs      []nvmlGPU `json:"gpus,omitempty"`
}

type nvmlSnapshot struct {
	DriverVersion string       `json:"driver_version,omitempty"`
	Samples       []nvmlSample `json:"samples,omitempty"`
}
