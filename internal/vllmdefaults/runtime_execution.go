package vllmdefaults

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

const (
	remoteScriptPath = "/tmp/inferlean-dump-vllm-defaults.py"
	remoteDumpPath   = "/tmp/inferlean-vllm-defaults.json"
)

func runDumpScript(ctx context.Context, target shared.Candidate, scriptPath, dumpPath, modelPathOverride string) (runtimeExecution, error) {
	source := strings.ToLower(strings.TrimSpace(target.Source))
	switch source {
	case "docker":
		return runDumpInDocker(ctx, target, scriptPath, dumpPath, modelPathOverride)
	case "pod", "kubernetes":
		return runDumpInPod(ctx, target, scriptPath, dumpPath, modelPathOverride)
	default:
		return runDumpOnHost(ctx, target, scriptPath, dumpPath, modelPathOverride)
	}
}

func runDumpOnHost(ctx context.Context, target shared.Candidate, scriptPath, dumpPath, modelPathOverride string) (runtimeExecution, error) {
	pid, err := runtimePID(target, "process")
	if err != nil {
		return runtimeExecution{}, err
	}
	if err := runPythonLocal(ctx, target, scriptPath, pid, dumpPath, modelPathOverride); err != nil {
		return runtimeExecution{}, err
	}
	return runtimeExecution{Source: "process", PID: pid}, nil
}

func runDumpInDocker(ctx context.Context, target shared.Candidate, scriptPath, dumpPath, modelPathOverride string) (runtimeExecution, error) {
	containerID := strings.TrimSpace(target.ContainerID)
	if containerID == "" {
		return runtimeExecution{}, fmt.Errorf("cannot run defaults script for docker target without container id")
	}
	pid, err := runtimePID(target, "docker")
	if err != nil {
		return runtimeExecution{}, err
	}

	if _, err := runCommand(ctx, "docker", "cp", scriptPath, containerID+":"+remoteScriptPath); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults script into container: %w", err)
	}

	if err := runPythonInDocker(ctx, target, containerID, pid, modelPathOverride); err != nil {
		return runtimeExecution{}, err
	}
	if _, err := runCommand(ctx, "docker", "cp", containerID+":"+remoteDumpPath, dumpPath); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults dump from container: %w", err)
	}
	_, _ = runCommand(ctx, "docker", "exec", containerID, "rm", "-f", remoteScriptPath, remoteDumpPath)
	return runtimeExecution{Source: "docker", PID: pid}, nil
}

func runPythonInDocker(ctx context.Context, target shared.Candidate, containerID string, pid int32, modelPathOverride string) error {
	return runPythonCandidates(ctx, "in container", pythonCandidates(target, pid), func(py string) (string, []string) {
		args := []string{
			"exec",
			containerID,
			py,
			remoteScriptPath,
		}
		args = append(args, dumpScriptArgs(pid, remoteDumpPath, modelPathOverride)...)
		return "docker", args
	})
}

