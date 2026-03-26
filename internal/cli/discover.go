package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/internal/discovery/process"
	"github.com/inferLean/inferlean/internal/output"
	"github.com/inferLean/inferlean/internal/ui/progress"
	"github.com/inferLean/inferlean/internal/ui/selecttarget"
)

func newDiscoverCommand() *cobra.Command {
	var pid int32
	var noInteractive bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover a local vLLM deployment and parse its runtime configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			service := discovery.NewService(process.SystemInspector{})
			interactive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) && !noInteractive

			run := func(stepf func(discovery.StepUpdate)) (discovery.Result, error) {
				return service.Discover(ctx, discovery.Options{
					PID:     pid,
					Stepf:   stepf,
					WithEnv: true,
				})
			}

			var (
				result discovery.Result
				err    error
			)
			if interactive {
				result, err = progress.Run(ctx, run)
			} else {
				result, err = run(nil)
			}

			if err != nil {
				if errors.Is(err, discovery.ErrAmbiguous) {
					if interactive {
						selected, selectErr := selecttarget.Choose(result.Candidates)
						if selectErr != nil {
							return selectErr
						}
						result.Selected = &selected
						result.Reason = "selected interactively because multiple vLLM deployments were found"
						output.RenderDiscovery(cmd.OutOrStdout(), result)
						return nil
					}

					output.RenderAmbiguity(cmd.OutOrStdout(), result)
					return fmt.Errorf("%w; rerun with --pid or in an interactive terminal", err)
				}

				return explainDiscoverError(ctx, err)
			}

			output.RenderDiscovery(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().Int32Var(&pid, "pid", 0, "select a specific vLLM process by pid")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "disable the interactive target selector")

	return cmd
}

func explainDiscoverError(_ context.Context, err error) error {
	switch {
	case errors.Is(err, discovery.ErrNoCandidates):
		return fmt.Errorf("%w; start your vLLM deployment first or rerun with --debug for more detail", err)
	case errors.Is(err, discovery.ErrPIDNotFound):
		return fmt.Errorf("%w; verify the process is still running", err)
	case errors.Is(err, discovery.ErrPIDNotVLLM):
		return fmt.Errorf("%w; rerun without --pid to inspect detected candidates", err)
	default:
		return err
	}
}
