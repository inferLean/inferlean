package intent

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

type View struct{}

func NewView() View {
	return View{}
}

func (View) ShowResolved(intent types.UserIntent) {
	fmt.Printf(
		"[intent] mode=%s target=%s prefix_heavy=%t multimodal=%t multimodal_cache=%t\n",
		intent.WorkloadMode,
		intent.WorkloadTarget,
		intent.PrefixHeavy,
		intent.Multimodal,
		intent.MultimodalCache,
	)
}
