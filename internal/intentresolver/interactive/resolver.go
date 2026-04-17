package interactive

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/inferLean/inferlean-main/new-cli/internal/types"
)

func Resolve(seed types.UserIntent) (types.UserIntent, error) {
	intent := seed
	reader := bufio.NewReader(os.Stdin)
	if strings.TrimSpace(intent.WorkloadMode) == "" {
		mode, err := ask(reader, "Workload mode (realtime_chat|batch_processing|mixed) [mixed]: ", "mixed")
		if err != nil {
			return intent, err
		}
		intent.WorkloadMode = mode
	}
	if strings.TrimSpace(intent.WorkloadTarget) == "" {
		target, err := ask(reader, "Workload target (latency|throughput) [latency]: ", "latency")
		if err != nil {
			return intent, err
		}
		intent.WorkloadTarget = target
	}
	prefixAnswer, err := ask(reader, "Prefix heavy traffic? (y/N): ", "n")
	if err != nil {
		return intent, err
	}
	intent.PrefixHeavy = isYes(prefixAnswer)
	multimodalAnswer, err := ask(reader, "Multimodal workload? (y/N): ", "n")
	if err != nil {
		return intent, err
	}
	intent.Multimodal = isYes(multimodalAnswer)
	cacheAnswer, err := ask(reader, "Multimodal cache enabled? (y/N): ", "n")
	if err != nil {
		return intent, err
	}
	intent.MultimodalCache = isYes(cacheAnswer)
	return intent, nil
}

func isYes(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "y")
}

func ask(reader *bufio.Reader, prompt, defaultValue string) (string, error) {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return defaultValue, nil
	}
	return trimmed, nil
}
