package process

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

type Snapshot struct {
	PID            int32
	PPID           int32
	Executable     string
	RawCommandLine string
	Args           []string
	StartedAt      time.Time
	EnvHints       map[string]string
}

type Inspector interface {
	List(context.Context, bool) ([]Snapshot, error)
}

type SystemInspector struct{}

func (SystemInspector) List(ctx context.Context, withEnv bool) ([]Snapshot, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	snapshots := make([]Snapshot, 0, len(procs))
	for _, proc := range procs {
		args, err := proc.CmdlineSliceWithContext(ctx)
		if err != nil || len(args) == 0 {
			raw, rawErr := proc.CmdlineWithContext(ctx)
			if rawErr != nil || strings.TrimSpace(raw) == "" {
				continue
			}

			args = strings.Fields(raw)
		}

		raw, err := proc.CmdlineWithContext(ctx)
		if err != nil || strings.TrimSpace(raw) == "" {
			raw = strings.Join(args, " ")
		}

		exe, _ := proc.ExeWithContext(ctx)
		ppid, _ := proc.PpidWithContext(ctx)
		startedAt := time.Time{}
		if createdMS, err := proc.CreateTimeWithContext(ctx); err == nil {
			startedAt = time.UnixMilli(createdMS)
		}

		snapshot := Snapshot{
			PID:            proc.Pid,
			PPID:           ppid,
			Executable:     exe,
			RawCommandLine: raw,
			Args:           args,
			StartedAt:      startedAt,
		}

		if withEnv {
			snapshot.EnvHints = collectEnvHints(proc, ctx)
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func collectEnvHints(proc *process.Process, ctx context.Context) map[string]string {
	environ, err := proc.EnvironWithContext(ctx)
	if err != nil {
		return nil
	}

	allowed := map[string]struct{}{
		"CUDA_VISIBLE_DEVICES":         {},
		"NCCL_SOCKET_IFNAME":           {},
		"VLLM_HOST_IP":                 {},
		"VLLM_PORT":                    {},
		"VLLM_WORKER_MULTIPROC_METHOD": {},
		"VLLM_USE_V1":                  {},
	}

	hints := map[string]string{}
	for _, entry := range environ {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}

		if _, ok := allowed[parts[0]]; ok {
			hints[parts[0]] = parts[1]
		}
	}

	if len(hints) == 0 {
		return nil
	}

	return hints
}
