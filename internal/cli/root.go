package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/debug"
)

var version = "dev"

type rootOptions struct {
	debug     bool
	debugFile string
}

func Execute() error {
	defer debug.Close()
	return newRootCommand(context.Background()).Execute()
}

func newRootCommand(ctx context.Context) *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "inferlean",
		Short:         "The optimization copilot for self-hosted LLM inference",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return debug.Configure(opts.debug, opts.debugFile)
		},
	}

	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "show debug output")
	cmd.PersistentFlags().StringVar(&opts.debugFile, "debug-file", "", "write debug output to a file")
	cmd.AddCommand(newCollectCommand())
	cmd.AddCommand(newDiscoverCommand())
	cmd.AddCommand(newLoginCommand())
	cmd.AddCommand(newLogoutCommand())
	cmd.AddCommand(newRunsCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}
