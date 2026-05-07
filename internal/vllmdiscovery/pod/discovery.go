package pod

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

type podList struct {
	Items []podItem `json:"items"`
}

type podItem struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Containers []podContainer `json:"containers"`
	} `json:"spec"`
}

type podContainer struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Args    []string `json:"args"`
	Env     []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
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
	var one podItem
	if err := json.Unmarshal(out, &one); err != nil {
		return nil, nil
	}
	appendPod(&items, namespace, one.Metadata.Name, one.Spec.Containers)
	return items, nil
}

func appendPod(items *[]shared.Candidate, namespace, podName string, containers []podContainer) {
	for _, container := range containers {
		cmd := strings.Join(append(container.Command, container.Args...), " ")
		if !shared.IsServeCommand(cmd) {
			continue
		}
		port := shared.InferMetricsPort(cmd, podEnv(container))
		*items = append(*items, shared.Candidate{
			Source:          "pod",
			PodName:         podName,
			Namespace:       namespace,
			Executable:      "k8s-container:" + container.Name,
			RawCommandLine:  cmd,
			MetricsEndpoint: shared.MetricsEndpoint("127.0.0.1", port),
		})
	}
}

func podEnv(container podContainer) []string {
	env := make([]string, 0, len(container.Env))
	for _, item := range container.Env {
		env = append(env, item.Name+"="+item.Value)
	}
	return env
}
