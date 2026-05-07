package process

import (
	"context"
	"fmt"
	"time"

	gopsprocess "github.com/shirou/gopsutil/v4/process"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

func Discover(ctx context.Context, pidFilter int32) ([]shared.Candidate, error) {
	if pidFilter != 0 {
		proc, err := gopsprocess.NewProcess(pidFilter)
		if err != nil {
			return nil, fmt.Errorf("inspect process %d: %w", pidFilter, err)
		}
		cand := fromProcess(ctx, proc)
		if shared.IsServeCommand(cand.RawCommandLine) {
			return []shared.Candidate{cand}, nil
		}
		return nil, fmt.Errorf("process %d is not a vLLM serve process", pidFilter)
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
	env := processEnv(ctx, proc)
	return shared.Candidate{
		Source:          "process",
		PID:             proc.Pid,
		Executable:      exe,
		RawCommandLine:  raw,
		MetricsEndpoint: shared.InferMetricsEndpoint(raw, env),
		StartedAt:       started,
	}
}

func processEnv(ctx context.Context, proc *gopsprocess.Process) []string {
	env, err := proc.EnvironWithContext(ctx)
	if err != nil {
		return nil
	}
	return env
}
