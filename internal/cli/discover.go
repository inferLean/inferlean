package cli

import (
	"github.com/spf13/cobra"

	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
)

func newDiscoverCommand() *cobra.Command {
	opts := &DiscoverFlags{}
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover running vLLM targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			_, _, err := application.discover.Run(cmd.Context(), discoverpresenter.Options{
				PID:               opts.PID,
				ContainerName:     opts.ContainerName,
				PodName:           opts.PodName,
				Namespace:         opts.Namespace,
				ExcludeProcesses:  opts.ExcludeProcesses,
				ExcludeDocker:     opts.ExcludeDocker,
				ExcludeKubernetes: opts.ExcludeKubernetes,
				NonInteractive:    application.nonInteractive,
			})
			return err
		},
	}
	bindDiscoverFlags(cmd, opts)
	return cmd
}

func bindDiscoverFlags(cmd *cobra.Command, opts *DiscoverFlags) {
	cmd.Flags().Int32Var(&opts.PID, "pid", 0, "target process id")
	cmd.Flags().StringVar(&opts.ContainerName, "container", "", "target docker container name")
	cmd.Flags().StringVar(&opts.PodName, "pod", "", "target kubernetes pod name")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "", "kubernetes namespace")
	cmd.Flags().BoolVar(&opts.ExcludeProcesses, "exclude-processes", false, "skip process-based discovery")
	cmd.Flags().BoolVar(&opts.ExcludeDocker, "exclude-docker", false, "skip docker-based discovery")
	cmd.Flags().BoolVar(&opts.ExcludeKubernetes, "exclude-kubernetes", false, "skip kubernetes-based discovery")
}
