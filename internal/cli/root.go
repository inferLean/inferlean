package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/debug"
)

var version = "dev"

type rootOptions struct {
	debug bool
}

func Execute() error {
	return newRootCommand(context.Background()).Execute()
}

func newRootCommand(ctx context.Context) *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "inferlean",
		Short:         "The optimization copilot for self-hosted LLM inference",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			debug.SetEnabled(opts.debug)
		},
	}

	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "show debug output")
	cmd.AddCommand(newCollectCommand())
	cmd.AddCommand(newDiscoverCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}
