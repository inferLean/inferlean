package cli

import (
	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/output"
)

func newDiscoverCommand() *cobra.Command {
	var pid int32
	var noInteractive bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover a local vLLM deployment and parse its runtime configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := resolveTarget(cmd, targetResolutionOptions{
				PID:           pid,
				NoInteractive: noInteractive,
			})
			if err != nil {
				return err
			}

			output.RenderDiscovery(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().Int32Var(&pid, "pid", 0, "select a specific vLLM process by pid")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "disable the interactive target selector")

	return cmd
}
