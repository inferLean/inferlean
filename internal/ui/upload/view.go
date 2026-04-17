package upload

import "fmt"

type View struct{}

func NewView() View {
	return View{}
}

func (View) ShowStart() {
	fmt.Println("[upload] uploading artifact...")
}

func (View) ShowSuccess(reportURL string) {
	fmt.Printf("[upload] upload accepted, report_url=%s\n", reportURL)
}

func (View) ShowFailure(err error) {
	fmt.Printf("[upload] failed: %v\n", err)
}
