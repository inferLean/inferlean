package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/inferLean/inferlean/internal/debug"
)

type metadataResolver interface {
	Enrich(context.Context, []CandidateGroup, Options) ([]CandidateGroup, runtimeInventory, error)
}

type runtimeInventory struct {
	Docker []dockerContainer
	Pods   []kubernetesPod
}

type kubernetesPod struct {
	Namespace     string
	Name          string
	ContainerName string
	ContainerID   string
}

type runtimeMetadataResolver struct{}

func (runtimeMetadataResolver) Enrich(ctx context.Context, groups []CandidateGroup, opts Options) ([]CandidateGroup, runtimeInventory, error) {
	if runtime.GOOS != "linux" {
		return groups, runtimeInventory{}, nil
	}

	containerIDs := map[int]string{}
	for idx := range groups {
		if groups[idx].PrimaryPID <= 0 {
			continue
		}
		containerID, err := containerIDForPID(groups[idx].PrimaryPID)
		if err != nil {
			debug.Debugf("skip target metadata for pid=%d: %v", groups[idx].PrimaryPID, err)
			continue
		}
		if containerID != "" {
			containerIDs[idx] = containerID
		}
	}

	needDocker := opts.Container != "" || len(containerIDs) > 0
	needKubernetes := opts.Pod != "" || len(containerIDs) > 0
	if !needDocker && !needKubernetes {
		return groups, runtimeInventory{}, nil
	}

	inventory := runtimeInventory{}
	dockerIndex := map[string]dockerContainer{}
	kubernetesIndex := map[string]kubernetesPod{}

	if needDocker {
		index, containers, err := loadDockerInventory(ctx)
		if err != nil {
			if opts.Container != "" {
				return groups, runtimeInventory{}, err
			}
			debug.Debugf("skip docker target metadata: %v", err)
		} else {
			dockerIndex = index
			inventory.Docker = containers
		}
	}

	if needKubernetes {
		index, pods, err := loadKubernetesInventory(ctx)
		if err != nil {
			if opts.Pod != "" {
				return groups, runtimeInventory{}, err
			}
			debug.Debugf("skip kubernetes target metadata: %v", err)
		} else {
			kubernetesIndex = index
			inventory.Pods = pods
		}
	}

	for idx := range groups {
		containerID := containerIDs[idx]
		switch {
		case containerID == "":
			groups[idx].Target = TargetRef{Kind: TargetKindHost}
		case hasMatch(kubernetesIndex, containerID):
			pod := matchKubernetesTarget(kubernetesIndex, containerID)
			groups[idx].Target = TargetRef{
				Kind:                    TargetKindKubernetes,
				KubernetesNamespace:     pod.Namespace,
				KubernetesPodName:       pod.Name,
				KubernetesContainerName: pod.ContainerName,
			}
		case hasMatch(dockerIndex, containerID):
			container := matchDockerTarget(dockerIndex, containerID)
			groups[idx].Target = TargetRef{
				Kind:                TargetKindDocker,
				DockerContainerID:   container.ID,
				DockerContainerName: container.Name,
			}
			applyDockerPortBinding(&groups[idx], container)
		default:
			groups[idx].Target = TargetRef{Kind: TargetKindHost}
		}
	}

	return groups, inventory, nil
}

func loadKubernetesInventory(ctx context.Context) (map[string]kubernetesPod, []kubernetesPod, error) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, nil, errors.New("kubectl was not found in PATH")
	}

	output, err := exec.CommandContext(ctx, "kubectl", "get", "pods", "--all-namespaces", "-o", "json").Output()
	if err != nil {
		output, err = exec.CommandContext(ctx, "kubectl", "get", "pods", "--namespace", "default", "-o", "json").Output()
		if err != nil {
			return nil, nil, fmt.Errorf("list kubernetes pods: %w", err)
		}
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"metadata"`
			Status struct {
				ContainerStatuses     []kubernetesStatus `json:"containerStatuses"`
				InitContainerStatuses []kubernetesStatus `json:"initContainerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode kubernetes pods: %w", err)
	}

	index := map[string]kubernetesPod{}
	pods := []kubernetesPod{}
	seenPods := map[string]struct{}{}
	for _, item := range payload.Items {
		base := kubernetesPod{
			Namespace: item.Metadata.Namespace,
			Name:      item.Metadata.Name,
		}
		key := base.Namespace + "/" + base.Name
		if _, ok := seenPods[key]; !ok {
			pods = append(pods, base)
			seenPods[key] = struct{}{}
		}
		for _, status := range append(item.Status.ContainerStatuses, item.Status.InitContainerStatuses...) {
			containerID := normalizeContainerID(status.ContainerID)
			if containerID == "" {
				continue
			}
			index[containerID] = kubernetesPod{
				Namespace:     base.Namespace,
				Name:          base.Name,
				ContainerName: status.Name,
				ContainerID:   containerID,
			}
		}
	}

	return index, pods, nil
}

type kubernetesStatus struct {
	Name        string `json:"name"`
	ContainerID string `json:"containerID"`
}

func (r runtimeInventory) findDocker(query string) (dockerContainer, bool) {
	for _, container := range r.Docker {
		if strings.EqualFold(container.Name, query) || strings.HasPrefix(container.ID, query) {
			return container, true
		}
	}
	return dockerContainer{}, false
}

func (r runtimeInventory) hasPod(name, namespace string) bool {
	for _, pod := range r.Pods {
		if strings.EqualFold(pod.Name, name) && strings.EqualFold(defaultNamespace(pod.Namespace), defaultNamespace(namespace)) {
			return true
		}
	}
	return false
}

func matchDockerTarget(index map[string]dockerContainer, containerID string) dockerContainer {
	for id, container := range index {
		if sameContainerID(id, containerID) {
			return container
		}
	}
	return dockerContainer{}
}

func matchKubernetesTarget(index map[string]kubernetesPod, containerID string) kubernetesPod {
	for id, pod := range index {
		if sameContainerID(id, containerID) {
			return pod
		}
	}
	return kubernetesPod{}
}

func hasMatch[T any](index map[string]T, containerID string) bool {
	for id := range index {
		if sameContainerID(id, containerID) {
			return true
		}
	}
	return false
}

func sameContainerID(left, right string) bool {
	left = normalizeContainerID(left)
	right = normalizeContainerID(right)
	return left != "" && right != "" && (left == right || strings.HasPrefix(left, right) || strings.HasPrefix(right, left))
}

func normalizeContainerID(value string) string {
	value = strings.TrimSpace(value)
	if _, remainder, ok := strings.Cut(value, "://"); ok {
		return remainder
	}
	return value
}
