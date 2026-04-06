package cli

import (
	"errors"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/collector"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/events"
	"github.com/inferLean/inferlean/internal/output"
	"github.com/inferLean/inferlean/internal/publish"
	"github.com/inferLean/inferlean/internal/ui/collectprogress"
	"github.com/inferLean/inferlean/internal/ui/publishprogress"
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
	var workloadMode string
	var workloadTarget string
	var outputPath string
	var publishArtifact bool
	var backendURL string

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
			normalizedWorkloadMode, err := collector.NormalizeWorkloadMode(workloadMode)
			if err != nil {
				return err
			}
			normalizedWorkloadTarget, err := collector.NormalizeWorkloadTarget(workloadTarget)
			if err != nil {
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
					Target:         *target.Selected,
					CollectFor:     collectFor,
					ScrapeEvery:    scrapeEvery,
					WorkloadMode:   normalizedWorkloadMode,
					WorkloadTarget: normalizedWorkloadTarget,
					OutputPath:     outputPath,
					Stepf:          stepf,
					Version:        version,
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

			var publishResult publish.Result
			if publishArtifact {
				store, err := config.NewStore()
				if err != nil {
					return err
				}
				cfg, err := store.Load()
				if err != nil {
					return err
				}

				baseURL, err := resolveBackendURL(backendURL, cfg.Auth)
				if err != nil {
					return err
				}

				var publishAuth config.AuthState
				if cfg.Auth != nil && cfg.Auth.HasSession() {
					if cfg.Auth.BackendURL == "" || baseURL == cfg.Auth.BackendURL {
						publishAuth = *cfg.Auth
					}
				}

				publisher := publish.NewService()
				runPublish := func(stepf func(publish.StepUpdate)) (publish.Result, error) {
					return publisher.Publish(cmd.Context(), publish.Options{
						BaseURL:  baseURL,
						Artifact: result.Artifact,
						Auth:     publishAuth,
						Stepf:    stepf,
					})
				}

				if isInteractiveTerminal(noInteractive) {
					publishResult, err = publishprogress.Run(cmd.Context(), runPublish)
				} else {
					publishResult, err = runPublish(nil)
				}
				if err != nil {
					if emitter, emitterErr := events.NewEmitter(); emitterErr == nil {
						_ = emitter.EmitAsync(baseURL, publishAuth, events.NewCrashEvent(
							result.Artifact.Job.InstallationID,
							result.Artifact.Job.RunID,
							"collect",
							"publish",
							err.Error(),
							map[string]string{"backend_url": baseURL},
						))
					}
					return err
				}

				if publishResult.Auth.HasSession() {
					cfg.Auth = &publishResult.Auth
					if err := store.Save(cfg); err != nil {
						return err
					}
				}
				if emitter, emitterErr := events.NewEmitter(); emitterErr == nil {
					_ = emitter.EmitAsync(baseURL, publishResult.Auth, events.NewWorkflowEvent(
						result.Artifact.Job.InstallationID,
						result.Artifact.Job.RunID,
						"collect",
						"publish",
						"success",
						map[string]string{
							"backend_url": baseURL,
							"upload_id":   publishResult.Ack.UploadID,
						},
					))
				}
			}

			output.RenderCollection(cmd.OutOrStdout(), target, result)
			if publishArtifact {
				output.RenderPublication(cmd.OutOrStdout(), publishResult.Ack)
				if publishResult.Report != nil {
					output.RenderReportSummary(cmd.OutOrStdout(), *publishResult.Report)
				} else {
					output.RenderSummaryPreview(cmd.OutOrStdout(), publishResult.SummaryPreview)
				}
			}
			return nil
		},
	}

	cmd.Flags().Int32Var(&pid, "pid", 0, "select a specific vLLM process by pid")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "disable the interactive target selector")
	cmd.Flags().DurationVar(&collectFor, "collect-for", defaultCollectFor, "how long to collect metrics before building the artifact")
	cmd.Flags().DurationVar(&collectFor, "collect-interval", defaultCollectFor, "alias for --collect-for")
	cmd.Flags().DurationVar(&scrapeEvery, "scrape-every", defaultScrapeEvery, "how often Prometheus scrapes configured targets during collection")
	cmd.Flags().StringVar(&workloadMode, "workload-mode", "", "workload mode for this run: realtime_chat, batch_processing, or mixed")
	cmd.Flags().StringVar(&workloadTarget, "workload-target", "", "optimization target for this run: latency, balanced, or throughput")
	cmd.Flags().StringVar(&outputPath, "output", "", "write the artifact to a specific path")
	cmd.Flags().BoolVar(&publishArtifact, "publish", false, "publish the collected artifact to the configured backend")
	cmd.Flags().StringVar(&backendURL, "backend-url", "", "InferLean backend base URL")
	_ = cmd.Flags().MarkHidden("collect-interval")

	return cmd
}
