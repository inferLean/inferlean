package cli

import (
	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

func newDiscoverCommand() *cobra.Command {
	opts := &targetFlags{}
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover running vLLM targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			_, _, err := application.discover.Run(cmd.Context(), opts.toDiscoverOptions())
			return err
		},
	}
	bindTargetFlags(cmd, opts)
	return cmd
}

type targetFlags struct {
	pid               int32
	containerName     string
	podName           string
	namespace         string
	noInteractive     bool
	excludeProcesses  bool
	excludeDocker     bool
	excludeKubernetes bool
}

func bindTargetFlags(cmd *cobra.Command, opts *targetFlags) {
	cmd.Flags().Int32Var(&opts.pid, "pid", 0, "target process id")
	cmd.Flags().StringVar(&opts.containerName, "container", "", "target docker container name")
	cmd.Flags().StringVar(&opts.podName, "pod", "", "target kubernetes pod name")
	cmd.Flags().StringVar(&opts.namespace, "namespace", "", "kubernetes namespace")
	cmd.Flags().BoolVar(&opts.noInteractive, "non-interactive", false, "disable interactive chooser and intent prompts")
	cmd.Flags().BoolVar(&opts.excludeProcesses, "exclude-processes", false, "skip process-based discovery")
	cmd.Flags().BoolVar(&opts.excludeDocker, "exclude-docker", false, "skip docker-based discovery")
	cmd.Flags().BoolVar(&opts.excludeKubernetes, "exclude-kubernetes", false, "skip kubernetes-based discovery")
}

func (f targetFlags) toDiscoverOptions() vllmdiscovery.DiscoverOptions {
	return vllmdiscovery.DiscoverOptions{
		PID:               f.pid,
		ContainerName:     f.containerName,
		PodName:           f.podName,
		Namespace:         f.namespace,
		NoInteractive:     f.noInteractive,
		ExcludeProcesses:  f.excludeProcesses,
		ExcludeDocker:     f.excludeDocker,
		ExcludeKubernetes: f.excludeKubernetes,
	}
}
