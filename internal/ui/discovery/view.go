package discovery

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

type View struct{}

func NewView() View {
	return View{}
}

func (View) ShowStart() {
	fmt.Println("[discovery] scanning for running vLLM targets...")
}

func (View) ShowCandidates(items []vllmdiscovery.Candidate) {
	fmt.Printf("[discovery] found %d candidate(s)\n", len(items))
}

func (View) Select(candidates []vllmdiscovery.Candidate, noInteractive bool) (vllmdiscovery.Candidate, error) {
	if len(candidates) == 0 {
		return vllmdiscovery.Candidate{}, fmt.Errorf("no vLLM targets discovered")
	}
	if !shouldUseTUI(noInteractive, candidates) {
		return candidates[0], nil
	}
	return selectCandidateTUI(candidates)
}

func (View) ShowSelected(item vllmdiscovery.Candidate) {
	fmt.Printf("[discovery] selected source=%s pid=%d container=%s pod=%s\n", item.Source, item.PID, item.ContainerID, item.PodName)
}
