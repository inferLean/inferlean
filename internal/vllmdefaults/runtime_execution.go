package vllmdefaults

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

const (
	remoteScriptPath    = "/tmp/inferlean-dump-vllm-defaults.py"
	remoteDumpPath      = "/tmp/inferlean-vllm-defaults.json"
	defaultPodNamespace = "default"
)

const pidProbeScript = "for p in /proc/[0-9]*; do cmd=$(tr '\\000' ' ' < \"$p/cmdline\" 2>/dev/null); case \"$cmd\" in *vllm*serve*) echo \"${p##*/}\"; exit 0;; esac; done; echo 1"

func runDumpScript(ctx context.Context, target shared.Candidate, scriptPath, dumpPath string) (runtimeExecution, error) {
	source := strings.ToLower(strings.TrimSpace(target.Source))
	switch source {
	case "docker":
		return runDumpInDocker(ctx, target, scriptPath, dumpPath)
	case "pod":
		return runDumpInPod(ctx, target, scriptPath, dumpPath)
	default:
		return runDumpOnHost(ctx, target, scriptPath, dumpPath)
	}
}

func runDumpOnHost(ctx context.Context, target shared.Candidate, scriptPath, dumpPath string) (runtimeExecution, error) {
	if target.PID <= 0 {
		return runtimeExecution{}, fmt.Errorf("cannot run defaults script on host without a target pid")
	}
	if err := runPythonLocal(ctx, scriptPath, target.PID, dumpPath); err != nil {
		return runtimeExecution{}, err
	}
	return runtimeExecution{Source: "process", PID: target.PID}, nil
}

func runDumpInDocker(ctx context.Context, target shared.Candidate, scriptPath, dumpPath string) (runtimeExecution, error) {
	containerID := strings.TrimSpace(target.ContainerID)
	if containerID == "" {
		return runtimeExecution{}, fmt.Errorf("cannot run defaults script for docker target without container id")
	}

	if _, err := runCommand(ctx, "docker", "cp", scriptPath, containerID+":"+remoteScriptPath); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults script into container: %w", err)
	}

	pid := detectDockerPID(ctx, containerID)
	if err := runPythonInDocker(ctx, containerID, pid); err != nil {
		return runtimeExecution{}, err
	}
	if _, err := runCommand(ctx, "docker", "cp", containerID+":"+remoteDumpPath, dumpPath); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults dump from container: %w", err)
	}
	_, _ = runCommand(ctx, "docker", "exec", containerID, "rm", "-f", remoteScriptPath, remoteDumpPath)
	return runtimeExecution{Source: "docker", PID: pid}, nil
}

func detectDockerPID(ctx context.Context, containerID string) int32 {
	out, err := runCommand(ctx, "docker", "exec", containerID, "sh", "-c", pidProbeScript)
	if err != nil {
		return 1
	}
	pid := parsePositiveInt(string(out))
	if pid <= 0 {
		return 1
	}
	return int32(pid)
}

func runPythonInDocker(ctx context.Context, containerID string, pid int32) error {
	var lastErr error
	for _, py := range []string{"python3", "python"} {
		_, err := runCommand(
			ctx,
			"docker",
			"exec",
			containerID,
			py,
			remoteScriptPath,
			"--pid",
			strconv.Itoa(int(pid)),
			"--out",
			remoteDumpPath,
		)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("execute defaults script in container: %w", lastErr)
}

func runDumpInPod(ctx context.Context, target shared.Candidate, scriptPath, dumpPath string) (runtimeExecution, error) {
	podName := strings.TrimSpace(target.PodName)
	if podName == "" {
		return runtimeExecution{}, fmt.Errorf("cannot run defaults script for pod target without pod name")
	}
	namespace := strings.TrimSpace(target.Namespace)
	if namespace == "" {
		namespace = defaultPodNamespace
	}
	container := podContainerName(target.Executable)

	if _, err := runCommand(ctx, "kubectl", kubectlCopyToArgs(namespace, podName, container, scriptPath, remoteScriptPath)...); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults script into pod: %w", err)
	}

	pid := detectPodPID(ctx, namespace, podName, container)
	if err := runPythonInPod(ctx, namespace, podName, container, pid); err != nil {
		return runtimeExecution{}, err
	}
	if _, err := runCommand(ctx, "kubectl", kubectlCopyFromArgs(namespace, podName, container, remoteDumpPath, dumpPath)...); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults dump from pod: %w", err)
	}

	_, _ = runCommand(ctx, "kubectl", kubectlExecArgs(namespace, podName, container, "rm", "-f", remoteScriptPath, remoteDumpPath)...)
	return runtimeExecution{Source: "pod", PID: pid}, nil
}

func podContainerName(executable string) string {
	const prefix = "k8s-container:"
	trimmed := strings.TrimSpace(executable)
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
}

func detectPodPID(ctx context.Context, namespace, podName, container string) int32 {
	args := kubectlExecArgs(namespace, podName, container, "sh", "-c", pidProbeScript)
	out, err := runCommand(ctx, "kubectl", args...)
	if err != nil {
		return 1
	}
	pid := parsePositiveInt(string(out))
	if pid <= 0 {
		return 1
	}
	return int32(pid)
}

func runPythonInPod(ctx context.Context, namespace, podName, container string, pid int32) error {
	var lastErr error
	for _, py := range []string{"python3", "python"} {
		args := kubectlExecArgs(
			namespace,
			podName,
			container,
			py,
			remoteScriptPath,
			"--pid",
			strconv.Itoa(int(pid)),
			"--out",
			remoteDumpPath,
		)
		_, err := runCommand(ctx, "kubectl", args...)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("execute defaults script in pod: %w", lastErr)
}

func kubectlCopyToArgs(namespace, podName, container, localPath, remotePath string) []string {
	target := fmt.Sprintf("%s/%s:%s", namespace, podName, remotePath)
	args := []string{"cp", localPath, target}
	if container != "" {
		args = append(args, "-c", container)
	}
	return args
}

func kubectlCopyFromArgs(namespace, podName, container, remotePath, localPath string) []string {
	source := fmt.Sprintf("%s/%s:%s", namespace, podName, remotePath)
	args := []string{"cp", source, localPath}
	if container != "" {
		args = append(args, "-c", container)
	}
	return args
}

func kubectlExecArgs(namespace, podName, container string, command ...string) []string {
	args := []string{"exec", "-n", namespace, podName}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--")
	args = append(args, command...)
	return args
}

func runPythonLocal(ctx context.Context, scriptPath string, pid int32, dumpPath string) error {
	var lastErr error
	for _, py := range []string{"python3", "python"} {
		_, err := runCommand(
			ctx,
			py,
			scriptPath,
			"--pid",
			strconv.Itoa(int(pid)),
			"--out",
			dumpPath,
		)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("execute defaults script on host: %w", lastErr)
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, nil
	}
	return output, fmt.Errorf("%s %s failed: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
}

func parsePositiveInt(raw string) int {
	for _, token := range strings.Fields(strings.TrimSpace(raw)) {
		value, err := strconv.Atoi(token)
		if err != nil {
			continue
		}
		if value > 0 {
			return value
		}
	}
	return 0
}
