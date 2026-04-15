package cli

import (
	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/collector"
)

type workloadFlagValues struct {
	mode           string
	target         string
	repeatedPrefix bool
}

type normalizedWorkloadInputs struct {
	mode           string
	target         string
	repeatedPrefix *bool
}

func bindWorkloadFlags(cmd *cobra.Command, values *workloadFlagValues) {
	cmd.Flags().StringVar(&values.mode, "workload-mode", "mixed", "workload mode for this run: realtime_chat, batch_processing, or mixed")
	cmd.Flags().StringVar(&values.target, "workload-target", "balanced", "optimization target for this run: latency, balanced, or throughput")
	cmd.Flags().BoolVar(&values.repeatedPrefix, "repeated-prefix-present", false, "whether requests share repeated prefixes or prompt templates")
}

func normalizeWorkloadInputs(values workloadFlagValues) (normalizedWorkloadInputs, error) {
	mode, err := collector.NormalizeWorkloadMode(values.mode)
	if err != nil {
		return normalizedWorkloadInputs{}, err
	}
	target, err := collector.NormalizeWorkloadTarget(values.target)
	if err != nil {
		return normalizedWorkloadInputs{}, err
	}

	value := values.repeatedPrefix
	repeated := &value

	return normalizedWorkloadInputs{
		mode:           mode,
		target:         target,
		repeatedPrefix: repeated,
	}, nil
}
