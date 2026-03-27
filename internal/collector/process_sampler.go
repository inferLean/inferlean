package collector

import (
	"context"
	"fmt"
	"runtime"
	"time"

	gopsprocess "github.com/shirou/gopsutil/v4/process"
)

func collectProcessSamples(ctx context.Context, pids []int32, interval time.Duration, rawPath string) ([]processSample, error) {
	if len(pids) == 0 {
		return nil, nil
	}

	snapshots, err := runProcessSampler(ctx, pids, interval)
	if err != nil {
		return nil, err
	}
	if err := writeJSONFile(rawPath, snapshots); err != nil {
		return nil, fmt.Errorf("write process samples: %w", err)
	}
	return snapshots, nil
}

func runProcessSampler(ctx context.Context, pids []int32, interval time.Duration) ([]processSample, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastCPU, lastAt, _ := snapshotProcessGroup(ctx, pids)
	samples := []processSample{}
	for {
		select {
		case <-ctx.Done():
			return samples, nil
		case <-ticker.C:
			totalCPU, at, rss := snapshotProcessGroup(ctx, pids)
			cpuPercent := processCPUPercent(totalCPU-lastCPU, at.Sub(lastAt))
			lastCPU = totalCPU
			lastAt = at
			samples = append(samples, processSample{Timestamp: at, CPUPercent: cpuPercent, RSSBytes: rss})
		}
	}
}

func snapshotProcessGroup(ctx context.Context, pids []int32) (float64, time.Time, float64) {
	totalCPU := 0.0
	totalRSS := uint64(0)
	for _, pid := range pids {
		proc, err := gopsprocess.NewProcess(pid)
		if err != nil {
			continue
		}
		totalCPU += processCPUSeconds(ctx, proc)
		totalRSS += processRSS(ctx, proc)
	}
	return totalCPU, time.Now().UTC(), float64(totalRSS)
}

func processCPUSeconds(ctx context.Context, proc *gopsprocess.Process) float64 {
	times, err := proc.TimesWithContext(ctx)
	if err != nil {
		return 0
	}
	return times.User + times.System
}

func processRSS(ctx context.Context, proc *gopsprocess.Process) uint64 {
	info, err := proc.MemoryInfoWithContext(ctx)
	if err != nil || info == nil {
		return 0
	}
	return info.RSS
}

func processCPUPercent(delta float64, elapsed time.Duration) float64 {
	if delta <= 0 || elapsed <= 0 {
		return 0
	}
	return 100 * delta / elapsed.Seconds() / float64(runtime.NumCPU())
}
