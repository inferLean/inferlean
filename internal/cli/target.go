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

type targetResolutionOptions struct {
	PID           int32
	NoInteractive bool
}

func resolveTarget(cmd *cobra.Command, opts targetResolutionOptions) (discovery.Result, error) {
	ctx := cmd.Context()
	service := discovery.NewService(process.SystemInspector{})
	interactive := isInteractiveTerminal(opts.NoInteractive)

	run := func(stepf func(discovery.StepUpdate)) (discovery.Result, error) {
		return service.Discover(ctx, discovery.Options{
			PID:     opts.PID,
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

	if err == nil {
		return result, nil
	}

	if errors.Is(err, discovery.ErrAmbiguous) {
		if interactive {
			selected, selectErr := selecttarget.Choose(result.Candidates)
			if selectErr != nil {
				return discovery.Result{}, selectErr
			}

			result.Selected = &selected
			result.Reason = "selected interactively because multiple vLLM deployments were found"
			return result, nil
		}

		output.RenderAmbiguity(cmd.OutOrStdout(), result)
		return result, fmt.Errorf("%w; rerun with --pid or in an interactive terminal", err)
	}

	return result, explainDiscoverError(ctx, err)
}

func isInteractiveTerminal(noInteractive bool) bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) && !noInteractive
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
