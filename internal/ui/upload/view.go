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

func (View) ShowFailure(err error) {
	fmt.Printf("[upload] failed: %v\n", err)
}