func runDumpInPod(ctx context.Context, target shared.Candidate, scriptPath, dumpPath, modelPathOverride string) (runtimeExecution, error) {
	podName := strings.TrimSpace(target.PodName)
	if podName == "" {
		return runtimeExecution{}, fmt.Errorf("cannot run defaults script for pod target without pod name")
	}
	namespace := strings.TrimSpace(target.Namespace)
	pid, err := runtimePID(target, "pod")
	if err != nil {
		return runtimeExecution{}, err
	}
	container := podContainerName(target.Executable)

	if _, err := runCommand(ctx, "kubectl", kubectlCopyToArgs(namespace, podName, container, scriptPath, remoteScriptPath)...); err != nil {
		return runtimeExecution{}, fmt.Errorf("copy defaults script into pod: %w", err)
	}

	if err := runPythonInPod(ctx, target, namespace, podName, container, pid, modelPathOverride); err != nil {
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

func runPythonInPod(ctx context.Context, target shared.Candidate, namespace, podName, container string, pid int32, modelPathOverride string) error {
	return runPythonCandidates(ctx, "in pod", pythonCandidates(target, pid), func(py string) (string, []string) {
		command := []string{py, remoteScriptPath}
		command = append(command, dumpScriptArgs(pid, remoteDumpPath, modelPathOverride)...)
		return "kubectl", kubectlExecArgs(namespace, podName, container, command...)
	})
}

func kubectlCopyToArgs(namespace, podName, container, localPath, remotePath string) []string {
	target := kubectlFileRef(namespace, podName, remotePath)
	args := []string{"cp", localPath, target}
	if container != "" {
		args = append(args, "-c", container)
	}
	return args
}

func kubectlCopyFromArgs(namespace, podName, container, remotePath, localPath string) []string {
	source := kubectlFileRef(namespace, podName, remotePath)
	args := []string{"cp", source, localPath}
	if container != "" {
		args = append(args, "-c", container)
	}
	return args
}

func kubectlExecArgs(namespace, podName, container string, command ...string) []string {
	args := []string{"exec"}
	if strings.TrimSpace(namespace) != "" {
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

func kubectlFileRef(namespace, podName, remotePath string) string {
	if strings.TrimSpace(namespace) == "" {
		return fmt.Sprintf("%s:%s", podName, remotePath)
	}
	return fmt.Sprintf("%s/%s:%s", namespace, podName, remotePath)
}

func runtimePID(target shared.Candidate, source string) (int32, error) {
	if target.InternalPID > 0 {
		return target.InternalPID, nil
	}
	if strings.EqualFold(strings.TrimSpace(target.Source), "process") && target.PID > 0 {
		return target.PID, nil
	}
	return 0, fmt.Errorf("cannot run defaults script for %s target without internal pid", source)
}

func runPythonLocal(ctx context.Context, target shared.Candidate, scriptPath string, pid int32, dumpPath string, modelPathOverride string) error {
	return runPythonCandidates(ctx, "on host", pythonCandidates(target, pid), func(py string) (string, []string) {
		args := []string{scriptPath}
		args = append(args, dumpScriptArgs(pid, dumpPath, modelPathOverride)...)
		return py, args
	})
}

func dumpScriptArgs(pid int32, dumpPath string, modelPathOverride string) []string {
	args := []string{
		"--pid",
		strconv.Itoa(int(pid)),
		"--out",
		dumpPath,
	}
	if override := strings.TrimSpace(modelPathOverride); override != "" {
		args = append(args, "--model-path-override", override)
	}
	args = append(
		args,
		"--effective-timeout-seconds",
		strconv.Itoa(runtimeDefaultsEffectiveTimeoutSeconds),
	)
	return args
}

func pythonCandidates(target shared.Candidate, pid int32) []string {
	candidates := pythonCandidatesFromCommandLine(target.RawCommandLine)
	candidates = append(candidates, fmt.Sprintf("/proc/%d/exe", pid))
	return dedupeInterpreters(candidates)
}

func pythonCandidatesFromCommandLine(raw string) []string {
	argv := strings.Fields(strings.TrimSpace(raw))
	if len(argv) == 0 {
		return nil
	}
	executable := strings.Trim(argv[0], `"'`)
	if !isPythonInterpreter(executable) {
		return nil
	}
	return []string{executable}
}

func isPythonInterpreter(path string) bool {
	base := filepath.Base(strings.TrimSpace(path))
	return base == "python" ||
		strings.HasPrefix(base, "python2") ||
		strings.HasPrefix(base, "python3")
}

func runPythonCandidates(ctx context.Context, label string, interpreters []string, command func(py string) (string, []string)) error {
	var lastErr error
	for _, py := range dedupeInterpreters(interpreters) {
		name, args := command(py)
		_, err := runCommand(ctx, name, args...)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("execute defaults script %s: %w", label, lastErr)
}

func dedupeInterpreters(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, nil
	}
	return output, fmt.Errorf("%s %s failed: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
}
