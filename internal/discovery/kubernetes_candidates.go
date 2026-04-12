package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/inferLean/inferlean/internal/discovery/parse"
	"github.com/inferLean/inferlean/internal/discovery/process"
)

var kubernetesEnvRefPattern = regexp.MustCompile(`\$\(([A-Za-z_][A-Za-z0-9_]*)\)`)

func kubernetesCandidateGroups(ctx context.Context, pods []kubernetesPod, existing []CandidateGroup) []CandidateGroup {
	configMaps := map[string]map[string]string{}
	candidates := []CandidateGroup{}
	for _, pod := range pods {
		if hasKubernetesCandidate(existing, pod.Namespace, pod.Name) {
			continue
		}
		for _, container := range pod.Containers {
			if !isVLLMImage(container.Image) {
				continue
			}
			candidates = append(candidates, kubernetesCandidateGroup(ctx, pod, container, configMaps))
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Key < candidates[j].Key
	})
	return candidates
}

func kubernetesCandidateGroup(ctx context.Context, pod kubernetesPod, container kubernetesContainer, configMaps map[string]map[string]string) CandidateGroup {
	env := kubernetesContainerEnv(ctx, pod.Namespace, container, configMaps)
	args := resolveKubernetesArgs(kubernetesCommandArgs(container), env)
	parsed := parse.Parse(process.Snapshot{Args: args})
	runtimeConfig := runtimeConfigFromParsed(parsed.RuntimeConfig)
	if runtimeConfig.Port == 0 {
		runtimeConfig.Port = firstKubernetesPort(container.Ports)
	}
	if runtimeConfig.Host == "" {
		runtimeConfig.Host = pod.PodIP
	}

	raw := strings.Join(args, " ")
	return CandidateGroup{
		Key:            fmt.Sprintf("kubernetes:%s/%s/%s", pod.Namespace, pod.Name, container.Name),
		Target:         kubernetesTarget(pod, container),
		EntryPoint:     parsed.EntryPoint,
		RawCommandLine: raw,
		CommandExcerpt: commandExcerpt(raw),
		RuntimeConfig:  runtimeConfig,
		ParseWarnings:  parsed.Warnings,
	}
}

func kubernetesTarget(pod kubernetesPod, container kubernetesContainer) TargetRef {
	return TargetRef{
		Kind:                    TargetKindKubernetes,
		KubernetesNamespace:     pod.Namespace,
		KubernetesPodName:       pod.Name,
		KubernetesContainerName: container.Name,
	}
}

func kubernetesCommandArgs(container kubernetesContainer) []string {
	if len(container.Command) > 0 {
		return append(append([]string{}, container.Command...), container.Args...)
	}
	return append([]string{"vllm", "serve"}, container.Args...)
}

func kubernetesContainerEnv(ctx context.Context, namespace string, container kubernetesContainer, cache map[string]map[string]string) map[string]string {
	env := map[string]string{}
	for _, name := range container.ConfigMapRefs {
		for key, value := range loadKubernetesConfigMap(ctx, namespace, name, cache) {
			env[key] = value
		}
	}
	for key, value := range container.Env {
		env[key] = value
	}
	return env
}

func loadKubernetesConfigMap(ctx context.Context, namespace, name string, cache map[string]map[string]string) map[string]string {
	key := namespace + "/" + name
	if values, ok := cache[key]; ok {
		return values
	}
	values := map[string]string{}
	output, err := exec.CommandContext(ctx, "kubectl", "get", "configmap", name, "--namespace", namespace, "-o", "json").Output()
	if err == nil {
		var payload struct {
			Data map[string]string `json:"data"`
		}
		if json.Unmarshal(output, &payload) == nil {
			values = payload.Data
		}
	}
	cache[key] = values
	return values
}

func resolveKubernetesArgs(args []string, env map[string]string) []string {
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		resolved = append(resolved, kubernetesEnvRefPattern.ReplaceAllStringFunc(arg, func(match string) string {
			key := strings.TrimSuffix(strings.TrimPrefix(match, "$("), ")")
			if value, ok := env[key]; ok {
				return value
			}
			return match
		}))
	}
	return resolved
}

func runtimeConfigFromParsed(cfg parse.RuntimeConfig) RuntimeConfig {
	return RuntimeConfig{
		Model:                 cfg.Model,
		ServedModelName:       cfg.ServedModelName,
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		TensorParallelSize:    cfg.TensorParallelSize,
		DataParallelSize:      cfg.DataParallelSize,
		PipelineParallelSize:  cfg.PipelineParallelSize,
		MaxModelLen:           cfg.MaxModelLen,
		MaxNumBatchedTokens:   cfg.MaxNumBatchedTokens,
		MaxNumSeqs:            cfg.MaxNumSeqs,
		GPUMemoryUtilization:  cfg.GPUMemoryUtilization,
		KVCacheDType:          cfg.KVCacheDType,
		ChunkedPrefill:        cfg.ChunkedPrefill,
		PrefixCaching:         cfg.PrefixCaching,
		Quantization:          cfg.Quantization,
		DType:                 cfg.DType,
		GenerationConfig:      cfg.GenerationConfig,
		APIKeyConfigured:      cfg.APIKeyConfigured,
		MultimodalFlags:       cfg.MultimodalFlags,
		AttentionBackend:      cfg.AttentionBackend,
		FlashinferPresent:     cfg.FlashinferPresent,
		FlashAttentionPresent: cfg.FlashAttentionPresent,
		ImageProcessor:        cfg.ImageProcessor,
		MultimodalCacheHints:  cfg.MultimodalCacheHints,
		EnvHints:              cfg.EnvHints,
	}
}

func firstKubernetesPort(ports []int) int {
	for _, port := range ports {
		if port == 8000 {
			return port
		}
	}
	if len(ports) == 0 {
		return 0
	}
	return ports[0]
}

func hasKubernetesCandidate(groups []CandidateGroup, namespace, name string) bool {
	for _, group := range groups {
		if group.Target.MatchesPod(name, namespace) {
			return true
		}
	}
	return false
}

func isVLLMImage(image string) bool {
	return strings.Contains(strings.ToLower(image), "vllm")
}

func commandExcerpt(raw string) string {
	const maxLen = 120
	clean := strings.TrimSpace(raw)
	if len(clean) <= maxLen {
		return clean
	}
	return clean[:maxLen-1] + "..."
}
