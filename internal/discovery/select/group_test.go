package selection

import "testing"

func TestBuildGroupsGroupsRelatedProcesses(t *testing.T) {
	t.Parallel()

	groups := BuildGroups([]CandidateProcess{
		{
			PID:           100,
			EntryPoint:    "vllm serve",
			Signature:     "sig-a",
			RuntimeConfig: RuntimeConfig{Model: "meta-llama/Llama-3.1-8B-Instruct", Port: 8000},
		},
		{
			PID:           101,
			PPID:          100,
			EntryPoint:    "vllm serve",
			Signature:     "sig-a",
			RuntimeConfig: RuntimeConfig{Model: "meta-llama/Llama-3.1-8B-Instruct", Port: 8000},
		},
		{
			PID:           200,
			EntryPoint:    "vllm serve",
			Signature:     "sig-b",
			RuntimeConfig: RuntimeConfig{Model: "mistralai/Mistral-7B-Instruct-v0.3", Port: 8001},
		},
	})

	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}

	if groups[0].ProcessCount != 2 {
		t.Fatalf("first group process count = %d, want 2", groups[0].ProcessCount)
	}
}

func TestFindByPID(t *testing.T) {
	t.Parallel()

	group := Group{PrimaryPID: 100, PIDs: []int32{100, 101}}
	found := FindByPID([]Group{group}, 101)
	if found == nil {
		t.Fatal("expected group to be found")
	}
	if found.PrimaryPID != 100 {
		t.Fatalf("primary pid = %d, want 100", found.PrimaryPID)
	}
}
