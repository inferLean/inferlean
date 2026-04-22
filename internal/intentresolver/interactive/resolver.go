package interactive

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/types"
	"golang.org/x/term"
)

type questionKey string

const (
	keyWorkloadMode            questionKey = "workload_mode"
	keyWorkloadTarget          questionKey = "workload_target"
	keyPrefixHeavy             questionKey = "prefix_heavy"
	keyMultimodal              questionKey = "multimodal"
	keyRepeatedMultimodalMedia questionKey = "repeated_multimodal_media"
)

type questionOption struct {
	title       string
	description string
	value       string
}

type question struct {
	key          questionKey
	prompt       string
	options      []questionOption
	defaultIndex int
}

func Resolve(seed types.UserIntent) (types.UserIntent, error) {
	questions := buildQuestions(seed)
	if len(questions) == 0 {
		return seed, nil
	}
	if !isInteractiveTTY() {
		return resolveWithPrompts(seed, questions)
	}
	return resolveWithTUI(seed, questions)
}

func isInteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func buildQuestions(seed types.UserIntent) []question {
	questions := make([]question, 0, 5)
	if strings.TrimSpace(seed.WorkloadMode) == "" {
		questions = append(questions, modeQuestion())
	}
	if strings.TrimSpace(seed.WorkloadTarget) == "" {
		questions = append(questions, targetQuestion())
	}
	questions = append(questions, yesNoQuestion(
		keyPrefixHeavy,
		"Prefix-heavy traffic?",
		seed.PrefixHeavy,
		"Repeated prefixes are common.",
	))
	questions = append(questions, yesNoQuestion(
		keyMultimodal,
		"Multimodal workload?",
		seed.Multimodal,
		"Requests include image, video, or audio.",
	))
	questions = append(questions, yesNoQuestion(
		keyRepeatedMultimodalMedia,
		"Do the same images/media repeat across requests?",
		seed.RepeatedMultimodalMedia,
		"Repeated multimodal content appears across separate requests.",
	))
	return questions
}

func modeQuestion() question {
	return question{
		key:    keyWorkloadMode,
		prompt: "Workload mode",
		options: []questionOption{
			{title: "realtime_chat", description: "Interactive, low-latency conversation.", value: "realtime_chat"},
			{title: "batch_processing", description: "Offline jobs optimized for throughput.", value: "batch_processing"},
			{title: "mixed", description: "Mix of realtime and batch traffic.", value: "mixed"},
		},
		defaultIndex: 2,
	}
}

func targetQuestion() question {
	return question{
		key:    keyWorkloadTarget,
		prompt: "Primary optimization target",
		options: []questionOption{
			{title: "latency", description: "Prioritize response and tail latency.", value: "latency"},
			{title: "throughput", description: "Prioritize tokens/sec and total volume.", value: "throughput"},
		},
		defaultIndex: 0,
	}
}

func yesNoQuestion(key questionKey, prompt string, yesDefault bool, detail string) question {
	defaultIndex := 1
	if yesDefault {
		defaultIndex = 0
	}
	return question{
		key:    key,
		prompt: prompt,
		options: []questionOption{
			{title: "yes", description: detail, value: "true"},
			{title: "no", description: detail, value: "false"},
		},
		defaultIndex: defaultIndex,
	}
}

func resolveWithPrompts(seed types.UserIntent, questions []question) (types.UserIntent, error) {
	intent := seed
	reader := bufio.NewReader(os.Stdin)
	for _, q := range questions {
		answer, err := askQuestion(reader, q)
		if err != nil {
			return intent, err
		}
		applyAnswer(&intent, q.key, answer)
	}
	return intent, nil
}

func askQuestion(reader *bufio.Reader, q question) (string, error) {
	if len(q.options) == 0 {
		return "", fmt.Errorf("question %s has no options", q.key)
	}
	defaultOption := q.options[boundedIndex(q.defaultIndex, len(q.options))]
	optionLabels := make([]string, 0, len(q.options))
	for _, option := range q.options {
		optionLabels = append(optionLabels, option.title)
	}
	fmt.Printf("%s (%s) [%s]: ", q.prompt, strings.Join(optionLabels, "|"), defaultOption.title)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	trimmed := strings.ToLower(strings.TrimSpace(line))
	if trimmed == "" {
		return defaultOption.value, nil
	}
	for _, option := range q.options {
		if trimmed == strings.ToLower(option.title) || trimmed == strings.ToLower(option.value) {
			return option.value, nil
		}
	}
	return defaultOption.value, nil
}

func applyAnswer(intent *types.UserIntent, key questionKey, value string) {
	switch key {
	case keyWorkloadMode:
		intent.WorkloadMode = value
	case keyWorkloadTarget:
		intent.WorkloadTarget = value
	case keyPrefixHeavy:
		intent.PrefixHeavy = parseBool(value)
	case keyMultimodal:
		intent.Multimodal = parseBool(value)
	case keyRepeatedMultimodalMedia:
		intent.RepeatedMultimodalMedia = parseBool(value)
	}
}

func parseBool(value string) bool {
	result, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && result
}

func boundedIndex(idx, length int) int {
	if length <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= length {
		return length - 1
	}
	return idx
}
