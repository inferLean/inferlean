package shared

import "testing"

func TestParseProcList(t *testing.T) {
	t.Parallel()
	processes := ParseProcList("1\t/sbin/init\n17\tpython -m vllm.entrypoints.openai.api_server --model qwen\nbad\tignored\n")
	if len(processes) != 2 {
		t.Fatalf("len(ParseProcList()) = %d, want 2", len(processes))
	}
	if processes[1].PID != 17 {
		t.Fatalf("PID = %d, want 17", processes[1].PID)
	}
	if processes[1].RawCommandLine != "python -m vllm.entrypoints.openai.api_server --model qwen" {
		t.Fatalf("RawCommandLine = %q", processes[1].RawCommandLine)
	}
}

func TestFirstVLLMProcessPID(t *testing.T) {
	t.Parallel()
	pid := FirstVLLMProcessPID([]ProcessSnapshot{
		{PID: 1, RawCommandLine: "/sbin/init"},
		{PID: 17, RawCommandLine: "python -m vllm.entrypoints.openai.api_server --model qwen"},
	})
	if pid != 17 {
		t.Fatalf("FirstVLLMProcessPID() = %d, want 17", pid)
	}
}
