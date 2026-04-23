package noninteractive

import (
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

type Input struct {
	DeclaredWorkloadMode    string
	DeclaredWorkloadTarget  string
	PrefixHeavy             *bool
	Multimodal              *bool
	RepeatedMultimodalMedia *bool
}

func Resolve(input Input) (types.UserIntent, bool) {
	intent := types.UserIntent{}
	hasAny := false
	if trimmed := strings.TrimSpace(input.DeclaredWorkloadMode); trimmed != "" {
		intent.DeclaredWorkloadMode = trimmed
		hasAny = true
	}
	if trimmed := strings.TrimSpace(input.DeclaredWorkloadTarget); trimmed != "" {
		intent.DeclaredWorkloadTarget = trimmed
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
	if input.RepeatedMultimodalMedia != nil {
		intent.RepeatedMultimodalMedia = *input.RepeatedMultimodalMedia
		hasAny = true
	}
	return intent, hasAny
}
