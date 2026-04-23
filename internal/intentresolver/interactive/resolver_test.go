package interactive

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestBuildQuestions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		seed          types.UserIntent
		wantQuestions int
		firstKey      questionKey
	}{
		{
			name:          "asks all fields when empty",
			seed:          types.UserIntent{},
			wantQuestions: 5,
			firstKey:      keyDeclaredWorkloadMode,
		},
		{
			name: "skips mode and target when provided",
			seed: types.UserIntent{
				DeclaredWorkloadMode:   "mixed",
				DeclaredWorkloadTarget: "latency",
			},
			wantQuestions: 3,
			firstKey:      keyPrefixHeavy,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			questions := buildQuestions(tc.seed)
			if len(questions) != tc.wantQuestions {
				t.Fatalf("len(buildQuestions())=%d, want %d", len(questions), tc.wantQuestions)
			}
			if questions[0].key != tc.firstKey {
				t.Fatalf("questions[0].key=%s, want %s", questions[0].key, tc.firstKey)
			}
		})
	}
}

func TestApplyAnswer(t *testing.T) {
	t.Parallel()
	intent := types.UserIntent{}
	applyAnswer(&intent, keyDeclaredWorkloadMode, "mixed")
	applyAnswer(&intent, keyDeclaredWorkloadTarget, "throughput")
	applyAnswer(&intent, keyPrefixHeavy, "true")
	applyAnswer(&intent, keyMultimodal, "false")
	applyAnswer(&intent, keyRepeatedMultimodalMedia, "true")

	if intent.DeclaredWorkloadMode != "mixed" {
		t.Fatalf("DeclaredWorkloadMode=%q, want mixed", intent.DeclaredWorkloadMode)
	}
	if intent.DeclaredWorkloadTarget != "throughput" {
		t.Fatalf("DeclaredWorkloadTarget=%q, want throughput", intent.DeclaredWorkloadTarget)
	}
	if !intent.PrefixHeavy {
		t.Fatalf("PrefixHeavy=%v, want true", intent.PrefixHeavy)
	}
	if intent.Multimodal {
		t.Fatalf("Multimodal=%v, want false", intent.Multimodal)
	}
	if !intent.RepeatedMultimodalMedia {
		t.Fatalf("RepeatedMultimodalMedia=%v, want true", intent.RepeatedMultimodalMedia)
	}
}

func TestYesNoDefaultIndex(t *testing.T) {
	t.Parallel()
	yesDefault := yesNoQuestion(keyPrefixHeavy, "Prefix-heavy traffic?", true, "desc")
	noDefault := yesNoQuestion(keyPrefixHeavy, "Prefix-heavy traffic?", false, "desc")

	if yesDefault.defaultIndex != 0 {
		t.Fatalf("yes default index=%d, want 0", yesDefault.defaultIndex)
	}
	if noDefault.defaultIndex != 1 {
		t.Fatalf("no default index=%d, want 1", noDefault.defaultIndex)
	}
}
