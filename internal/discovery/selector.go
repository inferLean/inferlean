package discovery

import (
	"errors"
	"fmt"
	"strings"
)

func validateSelection(opts Options) error {
	selectionCount := 0
	if opts.PID > 0 {
		selectionCount++
	}
	if strings.TrimSpace(opts.Container) != "" {
		selectionCount++
	}
	if strings.TrimSpace(opts.Pod) != "" {
		selectionCount++
	}
	if selectionCount > 1 {
		return errors.New("specify only one of --pid, --container, or --pod")
	}
	if strings.TrimSpace(opts.Namespace) != "" && strings.TrimSpace(opts.Pod) == "" {
		return errors.New("--namespace can only be used with --pod")
	}
	return nil
}

func explicitSelection(groups []CandidateGroup, inventory runtimeInventory, opts Options) ([]CandidateGroup, string, bool, error) {
	switch {
	case opts.PID > 0:
		selected := findGroupByPID(groups, opts.PID)
		if selected == nil {
			return nil, "", true, fmt.Errorf("%w: %d", ErrPIDNotVLLM, opts.PID)
		}
		return []CandidateGroup{*selected}, fmt.Sprintf("selected explicitly because --pid=%d was provided", opts.PID), true, nil
	case strings.TrimSpace(opts.Container) != "":
		container, ok := inventory.findDocker(strings.TrimSpace(opts.Container))
		if !ok {
			return nil, "", true, fmt.Errorf("%w: %s", ErrContainerNotFound, opts.Container)
		}
		matched := filterCandidateGroups(groups, func(group CandidateGroup) bool {
			return group.Target.MatchesContainer(container.Name) || group.Target.MatchesContainer(container.ID)
		})
		if len(matched) == 0 {
			return nil, "", true, fmt.Errorf("%w: %s", ErrContainerNotVLLM, opts.Container)
		}
		return matched, fmt.Sprintf("selected explicitly because --container=%s was provided", opts.Container), true, nil
	case strings.TrimSpace(opts.Pod) != "":
		podName, namespace := normalizePodSelector(opts.Pod, opts.Namespace)
		if !inventory.hasPod(podName, namespace) {
			return nil, "", true, fmt.Errorf("%w: %s/%s", ErrPodNotFound, namespace, podName)
		}
		matched := filterCandidateGroups(groups, func(group CandidateGroup) bool {
			return group.Target.MatchesPod(podName, namespace)
		})
		if len(matched) == 0 {
			return nil, "", true, fmt.Errorf("%w: %s/%s", ErrPodNotVLLM, namespace, podName)
		}
		return matched, fmt.Sprintf("selected explicitly because --pod=%s and --namespace=%s were provided", podName, namespace), true, nil
	default:
		return nil, "", false, nil
	}
}

func filterCandidateGroups(groups []CandidateGroup, keep func(CandidateGroup) bool) []CandidateGroup {
	filtered := make([]CandidateGroup, 0, len(groups))
	for _, group := range groups {
		if keep(group) {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func normalizePodSelector(pod, namespace string) (string, string) {
	pod = strings.TrimSpace(pod)
	namespace = strings.TrimSpace(namespace)
	if left, right, ok := strings.Cut(pod, "/"); ok {
		if namespace == "" {
			return strings.TrimSpace(right), defaultNamespace(left)
		}
		return strings.TrimSpace(right), defaultNamespace(namespace)
	}
	return pod, defaultNamespace(namespace)
}
