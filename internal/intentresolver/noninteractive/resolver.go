package noninteractive

import (
	"strings"

	"github.com/inferLean/inferlean-main/new-cli/internal/types"
)

type Input struct {
	WorkloadMode    string
	WorkloadTarget  string
	PrefixHeavy     *bool
	Multimodal      *bool
	MultimodalCache *bool
}

func Resolve(input Input) (types.UserIntent, bool) {
	intent := types.UserIntent{}
	hasAny := false
	if trimmed := strings.TrimSpace(input.WorkloadMode); trimmed != "" {
		intent.WorkloadMode = trimmed
		hasAny = true
	}
	if trimmed := strings.TrimSpace(input.WorkloadTarget); trimmed != "" {
		intent.WorkloadTarget = trimmed
		hasAny = true
	}
	if input.PrefixHeavy != nil {
		intent.PrefixHeavy = *input.PrefixHeavy
		hasAny = true
	}
	if input.Multimodal != nil {
		intent.Multimodal = *input.Multimodal
		hasAny = true
	}
	if input.MultimodalCache != nil {
		intent.MultimodalCache = *input.MultimodalCache
		hasAny = true
	}
	return intent, hasAny
}
