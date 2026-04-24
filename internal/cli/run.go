package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	runpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/run"
)

const (
	defaultCollectFor  = 30 * time.Second
	defaultScrapeEvery = 5 * time.Second
)

type runCommandOptions struct {
	target                  targetFlags
	collectFor              time.Duration
	scrapeEvery             time.Duration
	outputPath              string
	declaredWorkloadMode    string
	declaredWorkloadTarget  string
	prefixHeavy             string
	multimodal              string
	repeatedMultimodalMedia string
	backendURL              string
	requireUpload           bool
}

func newRunCommand() *cobra.Command {
	target := &targetFlags{}
	var collectFor time.Duration
	var scrapeEvery time.Duration
	var outputPath string
	var declaredWorkloadMode string
	var declaredWorkloadTarget string
	var prefixHeavy string
	var multimodal string
	var repeatedMultimodalMedia string
	backendURL := defaults.BackendURL
	var requireUpload bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run discover -> collect -> optional upload -> optional report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithOptions(cmd, runCommandOptions{
				target:                  *target,
				collectFor:              collectFor,
				scrapeEvery:             scrapeEvery,
				outputPath:              outputPath,
				declaredWorkloadMode:    declaredWorkloadMode,
				declaredWorkloadTarget:  declaredWorkloadTarget,
				prefixHeavy:             prefixHeavy,
				multimodal:              multimodal,
				repeatedMultimodalMedia: repeatedMultimodalMedia,
				backendURL:              backendURL,
				requireUpload:           requireUpload,
			})
		},
	}
	bindTargetFlags(cmd, target)
	cmd.Flags().DurationVar(&collectFor, "collect-for", defaultCollectFor, "collection duration")
	cmd.Flags().DurationVar(&scrapeEvery, "scrape-every", defaultScrapeEvery, "scrape interval")
	cmd.Flags().StringVar(&outputPath, "output", "", "artifact output path")
	cmd.Flags().StringVar(&declaredWorkloadMode, "workload-mode", "", "declared workload mode")
	cmd.Flags().StringVar(&declaredWorkloadTarget, "workload-target", "", "declared optimization target")
	cmd.Flags().StringVar(&prefixHeavy, "prefix-heavy", "auto", "prefix heavy (true|false|auto)")
	cmd.Flags().StringVar(&multimodal, "multimodal", "auto", "multimodal workload (true|false|auto)")
	cmd.Flags().StringVar(&repeatedMultimodalMedia, "repeated-multimodal-media", "auto", "same images/media repeat across requests (true|false|auto)")
	cmd.Flags().StringVar(&backendURL, "backend-url", defaults.BackendURL, "backend base URL")
	cmd.Flags().BoolVar(&requireUpload, "require-upload", false, "fail run when upload/report fails")
	return cmd
}

func runWithDefaultOptions(cmd *cobra.Command) error {
	return runWithOptions(cmd, runCommandOptions{
		collectFor:              defaultCollectFor,
		scrapeEvery:             defaultScrapeEvery,
		prefixHeavy:             "auto",
		multimodal:              "auto",
		repeatedMultimodalMedia: "auto",
		backendURL:              defaults.BackendURL,
	})
}

func runWithOptions(cmd *cobra.Command, opts runCommandOptions) error {
	application := appFromContext(cmd.Context())
	prefixValue, err := parseOptionalBool(opts.prefixHeavy)
	if err != nil {
		return err
	}
	multimodalValue, err := parseOptionalBool(opts.multimodal)
	if err != nil {
		return err
	}
	repeatedMultimodalMediaValue, err := parseOptionalBool(opts.repeatedMultimodalMedia)
	if err != nil {
		return err
	}
	result, err := application.run.Run(cmd.Context(), runpresenter.Options{
		Discover:                opts.target.toDiscoverOptions(),
		CollectFor:              opts.collectFor,
		ScrapeEvery:             opts.scrapeEvery,
		OutputPath:              opts.outputPath,
		Version:                 version,
		DeclaredWorkloadMode:    opts.declaredWorkloadMode,
		DeclaredWorkloadTarget:  opts.declaredWorkloadTarget,
		PrefixHeavy:             prefixValue,
		Multimodal:              multimodalValue,
		RepeatedMultimodalMedia: repeatedMultimodalMediaValue,
		NoInteractive:           opts.target.noInteractive,
		BackendURL:              opts.backendURL,
		RequireUpload:           opts.requireUpload,
	})
	if err != nil {
		return err
	}
	fmt.Printf("run artifact: %s\n", result.ArtifactPath)
	if result.UploadErr != nil {
		fmt.Printf("run upload warning: %v\n", result.UploadErr)
	}
	return nil
}
