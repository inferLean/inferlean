package process

import (
	"context"
	"time"

	gopsprocess "github.com/shirou/gopsutil/v4/process"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

func Discover(ctx context.Context, pidFilter int32) ([]shared.Candidate, error) {
	if pidFilter != 0 {
		proc, err := gopsprocess.NewProcess(pidFilter)
		if err != nil {
			return nil, nil
		}
		cand := fromProcess(ctx, proc)
		if shared.IsServeCommand(cand.RawCommandLine) {
			return []shared.Candidate{cand}, nil
		}
		return nil, nil
	}
	procs, err := gopsprocess.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]shared.Candidate, 0, len(procs))
	for _, proc := range procs {
		cand := fromProcess(ctx, proc)
		if !shared.IsServeCommand(cand.RawCommandLine) {
			continue
		}
		out = append(out, cand)
	}
	return out, nil
}

func fromProcess(ctx context.Context, proc *gopsprocess.Process) shared.Candidate {
	raw, _ := proc.CmdlineWithContext(ctx)
	exe, _ := proc.ExeWithContext(ctx)
	started := time.Time{}
	if ms, err := proc.CreateTimeWithContext(ctx); err == nil {
		started = time.UnixMilli(ms)
	}
	return shared.Candidate{
		Source:         "process",
		PID:            proc.Pid,
		Executable:     exe,
		RawCommandLine: raw,
		StartedAt:      started,
	}
}
