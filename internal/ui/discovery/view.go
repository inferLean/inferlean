package discovery

import (
	"fmt"

	"github.com/inferLean/inferlean-main/new-cli/internal/vllmdiscovery"
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

func (View) ShowSelected(item vllmdiscovery.Candidate) {
	fmt.Printf("[discovery] selected source=%s pid=%d container=%s pod=%s\n", item.Source, item.PID, item.ContainerID, item.PodName)
}
