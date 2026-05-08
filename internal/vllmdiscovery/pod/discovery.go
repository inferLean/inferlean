package pod

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

type podList struct {
	Items []podItem `json:"items"`
}

type podItem struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Containers []podContainer `json:"containers"`
	} `json:"spec"`
}

type podContainer struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
	Args    []string `json:"args"`
	Env     []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
}

func Discover(ctx context.Context, podName, namespace string) ([]shared.Candidate, error) {
	podName = strings.TrimSpace(podName)
	namespace = strings.TrimSpace(namespace)
	args := kubectlGetArgs(podName, namespace)
	out, err := exec.CommandContext(ctx, "kubectl", args...).CombinedOutput()
	if err != nil {
		if podName != "" {
			return nil, commandError("get kubernetes pod "+podName, err, out)
		}
		return nil, nil
	}
	items := make([]shared.Candidate, 0)
	if podName == "" {
		var list podList
		if err := json.Unmarshal(out, &list); err != nil {
			return nil, nil
		}
		for _, item := range list.Items {
			appendPod(ctx, &items, podNamespace(namespace, item.Metadata.Namespace), item.Metadata.Name, item.Spec.Containers)
		}
		return items, nil
	}
	var one podItem
	if err := json.Unmarshal(out, &one); err != nil {
		return nil, fmt.Errorf("parse kubernetes pod %s: %w", podName, err)
	}
	appendPod(ctx, &items, podNamespace(namespace, one.Metadata.Namespace), one.Metadata.Name, one.Spec.Containers)
	return items, nil
}

func appendPod(ctx context.Context, items *[]shared.Candidate, namespace, podName string, containers []podContainer) {
	for _, container := range containers {
		cmd := strings.Join(append(container.Command, container.Args...), " ")
		if !shared.IsServeCommand(cmd) && !shared.IsVLLMImage(container.Image) {
			continue
		}
		port := shared.InferMetricsPort(cmd, podEnv(container))
		*items = append(*items, shared.Candidate{
			Source:          "pod",
			InternalPID:     detectInternalPID(ctx, namespace, podName, container.Name),
			PodName:         podName,
			Namespace:       namespace,
			Executable:      "k8s-container:" + container.Name,
			RawCommandLine:  cmd,
			MetricsEndpoint: shared.MetricsEndpoint("127.0.0.1", port),
		})
	}
}

func detectInternalPID(ctx context.Context, namespace, podName, container string) int32 {
	out, err := exec.CommandContext(
		ctx,
		"kubectl",
		kubectlExecArgs(namespace, podName, container, "sh", "-c", shared.ProcListScript)...,
	).Output()
	if err != nil {
		return 0
	}
	return shared.FirstVLLMProcessPID(shared.ParseProcList(string(out)))
}

func kubectlGetArgs(podName, namespace string) []string {
	args := []string{"get"}
	if podName == "" {
		args = append(args, "pods")
	} else {
		args = append(args, "pod", podName)
	}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	return append(args, "-o", "json")
}

func podNamespace(requested, observed string) string {
	if namespace := strings.TrimSpace(observed); namespace != "" {
		return namespace
	}
	if namespace := strings.TrimSpace(requested); namespace != "" {
		return namespace
	}
	return "default"
}

func commandError(action string, err error, output []byte) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, message)
}

func kubectlExecArgs(namespace, podName, container string, command ...string) []string {
	args := []string{"exec"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, podName)
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--")
	args = append(args, command...)
	return args
}

func podEnv(container podContainer) []string {
	env := make([]string, 0, len(container.Env))
	for _, item := range container.Env {
		env = append(env, item.Name+"="+item.Value)
	}
	return env
}
