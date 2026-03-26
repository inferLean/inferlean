package cli

import (
	"errors"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/collector"
	"github.com/inferLean/inferlean/internal/output"
	"github.com/inferLean/inferlean/internal/ui/collectprogress"
)

const (
	defaultCollectFor  = 30 * time.Second
	defaultScrapeEvery = 5 * time.Second
)

func newCollectCommand() *cobra.Command {
	var pid int32
	var noInteractive bool
	var collectFor time.Duration
	var scrapeEvery time.Duration
	var outputPath string

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect a validated local run artifact from a vLLM host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "linux" {
				return errors.New("collection is only supported on Linux in Phase 2")
			}
			if err := collector.ValidateDurations(collectFor, scrapeEvery); err != nil {
				return err
			}

			target, err := resolveTarget(cmd, targetResolutionOptions{
				PID:           pid,
				NoInteractive: noInteractive,
			})
			if err != nil {
				return err
			}

			service := collector.NewService()
			run := func(stepf func(collector.StepUpdate)) (collector.Result, error) {
				return service.Collect(cmd.Context(), collector.Options{
					Target:      *target.Selected,
					CollectFor:  collectFor,
					ScrapeEvery: scrapeEvery,
					OutputPath:  outputPath,
					Stepf:       stepf,
					Version:     version,
				})
			}

			var result collector.Result
			if isInteractiveTerminal(noInteractive) {
				result, err = collectprogress.Run(cmd.Context(), run)
			} else {
				result, err = run(nil)
			}
			if err != nil {
				return err
			}

			output.RenderCollection(cmd.OutOrStdout(), target, result)
			return nil
		},
	}

	cmd.Flags().Int32Var(&pid, "pid", 0, "select a specific vLLM process by pid")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "disable the interactive target selector")
	cmd.Flags().DurationVar(&collectFor, "collect-for", defaultCollectFor, "how long to collect metrics before building the artifact")
	cmd.Flags().DurationVar(&collectFor, "collect-interval", defaultCollectFor, "alias for --collect-for")
	cmd.Flags().DurationVar(&scrapeEvery, "scrape-every", defaultScrapeEvery, "how often Prometheus scrapes configured targets during collection")
	cmd.Flags().StringVar(&outputPath, "output", "", "write the artifact to a specific path")
	_ = cmd.Flags().MarkHidden("collect-interval")

	return cmd
}
