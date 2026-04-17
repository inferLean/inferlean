package pod

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/inferLean/inferlean-main/new-cli/internal/vllmdiscovery/shared"
)

type podList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name    string   `json:"name"`
				Command []string `json:"command"`
				Args    []string `json:"args"`
			} `json:"containers"`
		} `json:"spec"`
	} `json:"items"`
}

func Discover(ctx context.Context, podName, namespace string) ([]shared.Candidate, error) {
	if strings.TrimSpace(namespace) == "" {
		namespace = "default"
	}
	args := []string{"get", "pods", "-n", namespace, "-o", "json"}
	if strings.TrimSpace(podName) != "" {
		args = []string{"get", "pod", podName, "-n", namespace, "-o", "json"}
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	items := make([]shared.Candidate, 0)
	if strings.TrimSpace(podName) == "" {
		var list podList
		if err := json.Unmarshal(out, &list); err != nil {
			return nil, nil
		}
		for _, item := range list.Items {
			appendPod(&items, namespace, item.Metadata.Name, item.Spec.Containers)
		}
		return items, nil
	}
	var one struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name    string   `json:"name"`
				Command []string `json:"command"`
				Args    []string `json:"args"`
			} `json:"containers"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &one); err != nil {
		return nil, nil
	}
	appendPod(&items, namespace, one.Metadata.Name, one.Spec.Containers)
	return items, nil
}

func appendPod(items *[]shared.Candidate, namespace, podName string, containers []struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Args    []string `json:"args"`
}) {
	for _, container := range containers {
		cmd := strings.Join(append(container.Command, container.Args...), " ")
		if !shared.IsServeCommand(cmd) {
			continue
		}
		*items = append(*items, shared.Candidate{
			Source:         "pod",
			PodName:        podName,
			Namespace:      namespace,
			Executable:     "k8s-container:" + container.Name,
			RawCommandLine: cmd,
		})
	}
}
