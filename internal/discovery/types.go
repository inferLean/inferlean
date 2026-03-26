package discovery

import (
	"errors"
	"time"
)

var (
	ErrNoCandidates = errors.New("no vLLM deployments found")
	ErrAmbiguous    = errors.New("multiple vLLM deployments found")
	ErrPIDNotFound  = errors.New("specified pid was not found")
	ErrPIDNotVLLM   = errors.New("specified pid is not a vLLM process")
)

type Step string

const (
	StepEnumerate Step = "enumerate"
	StepParse     Step = "parse"
	StepResolve   Step = "resolve"
)

type StepUpdate struct {
	Step    Step
	Message string
}

type Options struct {
	PID     int32
	Stepf   func(StepUpdate)
	WithEnv bool
}

type Result struct {
	Selected   *CandidateGroup
	Candidates []CandidateGroup
	Reason     string
	Warnings   []string
}

type CandidateGroup struct {
	Key            string
	ProcessCount   int
	PrimaryPID     int32
	PIDs           []int32
	EntryPoint     string
	Executable     string
	ParentPID      int32
	StartedAt      time.Time
	RawCommandLine string
	CommandExcerpt string
	RuntimeConfig  RuntimeConfig
	ParseWarnings  []string
}

type RuntimeConfig struct {
	Model                string
	Host                 string
	Port                 int
	TensorParallelSize   int
	DataParallelSize     int
	PipelineParallelSize int
	MaxModelLen          int
	MaxNumBatchedTokens  int
	MaxNumSeqs           int
	GPUMemoryUtilization float64
	KVCacheDType         string
	ChunkedPrefill       *bool
	PrefixCaching        *bool
	Quantization         string
	MultimodalFlags      []string
	EnvHints             map[string]string
}

func (g CandidateGroup) DisplayModel() string {
	if g.RuntimeConfig.Model != "" {
		return g.RuntimeConfig.Model
	}

	return "vLLM deployment"
}
