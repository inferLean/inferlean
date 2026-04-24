package upload

import "fmt"

type View struct{}

func NewView() View {
	return View{}
}

func (View) ShowUploadStart() {
	fmt.Println("[upload] uploading artifact...")
}

func (View) ShowUploadSuccess() {
	fmt.Println("[upload] upload accepted")
}

func (View) ShowReportFetchStart(runID string) {
	fmt.Printf("[upload] loading report for run_id=%s\n", runID)
}

func (View) ShowReportFetchSuccess(runID string) {
	fmt.Printf("[upload] report loaded for run_id=%s\n", runID)
}

func (View) ShowFailure(err error) {
	fmt.Printf("[upload] failed: %v\n", err)
}
