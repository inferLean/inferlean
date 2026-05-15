package cli

import (
	"time"

	"github.com/spf13/cobra"

	collectpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/collect"
	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
	reportpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/report"
	runpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/run"
	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
)

const (
	defaultCollectFor  = 60 * time.Second
	defaultScrapeEvery = time.Second
	runShort           = "Run discover -> collect -> upload -> report"
)

type runCommandOptions struct {
	target        DiscoverFlags
	collect       CollectFlags
	requireUpload bool
}

type runFlags struct {
	target        DiscoverFlags
	collect       CollectFlags
	requireUpload bool
}

func newRunCommand() *cobra.Command {
	flags := &runFlags{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: runShort,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithOptions(cmd, flags.options())
		},
	}
	bindRunFlags(cmd, flags)
	return cmd
}

func bindRunFlags(cmd *cobra.Command, flags *runFlags) {
	bindDiscoverFlags(cmd, &flags.target)
	bindCollectFlags(cmd, &flags.collect)
	cmd.Flags().BoolVar(&flags.requireUpload, "require-upload", false, "fail run when upload/report fails")
}

func (flags *runFlags) options() runCommandOptions {
	return runCommandOptions{
		target:        flags.target,
		collect:       flags.collect,
		requireUpload: flags.requireUpload,
	}
}

func runWithOptions(cmd *cobra.Command, opts runCommandOptions) error {
	application := appFromContext(cmd.Context())
	intent, err := parseCollectIntentFlags(opts.collect)
	if err != nil {
		return err
	}
	_, err = application.run.Run(cmd.Context(), runpresenter.Options{
		Discover: discoverpresenter.Options{
			PID:               opts.target.PID,
			ContainerName:     opts.target.ContainerName,
			PodName:           opts.target.PodName,
			Namespace:         opts.target.Namespace,
			ExcludeProcesses:  opts.target.ExcludeProcesses,
			ExcludeDocker:     opts.target.ExcludeDocker,
			ExcludeKubernetes: opts.target.ExcludeKubernetes,
			NonInteractive:    application.nonInteractive,
		},
		Collect: collectpresenter.Options{
			CollectFor:              opts.collect.CollectFor,
			ScrapeEvery:             opts.collect.ScrapeEvery,
			OutputPath:              opts.collect.OutputPath,
			DCGMEndpoint:            opts.collect.DCGMEndpoint,
			AllowDCGMEstimation:     opts.collect.AllowDCGMEstimation,
			CollectorVersion:        version,
			DeclaredWorkloadMode:    opts.collect.DeclaredWorkloadMode,
			DeclaredWorkloadTarget:  opts.collect.DeclaredWorkloadTarget,
			PrefixHeavy:             intent.PrefixHeavy,
			Multimodal:              intent.Multimodal,
			RepeatedMultimodalMedia: intent.RepeatedMultimodalMedia,
			NonInteractive:          application.nonInteractive,
		},
		Upload: uploadpresenter.Options{
			BackendURL:    application.backendURL,
			RequireReport: opts.requireUpload,
		},
		Report: reportpresenter.Options{
			BackendURL:     application.backendURL,
			NonInteractive: application.nonInteractive,
		},
	})
	return err
}
