package artifactnormalize

import (
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
)

type Input struct {
	Job               JobInput
	Target            TargetInput
	Configurations    types.Configurations
	Observations      ObservationsInput
	UserIntent        types.UserIntent
	CollectionQuality types.CollectionQuality
}

type JobInput struct {
	RunID            string
	InstallationID   string
	CollectorVersion string
	StartedAt        time.Time
	FinishedAt       time.Time
}

type TargetInput struct {
	PID             int32
	Executable      string
	RawCommandLine  string
	MetricsEndpoint string
	ContainerID     string
	PodName         string
}

type ObservationsInput struct {
	Prometheus map[string][]promcollector.Sample
}
