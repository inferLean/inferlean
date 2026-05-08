package shared

import (
	"strconv"
	"strings"
	"time"
)

const ProcListScript = "for p in /proc/[0-9]*; do pid=${p##*/}; cmd=$(tr '\\000' ' ' < \"$p/cmdline\" 2>/dev/null); printf '%s\\t%s\\n' \"$pid\" \"$cmd\"; done"

type ProcessSnapshot struct {
	PID            int32
	Executable     string
	RawCommandLine string
	Env            []string
	StartedAt      time.Time
}

func IsVLLMProcess(process ProcessSnapshot) bool {
	return IsServeCommand(process.RawCommandLine)
}

func VLLMProcesses(processes []ProcessSnapshot) []ProcessSnapshot {
	out := make([]ProcessSnapshot, 0, len(processes))
	for _, process := range processes {
		if IsVLLMProcess(process) {
			out = append(out, process)
		}
	}
	return out
}

func ParseProcList(raw string) []ProcessSnapshot {
	out := []ProcessSnapshot{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		pidText, cmdline, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(pidText))
		if err != nil || pid <= 0 {
			continue
		}
		out = append(out, ProcessSnapshot{
			PID:            int32(pid),
			RawCommandLine: strings.TrimSpace(cmdline),
		})
	}
	return out
}

func FirstVLLMProcessPID(processes []ProcessSnapshot) int32 {
	for _, process := range VLLMProcesses(processes) {
		return process.PID
	}
	return 0
}
