package chrome

import "os"

const (
	HeaderTitle = "InferLean"
	HeaderTag   = " - optimization copilot for self-hosted LLM inference"
)

func Render(useColor bool) string {
	if !useColor {
		return HeaderTitle + HeaderTag
	}
	return "\x1b[48;5;25m\x1b[38;5;255m \x1b[1m" + HeaderTitle + "\x1b[22m" + HeaderTag + " \x1b[0m"
}

func UseColor() bool {
	_, disabled := os.LookupEnv("NO_COLOR")
	return !disabled
}
