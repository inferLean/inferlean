package discovery

import (
	"errors"
	"time"
)

var (
	ErrNoCandidates      = errors.New("no vLLM deployments found")
	ErrAmbiguous         = errors.New("multiple vLLM deployments found")
	ErrPIDNotFound       = errors.New("specified pid was not found")
	ErrPIDNotVLLM        = errors.New("specified pid is not a vLLM process")
	ErrContainerNotFound = errors.New("specified container was not found")
	ErrContainerNotVLLM  = errors.New("specified container is not running vLLM")
	ErrPodNotFound       = errors.New("specified pod was not found")
	ErrPodNotVLLM        = errors.New("specified pod is not running vLLM")
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
	PID       int32
	Container string
	Pod       string
	Namespace string
	Stepf     func(StepUpdate)
	WithEnv   bool
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
	Target         TargetRef
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
	Model                 string
	ServedModelName       string
	Host                  string
	Port                  int
	PortDefaulted         bool
	TensorParallelSize    int
	DataParallelSize      int
	PipelineParallelSize  int
	MaxModelLen           int
	MaxNumBatchedTokens   int
	MaxNumSeqs            int
	GPUMemoryUtilization  float64
	KVCacheDType          string
	ChunkedPrefill        *bool
	PrefixCaching         *bool
	Quantization          string
	DType                 string
	GenerationConfig      string
	APIKeyConfigured      bool
	MultimodalFlags       []string
	AttentionBackend      string
	FlashinferPresent     *bool
	FlashAttentionPresent *bool
	ImageProcessor        string
	MultimodalCacheHints  []string
	EnvHints              map[string]string
}

func (g CandidateGroup) DisplayModel() string {
	if g.RuntimeConfig.Model != "" {
		return g.RuntimeConfig.Model
	}
	if g.RuntimeConfig.ServedModelName != "" {
		return g.RuntimeConfig.ServedModelName
	}

	return "vLLM deployment"
}
