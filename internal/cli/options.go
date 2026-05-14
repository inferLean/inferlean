package cli

import "time"

type DiscoverFlags struct {
	PID               int32
	ContainerName     string
	PodName           string
	Namespace         string
	ExcludeProcesses  bool
	ExcludeDocker     bool
	ExcludeKubernetes bool
}

type CollectFlags struct {
	CollectFor              time.Duration
	ScrapeEvery             time.Duration
	OutputPath              string
	DCGMEndpoint            string
	AllowDCGMEstimation     bool
	DeclaredWorkloadMode    string
	DeclaredWorkloadTarget  string
	PrefixHeavy             string
	Multimodal              string
	RepeatedMultimodalMedia string
}

type collectIntentFlags struct {
	PrefixHeavy             *bool
	Multimodal              *bool
	RepeatedMultimodalMedia *bool
}

type UploadFlags struct {
	RequireReport bool
	RunID         string
}
