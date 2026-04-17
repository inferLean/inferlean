package collection

import "fmt"

type View struct{}

func NewView() View {
	return View{}
}

func (View) ShowStart(seconds float64) {
	fmt.Printf("[collect] collecting for %.0fs...\n", seconds)
}

func (View) ShowStep(message string) {
	fmt.Printf("[collect] %s\n", message)
}

func (View) ShowDone(path string) {
	fmt.Printf("[collect] artifact written: %s\n", path)
}
