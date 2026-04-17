package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

func Discover(ctx context.Context, name string) ([]shared.Candidate, error) {
	args := []string{"ps", "--format", "{{.ID}}|{{.Names}}"}
	if strings.TrimSpace(name) != "" {
		args = append(args, "--filter", "name="+name)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	trimmedName := strings.TrimSpace(name)
	type namedCandidate struct {
		name      string
		candidate shared.Candidate
	}
	items := make([]namedCandidate, 0)
	scan := bufio.NewScanner(bytes.NewReader(out))
	for scan.Scan() {
		parts := strings.SplitN(scan.Text(), "|", 2)
		if len(parts) != 2 {
			continue
		}
		containerID := strings.TrimSpace(parts[0])
		containerName := strings.TrimSpace(parts[1])
		inspected, err := inspectContainer(ctx, containerID)
		if err != nil || strings.TrimSpace(inspected.RawCommandLine) == "" {
			continue
		}
		if !shared.IsServeCommand(inspected.RawCommandLine) {
			continue
		}
		items = append(items, namedCandidate{
			name: containerName,
			candidate: shared.Candidate{
				Source:         "docker",
				PID:            inspected.PID,
				ContainerID:    containerID,
				RawCommandLine: inspected.RawCommandLine,
				Executable:     "docker-container:" + containerName,
			},
		})
	}
	if len(items) == 0 {
		return nil, nil
	}
	exact := make([]shared.Candidate, 0)
	for _, item := range items {
		if trimmedName != "" && strings.EqualFold(item.name, trimmedName) {
			exact = append(exact, item.candidate)
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}
	outItems := make([]shared.Candidate, 0, len(items))
	for _, item := range items {
		outItems = append(outItems, item.candidate)
	}
	return outItems, nil
}

type inspectOutput []struct {
	Config struct {
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
	} `json:"Config"`
	Path  string `json:"Path"`
	Args  []string
	State struct {
		PID int `json:"Pid"`
	} `json:"State"`
}

type inspectedContainer struct {
	RawCommandLine string
	PID            int32
}

func inspectContainer(ctx context.Context, containerID string) (inspectedContainer, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	out, err := cmd.Output()
	if err != nil {
		return inspectedContainer{}, err
	}
	return parseInspectContainer(out)
}

func inspectRawCommandLine(ctx context.Context, containerID string) (string, error) {
	inspected, err := inspectContainer(ctx, containerID)
	if err != nil {
		return "", err
	}
	return inspected.RawCommandLine, nil
}

func parseInspectContainer(payload []byte) (inspectedContainer, error) {
	inspected, err := parseInspectOutput(payload)
	if err != nil {
		return inspectedContainer{}, err
	}
	item := inspected[0]
	command := renderCommand(appendSlices(item.Config.Entrypoint, item.Config.Cmd))
	if strings.TrimSpace(command) == "" {
		command = renderCommand(appendSlices([]string{item.Path}, item.Args))
	}
	result := inspectedContainer{RawCommandLine: command}
	if item.State.PID > 0 {
		result.PID = int32(item.State.PID)
	}
	return result, nil
}

func parseInspectRawCommandLine(payload []byte) (string, error) {
	inspected, err := parseInspectContainer(payload)
	if err != nil {
		return "", err
	}
	return inspected.RawCommandLine, nil
}

func parseInspectOutput(payload []byte) (inspectOutput, error) {
	var inspected inspectOutput
	if err := json.Unmarshal(payload, &inspected); err != nil {
		return nil, err
	}
	if len(inspected) == 0 {
		return nil, fmt.Errorf("inspect payload is empty")
	}
	return inspected, nil
}

func appendSlices(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

func renderCommand(parts []string) string {
	rendered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.ContainsAny(part, " \t") {
			rendered = append(rendered, strconv.Quote(part))
			continue
		}
		rendered = append(rendered, part)
	}
	return strings.Join(rendered, " ")
}
