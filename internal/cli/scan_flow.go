package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/collector"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/internal/events"
	"github.com/inferLean/inferlean/internal/output"
	"github.com/inferLean/inferlean/internal/publish"
	"github.com/inferLean/inferlean/internal/ui/collectprogress"
	"github.com/inferLean/inferlean/internal/ui/publishprogress"
	"github.com/inferLean/inferlean/internal/ui/reportview"
)

func runScanCollection(
	cmd *cobra.Command,
	opts scanOptions,
	target discovery.Result,
	workload normalizedWorkloadInputs,
	interactive bool,
) (collector.Result, error) {
	service := collector.NewService()
	runCollect := func(stepf func(collector.StepUpdate)) (collector.Result, error) {
		return service.Collect(cmd.Context(), collector.Options{
			Target:         *target.Selected,
			CollectFor:     opts.CollectFor,
			ScrapeEvery:    opts.ScrapeEvery,
			WorkloadMode:   workload.mode,
			WorkloadTarget: workload.target,
			RepeatedPrefix: workload.repeatedPrefix,
			OutputPath:     opts.OutputPath,
			Stepf:          stepf,
			Version:        version,
		})
	}
	if interactive {
		return collectprogress.Run(cmd.Context(), runCollect)
	}
	return runCollect(nil)
}

func runScanPublish(
	cmd *cobra.Command,
	baseURL string,
	session config.AuthState,
	collectResult collector.Result,
	interactive bool,
) (publish.Result, error) {
	publisher := publish.NewService()
	runPublish := func(stepf func(publish.StepUpdate)) (publish.Result, error) {
		return publisher.Publish(cmd.Context(), publish.Options{
			BaseURL:  baseURL,
			Artifact: collectResult.Artifact,
			Auth:     session,
			Stepf:    stepf,
		})
	}

	if interactive {
		result, err := publishprogress.Run(cmd.Context(), runPublish)
		if err != nil {
			emitScanCrash(baseURL, session, collectResult, err)
		}
		return result, err
	}

	result, err := runPublish(nil)
	if err != nil {
		emitScanCrash(baseURL, session, collectResult, err)
	}
	return result, err
}

func finalizeScan(
	cmd *cobra.Command,
	store *config.Store,
	cfg config.Config,
	baseURL string,
	target discovery.Result,
	collectResult collector.Result,
	publishResult publish.Result,
	interactive bool,
) error {
	if publishResult.Auth.HasSession() {
		cfg.Auth = &publishResult.Auth
		if err := store.Save(cfg); err != nil {
			return err
		}
	}
	emitScanSuccess(baseURL, publishResult, collectResult)

	if publishResult.Report == nil {
		return fmt.Errorf("scan completed without a canonical report")
	}
	if interactive {
		return reportview.Run(*publishResult.Report)
	}

	output.RenderCollection(cmd.OutOrStdout(), target, collectResult)
	output.RenderPublication(cmd.OutOrStdout(), publishResult.Ack)
	output.RenderReportSummary(cmd.OutOrStdout(), *publishResult.Report)
	return nil
}

func emitScanCrash(baseURL string, session config.AuthState, collectResult collector.Result, err error) {
	if emitter, emitterErr := events.NewEmitter(); emitterErr == nil {
		_ = emitter.EmitAsync(baseURL, session, events.NewCrashEvent(
			collectResult.Artifact.Job.InstallationID,
			collectResult.Artifact.Job.RunID,
			"scan",
			"publish",
			err.Error(),
			map[string]string{"backend_url": baseURL},
		))
	}
}

func emitScanSuccess(baseURL string, publishResult publish.Result, collectResult collector.Result) {
	if emitter, emitterErr := events.NewEmitter(); emitterErr == nil {
		_ = emitter.EmitAsync(baseURL, publishResult.Auth, events.NewWorkflowEvent(
			collectResult.Artifact.Job.InstallationID,
			collectResult.Artifact.Job.RunID,
			"scan",
			"publish",
			"success",
			map[string]string{
				"backend_url": baseURL,
				"upload_id":   publishResult.Ack.UploadID,
			},
		))
	}
}
