package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	runpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/run"
)

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
			application := appFromContext(cmd.Context())
			prefixValue, err := parseOptionalBool(prefixHeavy)
			if err != nil {
				return err
			}
			multimodalValue, err := parseOptionalBool(multimodal)
			if err != nil {
				return err
			}
			repeatedMultimodalMediaValue, err := parseOptionalBool(repeatedMultimodalMedia)
			if err != nil {
				return err
			}
			result, err := application.run.Run(cmd.Context(), runpresenter.Options{
				Discover:                target.toDiscoverOptions(),
				CollectFor:              collectFor,
				ScrapeEvery:             scrapeEvery,
				OutputPath:              outputPath,
				Version:                 version,
				DeclaredWorkloadMode:    declaredWorkloadMode,
				DeclaredWorkloadTarget:  declaredWorkloadTarget,
				PrefixHeavy:             prefixValue,
				Multimodal:              multimodalValue,
				RepeatedMultimodalMedia: repeatedMultimodalMediaValue,
				NoInteractive:           target.noInteractive,
				BackendURL:              backendURL,
				RequireUpload:           requireUpload,
			})
			if err != nil {
				return err
			}
			fmt.Printf("run artifact: %s\n", result.ArtifactPath)
			if result.UploadErr != nil {
				fmt.Printf("run upload warning: %v\n", result.UploadErr)
			}
			return nil
		},
	}
	bindTargetFlags(cmd, target)
	cmd.Flags().DurationVar(&collectFor, "collect-for", 30*time.Second, "collection duration")
	cmd.Flags().DurationVar(&scrapeEvery, "scrape-every", 5*time.Second, "scrape interval")
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
