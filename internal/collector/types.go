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

type collectionRun struct {
	service Service
	opts    Options

	cfg          contracts.Job
	runDir       string
	artifactPath string
	rawPaths     runtimeArtifacts
	warnings     []string

	tools      toolPaths
	nodePort   int
	promPort   int
	vllmTarget string
	nodeTarget string
	dcgmTarget string
	promBase   string

	nodeProc *managedProcess
	promProc *managedProcess

	vllmCapture    sourceCapture
	hostCapture    sourceCapture
	gpuCapture     sourceCapture
	nvidiaCapture  sourceCapture
	processCapture sourceCapture

	nvidiaSnapshot *nvidiaSnapshot
	processMetrics map[string]any
	env            contracts.Environment
	envMetrics     map[string]any
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
