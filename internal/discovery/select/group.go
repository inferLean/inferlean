package selection

import (
	"fmt"
	"sort"
	"strings"
)

type CandidateProcess struct {
	PID            int32
	PPID           int32
	Executable     string
	RawCommandLine string
	EntryPoint     string
	Signature      string
	RuntimeConfig  RuntimeConfig
	ParseWarnings  []string
}

type Group struct {
	Key            string
	ProcessCount   int
	PrimaryPID     int32
	PIDs           []int32
	EntryPoint     string
	Executable     string
	ParentPID      int32
	RawCommandLine string
	CommandExcerpt string
	RuntimeConfig  RuntimeConfig
	ParseWarnings  []string
}

type RuntimeConfig struct {
	Model                 string
	ServedModelName       string
	Host                  string
	Port                  int
	PortDefaulted         bool
	TensorParallelSize    int
	DataParallelSize      int
	PipelineParallelSize  int
	MaxModelLen           int
	MaxNumBatchedTokens   int
	MaxNumSeqs            int
	GPUMemoryUtilization  float64
	KVCacheDType          string
	ChunkedPrefill        *bool
	PrefixCaching         *bool
	Quantization          string
	DType                 string
	GenerationConfig      string
	APIKeyConfigured      bool
	MultimodalFlags       []string
	AttentionBackend      string
	FlashinferPresent     *bool
	FlashAttentionPresent *bool
	ImageProcessor        string
	MultimodalCacheHints  []string
	EnvHints              map[string]string
}

func BuildGroups(processes []CandidateProcess) []Group {
	byPID := map[int32]CandidateProcess{}
	for _, proc := range processes {
		byPID[proc.PID] = proc
	}

	grouped := map[string][]CandidateProcess{}
	for _, proc := range processes {
		key := keyFor(proc, byPID)
		grouped[key] = append(grouped[key], proc)
	}

	groups := make([]Group, 0, len(grouped))
	for key, members := range grouped {
		sort.Slice(members, func(i, j int) bool {
			return members[i].PID < members[j].PID
		})

		primary := choosePrimary(members, byPID)
		pids := make([]int32, 0, len(members))
		warnings := []string{}
		seenWarnings := map[string]struct{}{}
		for _, member := range members {
			pids = append(pids, member.PID)
			for _, warning := range member.ParseWarnings {
				if _, ok := seenWarnings[warning]; ok {
					continue
				}
				warnings = append(warnings, warning)
				seenWarnings[warning] = struct{}{}
			}
		}

		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })

		groups = append(groups, Group{
			Key:            key,
			ProcessCount:   len(members),
			PrimaryPID:     primary.PID,
			PIDs:           pids,
			EntryPoint:     primary.EntryPoint,
			Executable:     primary.Executable,
			ParentPID:      primary.PPID,
			RawCommandLine: primary.RawCommandLine,
			CommandExcerpt: excerpt(primary.RawCommandLine),
			RuntimeConfig:  primary.RuntimeConfig,
			ParseWarnings:  warnings,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].RuntimeConfig.Port != groups[j].RuntimeConfig.Port {
			return groups[i].RuntimeConfig.Port < groups[j].RuntimeConfig.Port
		}
		return groups[i].PrimaryPID < groups[j].PrimaryPID
	})

	return groups
}

func FindByPID(groups []Group, pid int32) *Group {
	for idx := range groups {
		for _, memberPID := range groups[idx].PIDs {
			if memberPID == pid {
				return &groups[idx]
			}
		}
	}

	return nil
}

func keyFor(proc CandidateProcess, byPID map[int32]CandidateProcess) string {
	if root := rootPID(proc, byPID); root != proc.PID || hasChildCandidate(proc.PID, byPID) {
		return fmt.Sprintf("root:%d", root)
	}

	if proc.RuntimeConfig.Port > 0 {
		host := proc.RuntimeConfig.Host
		if host == "" {
			host = "0.0.0.0"
		}
		return fmt.Sprintf("listen:%s:%d:%s", host, proc.RuntimeConfig.Port, proc.RuntimeConfig.Model)
	}

	return fmt.Sprintf("argv:%s", proc.Signature)
}

func hasChildCandidate(pid int32, byPID map[int32]CandidateProcess) bool {
	for _, candidate := range byPID {
		if candidate.PPID == pid {
			return true
		}
	}

	return false
}

func rootPID(proc CandidateProcess, byPID map[int32]CandidateProcess) int32 {
	root := proc.PID
	current := proc
	for {
		parent, ok := byPID[current.PPID]
		if !ok {
			return root
		}
		root = parent.PID
		current = parent
	}
}

func choosePrimary(members []CandidateProcess, byPID map[int32]CandidateProcess) CandidateProcess {
	if len(members) == 1 {
		return members[0]
	}

	primary := members[0]
	root := rootPID(primary, byPID)
	for _, member := range members {
		if member.PID == root {
			return member
		}
		if member.PID < primary.PID {
			primary = member
		}
	}

	return primary
}

func excerpt(raw string) string {
	const maxLen = 120
	clean := strings.TrimSpace(raw)
	if len(clean) <= maxLen {
		return clean
	}

	return clean[:maxLen-1] + "…"
}
