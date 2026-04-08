package cli

import (
	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/output"
)

func newDiscoverCommand() *cobra.Command {
	var pid int32
	var container string
	var pod string
	var namespace string
	var noInteractive bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover a local vLLM deployment and parse its runtime configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := resolveTarget(cmd, targetResolutionOptions{
				PID:           pid,
				Container:     container,
				Pod:           pod,
				Namespace:     namespace,
				NoInteractive: noInteractive,
			})
			if err != nil {
				return err
			}

			output.RenderDiscovery(cmd.OutOrStdout(), result)
			return nil
		},
	}

	bindTargetFlags(cmd, &pid, &container, &pod, &namespace, &noInteractive)

	return cmd
}
