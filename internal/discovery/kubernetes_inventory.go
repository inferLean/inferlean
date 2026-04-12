package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type kubernetesPod struct {
	Namespace     string
	Name          string
	ContainerName string
	ContainerID   string
	PodIP         string
	Containers    []kubernetesContainer
}

type kubernetesContainer struct {
	Name          string
	Image         string
	Command       []string
	Args          []string
	Env           map[string]string
	ConfigMapRefs []string
	Ports         []int
}

type kubernetesStatus struct {
	Name        string `json:"name"`
	ContainerID string `json:"containerID"`
}

type kubernetesContainerSpec struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
	Args    []string `json:"args"`
	Env     []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
	EnvFrom []struct {
		ConfigMapRef *struct {
			Name string `json:"name"`
		} `json:"configMapRef"`
	} `json:"envFrom"`
	Ports []struct {
		ContainerPort int    `json:"containerPort"`
		Protocol      string `json:"protocol"`
	} `json:"ports"`
}

func kubernetesInventoryNamespace(opts Options) string {
	if strings.TrimSpace(opts.Pod) == "" {
		return ""
	}
	_, namespace := normalizePodSelector(opts.Pod, opts.Namespace)
	return namespace
}

func loadKubernetesInventory(ctx context.Context, namespace string) (map[string]kubernetesPod, []kubernetesPod, error) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, nil, errors.New("kubectl was not found in PATH")
	}

	output, err := loadKubernetesPods(ctx, namespace)
	if err != nil {
		return nil, nil, err
	}

	return decodeKubernetesInventory(output)
}

func loadKubernetesPods(ctx context.Context, namespace string) ([]byte, error) {
	if strings.TrimSpace(namespace) != "" {
		output, err := exec.CommandContext(ctx, "kubectl", "get", "pods", "--namespace", namespace, "-o", "json").Output()
		if err != nil {
			return nil, fmt.Errorf("list kubernetes pods: %w", err)
		}
		return output, nil
	}

	output, err := exec.CommandContext(ctx, "kubectl", "get", "pods", "--all-namespaces", "-o", "json").Output()
	if err != nil {
		output, err = exec.CommandContext(ctx, "kubectl", "get", "pods", "-o", "json").Output()
		if err != nil {
			return nil, fmt.Errorf("list kubernetes pods: %w", err)
		}
	}
	return output, nil
}

func decodeKubernetesInventory(output []byte) (map[string]kubernetesPod, []kubernetesPod, error) {
	var payload struct {
		Items []struct {
			Metadata struct {
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"metadata"`
			Status struct {
				ContainerStatuses     []kubernetesStatus `json:"containerStatuses"`
				InitContainerStatuses []kubernetesStatus `json:"initContainerStatuses"`
				PodIP                 string             `json:"podIP"`
			} `json:"status"`
			Spec struct {
				Containers []kubernetesContainerSpec `json:"containers"`
			} `json:"spec"`
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
			Namespace:  item.Metadata.Namespace,
			Name:       item.Metadata.Name,
			PodIP:      item.Status.PodIP,
			Containers: kubernetesContainersFromSpec(item.Spec.Containers),
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
				PodIP:         base.PodIP,
				Containers:    base.Containers,
			}
		}
	}

	return index, pods, nil
}

func kubernetesContainersFromSpec(specs []kubernetesContainerSpec) []kubernetesContainer {
	containers := make([]kubernetesContainer, 0, len(specs))
	for _, spec := range specs {
		containers = append(containers, kubernetesContainerFromSpec(spec))
	}
	return containers
}

func kubernetesContainerFromSpec(spec kubernetesContainerSpec) kubernetesContainer {
	container := kubernetesContainer{
		Name:    spec.Name,
		Image:   spec.Image,
		Command: append([]string{}, spec.Command...),
		Args:    append([]string{}, spec.Args...),
		Env:     map[string]string{},
	}
	for _, env := range spec.Env {
		container.Env[env.Name] = env.Value
	}
	for _, envFrom := range spec.EnvFrom {
		if envFrom.ConfigMapRef != nil && strings.TrimSpace(envFrom.ConfigMapRef.Name) != "" {
			container.ConfigMapRefs = append(container.ConfigMapRefs, envFrom.ConfigMapRef.Name)
		}
	}
	for _, port := range spec.Ports {
		if port.ContainerPort > 0 && (port.Protocol == "" || strings.EqualFold(port.Protocol, "tcp")) {
			container.Ports = append(container.Ports, port.ContainerPort)
		}
	}
	return container
}
