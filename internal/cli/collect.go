package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	collectpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/collect"
	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
)

func newCollectCommand() *cobra.Command {
	target := &DiscoverFlags{}
	opts := &CollectFlags{}

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect local evidence and build artifact.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			selected, _, err := application.discover.Run(cmd.Context(), discoverpresenter.Options{
				PID:               target.PID,
				ContainerName:     target.ContainerName,
				PodName:           target.PodName,
				Namespace:         target.Namespace,
				ExcludeProcesses:  target.ExcludeProcesses,
				ExcludeDocker:     target.ExcludeDocker,
				ExcludeKubernetes: target.ExcludeKubernetes,
				NonInteractive:    application.nonInteractive,
			})
			if err != nil {
				return err
			}
			prefixValue, err := parseOptionalBool(opts.PrefixHeavy)
			if err != nil {
				return err
			}
			multimodalValue, err := parseOptionalBool(opts.Multimodal)
			if err != nil {
				return err
			}
			repeatedMultimodalMediaValue, err := parseOptionalBool(opts.RepeatedMultimodalMedia)
			if err != nil {
				return err
			}
			res, err := application.collect.Run(cmd.Context(), collectpresenter.Options{
				Target:                  selected,
				CollectFor:              opts.CollectFor,
				ScrapeEvery:             opts.ScrapeEvery,
				OutputPath:              opts.OutputPath,
				CollectorVersion:        version,
				DeclaredWorkloadMode:    opts.DeclaredWorkloadMode,
				DeclaredWorkloadTarget:  opts.DeclaredWorkloadTarget,
				PrefixHeavy:             prefixValue,
				Multimodal:              multimodalValue,
				RepeatedMultimodalMedia: repeatedMultimodalMediaValue,
				NonInteractive:          application.nonInteractive,
			})
			if err != nil {
				return err
			}
			fmt.Printf("artifact path: %s\n", res.ArtifactPath)
			return nil
		},
	}
	bindDiscoverFlags(cmd, target)
	bindCollectFlags(cmd, opts)
	return cmd
}

func bindCollectFlags(cmd *cobra.Command, opts *CollectFlags) {
	cmd.Flags().DurationVar(&opts.CollectFor, "collect-for", defaultCollectFor, "collection duration")
	cmd.Flags().DurationVar(&opts.ScrapeEvery, "scrape-every", defaultScrapeEvery, "scrape interval")
	cmd.Flags().StringVar(&opts.OutputPath, "output", "", "artifact output path")
	cmd.Flags().StringVar(&opts.DeclaredWorkloadMode, "workload-mode", "", "declared workload mode")
	cmd.Flags().StringVar(&opts.DeclaredWorkloadTarget, "workload-target", "", "declared optimization target")
	cmd.Flags().StringVar(&opts.PrefixHeavy, "prefix-heavy", "auto", "prefix heavy (true|false|auto)")
	cmd.Flags().StringVar(&opts.Multimodal, "multimodal", "auto", "multimodal workload (true|false|auto)")
	cmd.Flags().StringVar(&opts.RepeatedMultimodalMedia, "repeated-multimodal-media", "auto", "same images/media repeat across requests (true|false|auto)")
}
