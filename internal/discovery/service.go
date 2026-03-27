package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/inferLean/inferlean/internal/debug"
	"github.com/inferLean/inferlean/internal/discovery/parse"
	"github.com/inferLean/inferlean/internal/discovery/process"
	selection "github.com/inferLean/inferlean/internal/discovery/select"
)

type Service struct {
	inspector process.Inspector
}

func NewService(inspector process.Inspector) Service {
	return Service{inspector: inspector}
}

func (s Service) Discover(ctx context.Context, opts Options) (Result, error) {
	emitStep(opts.Stepf, StepEnumerate, "Enumerating local processes")
	debug.Debugf("starting process enumeration")
	snapshots, err := s.inspector.List(ctx, opts.WithEnv)
	if err != nil {
		return Result{}, err
	}
	debug.Debugf("enumerated %d processes", len(snapshots))

	emitStep(opts.Stepf, StepParse, "Parsing vLLM runtime configuration")
	matched := make([]selection.CandidateProcess, 0)
	pidExists := false
	for _, snapshot := range snapshots {
		if opts.PID > 0 && snapshot.PID == opts.PID {
			pidExists = true
		}

		parsed := parse.Parse(snapshot)
		if !parsed.Matched {
			continue
		}

		debug.Debugf("matched candidate pid=%d entrypoint=%s model=%s port=%d", snapshot.PID, parsed.EntryPoint, parsed.RuntimeConfig.Model, parsed.RuntimeConfig.Port)
		matched = append(matched, selection.CandidateProcess{
			PID:            snapshot.PID,
			PPID:           snapshot.PPID,
			Executable:     snapshot.Executable,
			RawCommandLine: snapshot.RawCommandLine,
			EntryPoint:     parsed.EntryPoint,
			Signature:      parsed.Signature,
			RuntimeConfig: selection.RuntimeConfig{
				Model:                 parsed.RuntimeConfig.Model,
				ServedModelName:       parsed.RuntimeConfig.ServedModelName,
				Host:                  parsed.RuntimeConfig.Host,
				Port:                  parsed.RuntimeConfig.Port,
				TensorParallelSize:    parsed.RuntimeConfig.TensorParallelSize,
				DataParallelSize:      parsed.RuntimeConfig.DataParallelSize,
				PipelineParallelSize:  parsed.RuntimeConfig.PipelineParallelSize,
				MaxModelLen:           parsed.RuntimeConfig.MaxModelLen,
				MaxNumBatchedTokens:   parsed.RuntimeConfig.MaxNumBatchedTokens,
				MaxNumSeqs:            parsed.RuntimeConfig.MaxNumSeqs,
				GPUMemoryUtilization:  parsed.RuntimeConfig.GPUMemoryUtilization,
				KVCacheDType:          parsed.RuntimeConfig.KVCacheDType,
				ChunkedPrefill:        parsed.RuntimeConfig.ChunkedPrefill,
				PrefixCaching:         parsed.RuntimeConfig.PrefixCaching,
				Quantization:          parsed.RuntimeConfig.Quantization,
				DType:                 parsed.RuntimeConfig.DType,
				GenerationConfig:      parsed.RuntimeConfig.GenerationConfig,
				APIKeyConfigured:      parsed.RuntimeConfig.APIKeyConfigured,
				MultimodalFlags:       parsed.RuntimeConfig.MultimodalFlags,
				AttentionBackend:      parsed.RuntimeConfig.AttentionBackend,
				FlashinferPresent:     parsed.RuntimeConfig.FlashinferPresent,
				FlashAttentionPresent: parsed.RuntimeConfig.FlashAttentionPresent,
				ImageProcessor:        parsed.RuntimeConfig.ImageProcessor,
				MultimodalCacheHints:  parsed.RuntimeConfig.MultimodalCacheHints,
				EnvHints:              parsed.RuntimeConfig.EnvHints,
			},
			ParseWarnings: parsed.Warnings,
		})
	}

	if opts.PID > 0 && !pidExists {
		return Result{}, fmt.Errorf("%w: %d", ErrPIDNotFound, opts.PID)
	}

	emitStep(opts.Stepf, StepResolve, "Resolving the target deployment")
	groupModels := selection.BuildGroups(matched)
	groups := make([]CandidateGroup, 0, len(groupModels))
	for _, group := range groupModels {
		groups = append(groups, CandidateGroup{
			Key:            group.Key,
			ProcessCount:   group.ProcessCount,
			PrimaryPID:     group.PrimaryPID,
			PIDs:           group.PIDs,
			EntryPoint:     group.EntryPoint,
			Executable:     group.Executable,
			ParentPID:      group.ParentPID,
			RawCommandLine: group.RawCommandLine,
			CommandExcerpt: group.CommandExcerpt,
			RuntimeConfig: RuntimeConfig{
				Model:                 group.RuntimeConfig.Model,
				ServedModelName:       group.RuntimeConfig.ServedModelName,
				Host:                  group.RuntimeConfig.Host,
				Port:                  group.RuntimeConfig.Port,
				TensorParallelSize:    group.RuntimeConfig.TensorParallelSize,
				DataParallelSize:      group.RuntimeConfig.DataParallelSize,
				PipelineParallelSize:  group.RuntimeConfig.PipelineParallelSize,
				MaxModelLen:           group.RuntimeConfig.MaxModelLen,
				MaxNumBatchedTokens:   group.RuntimeConfig.MaxNumBatchedTokens,
				MaxNumSeqs:            group.RuntimeConfig.MaxNumSeqs,
				GPUMemoryUtilization:  group.RuntimeConfig.GPUMemoryUtilization,
				KVCacheDType:          group.RuntimeConfig.KVCacheDType,
				ChunkedPrefill:        group.RuntimeConfig.ChunkedPrefill,
				PrefixCaching:         group.RuntimeConfig.PrefixCaching,
				Quantization:          group.RuntimeConfig.Quantization,
				DType:                 group.RuntimeConfig.DType,
				GenerationConfig:      group.RuntimeConfig.GenerationConfig,
				APIKeyConfigured:      group.RuntimeConfig.APIKeyConfigured,
				MultimodalFlags:       group.RuntimeConfig.MultimodalFlags,
				AttentionBackend:      group.RuntimeConfig.AttentionBackend,
				FlashinferPresent:     group.RuntimeConfig.FlashinferPresent,
				FlashAttentionPresent: group.RuntimeConfig.FlashAttentionPresent,
				ImageProcessor:        group.RuntimeConfig.ImageProcessor,
				MultimodalCacheHints:  group.RuntimeConfig.MultimodalCacheHints,
				EnvHints:              group.RuntimeConfig.EnvHints,
			},
			ParseWarnings: group.ParseWarnings,
		})
	}
	debug.Debugf("grouped %d processes into %d logical candidates", len(matched), len(groups))

	if len(groups) == 0 {
		if opts.PID > 0 {
			return Result{}, fmt.Errorf("%w: %d", ErrPIDNotVLLM, opts.PID)
		}
		return Result{}, ErrNoCandidates
	}

	if opts.PID > 0 {
		selected := findGroupByPID(groups, opts.PID)
		if selected == nil {
			return Result{Candidates: groups}, fmt.Errorf("%w: %d", ErrPIDNotVLLM, opts.PID)
		}

		debug.Debugf("selected candidate via explicit pid=%d group=%s", opts.PID, selected.Key)
		return Result{
			Selected:   selected,
			Candidates: groups,
			Reason:     fmt.Sprintf("selected explicitly because --pid=%d was provided", opts.PID),
		}, nil
	}

	if len(groups) == 1 {
		debug.Debugf("selected only detected candidate pid=%d", groups[0].PrimaryPID)
		return Result{
			Selected:   &groups[0],
			Candidates: groups,
			Reason:     "selected automatically because exactly one vLLM deployment was found",
		}, nil
	}

	var candidateSummary []string
	for _, group := range groups {
		candidateSummary = append(candidateSummary, fmt.Sprintf("pid=%d model=%s port=%d", group.PrimaryPID, group.DisplayModel(), group.RuntimeConfig.Port))
	}
	debug.Debugf("ambiguity detected: %s", strings.Join(candidateSummary, "; "))

	return Result{
		Candidates: groups,
		Warnings:   []string{"multiple vLLM deployments were found"},
	}, ErrAmbiguous
}

func emitStep(stepf func(StepUpdate), step Step, message string) {
	if stepf == nil {
		return
	}

	stepf(StepUpdate{
		Step:    step,
		Message: message,
	})
}

func findGroupByPID(groups []CandidateGroup, pid int32) *CandidateGroup {
	for idx := range groups {
		for _, memberPID := range groups[idx].PIDs {
			if memberPID == pid {
				return &groups[idx]
			}
		}
	}

	return nil
}
