package shared

import "time"

type Candidate struct {
	Source          string    `json:"source"`
	PID             int32     `json:"pid,omitempty"`
	Executable      string    `json:"executable,omitempty"`
	RawCommandLine  string    `json:"raw_command_line,omitempty"`
	MetricsEndpoint string    `json:"metrics_endpoint,omitempty"`
	ContainerID     string    `json:"container_id,omitempty"`
	PodName         string    `json:"pod_name,omitempty"`
	Namespace       string    `json:"namespace,omitempty"`
	StartedAt       time.Time `json:"started_at,omitempty"`
}

type DiscoverOptions struct {
	PID               int32
	ContainerName     string
	PodName           string
	Namespace         string
	NoInteractive     bool
	ExcludeProcesses  bool
	ExcludeDocker     bool
	ExcludeKubernetes bool
	CancelCurrent     <-chan struct{}
	OnSourceStart     func(source string)
	OnSourceCancelled func(source string)
}

const (
	SourceProcesses  = "processes"
	SourceDocker     = "docker"
	SourceKubernetes = "kubernetes"
)
