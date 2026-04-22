package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	collectpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/collect"
)

func newCollectCommand() *cobra.Command {
	target := &targetFlags{}
	var collectFor time.Duration
	var scrapeEvery time.Duration
	var outputPath string
	var workloadMode string
	var workloadTarget string
	var prefixHeavy string
	var multimodal string
	var repeatedMultimodalMedia string

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect local evidence and build artifact.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			selected, _, err := application.discover.Run(cmd.Context(), target.toDiscoverOptions())
			if err != nil {
				return err
			}
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
			res, err := application.collect.Run(cmd.Context(), collectpresenter.Options{
				Target:                  selected,
				CollectFor:              collectFor,
				ScrapeEvery:             scrapeEvery,
				OutputPath:              outputPath,
				CollectorVersion:        version,
				WorkloadMode:            workloadMode,
				WorkloadTarget:          workloadTarget,
				PrefixHeavy:             prefixValue,
				Multimodal:              multimodalValue,
				RepeatedMultimodalMedia: repeatedMultimodalMediaValue,
				NoInteractive:           target.noInteractive,
			})
			if err != nil {
				return err
			}
			fmt.Printf("artifact path: %s\n", res.ArtifactPath)
			return nil
		},
	}
	bindTargetFlags(cmd, target)
	cmd.Flags().DurationVar(&collectFor, "collect-for", 30*time.Second, "collection duration")
	cmd.Flags().DurationVar(&scrapeEvery, "scrape-every", 5*time.Second, "scrape interval")
	cmd.Flags().StringVar(&outputPath, "output", "", "artifact output path")
	cmd.Flags().StringVar(&workloadMode, "workload-mode", "", "workload mode")
	cmd.Flags().StringVar(&workloadTarget, "workload-target", "", "workload target")
	cmd.Flags().StringVar(&prefixHeavy, "prefix-heavy", "auto", "prefix heavy (true|false|auto)")
	cmd.Flags().StringVar(&multimodal, "multimodal", "auto", "multimodal workload (true|false|auto)")
	cmd.Flags().StringVar(&repeatedMultimodalMedia, "repeated-multimodal-media", "auto", "same images/media repeat across requests (true|false|auto)")
	return cmd
}
