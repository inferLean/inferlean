package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
	runpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/run"
)

const (
	defaultCollectFor  = 30 * time.Second
	defaultScrapeEvery = time.Second
)

type runCommandOptions struct {
	target        DiscoverFlags
	collect       CollectFlags
	requireUpload bool
}

func newRunCommand() *cobra.Command {
	target := &DiscoverFlags{}
	collect := &CollectFlags{}
	var requireUpload bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run discover -> collect -> upload -> report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithOptions(cmd, runCommandOptions{
				target:        *target,
				collect:       *collect,
				requireUpload: requireUpload,
			})
		},
	}
	bindDiscoverFlags(cmd, target)
	bindCollectFlags(cmd, collect)
	cmd.Flags().BoolVar(&requireUpload, "require-upload", false, "fail run when upload/report fails")
	return cmd
}

func runWithDefaultOptions(cmd *cobra.Command) error {
	return runWithOptions(cmd, runCommandOptions{
		collect: CollectFlags{
			CollectFor:              defaultCollectFor,
			ScrapeEvery:             defaultScrapeEvery,
			PrefixHeavy:             "auto",
			Multimodal:              "auto",
			RepeatedMultimodalMedia: "auto",
		},
	})
}

func runWithOptions(cmd *cobra.Command, opts runCommandOptions) error {
	application := appFromContext(cmd.Context())
	prefixValue, err := parseOptionalBool(opts.collect.PrefixHeavy)
	if err != nil {
		return err
	}
	multimodalValue, err := parseOptionalBool(opts.collect.Multimodal)
	if err != nil {
		return err
	}
	repeatedMultimodalMediaValue, err := parseOptionalBool(opts.collect.RepeatedMultimodalMedia)
	if err != nil {
		return err
	}
	result, err := application.run.Run(cmd.Context(), runpresenter.Options{
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
		CollectFor:              opts.collect.CollectFor,
		ScrapeEvery:             opts.collect.ScrapeEvery,
		OutputPath:              opts.collect.OutputPath,
		Version:                 version,
		DeclaredWorkloadMode:    opts.collect.DeclaredWorkloadMode,
		DeclaredWorkloadTarget:  opts.collect.DeclaredWorkloadTarget,
		PrefixHeavy:             prefixValue,
		Multimodal:              multimodalValue,
		RepeatedMultimodalMedia: repeatedMultimodalMediaValue,
		NonInteractive:          application.nonInteractive,
		BackendURL:              application.appURL,
		RequireUpload:           opts.requireUpload,
	})
	if err != nil {
		return err
	}
	if result.Failed {
		fmt.Println("status: fail")
		if strings.TrimSpace(result.FailureReason) != "" {
			fmt.Printf("reason: %s\n", result.FailureReason)
		}
		if strings.TrimSpace(result.FailureHint) != "" {
			fmt.Printf("hint: %s\n", result.FailureHint)
		}
		return fmt.Errorf("run failed: %s", strings.TrimSpace(result.FailureReason))
	}
	if result.UploadErr != nil {
		fmt.Printf("run upload warning: %v\n", result.UploadErr)
	}
	return nil
}
