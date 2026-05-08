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
	trimmedName := strings.TrimSpace(name)
	args := []string{"ps", "--format", "{{.ID}}|{{.Names}}"}
	if trimmedName != "" {
		args = append(args, "--filter", "name="+trimmedName)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		if trimmedName != "" {
			return nil, fmt.Errorf("list docker containers: %w", err)
		}
		return nil, nil
	}
	records := make([]containerRecord, 0)
	scan := bufio.NewScanner(bytes.NewReader(out))
	for scan.Scan() {
		parts := strings.SplitN(scan.Text(), "|", 2)
		if len(parts) != 2 {
			continue
		}
		containerID := strings.TrimSpace(parts[0])
		containerName := strings.TrimSpace(parts[1])
		inspected, err := inspectContainer(ctx, containerID)
		if err != nil {
			if trimmedName != "" && strings.EqualFold(containerName, trimmedName) {
				return nil, fmt.Errorf("inspect docker container %s: %w", containerName, err)
			}
			continue
		}
		if strings.TrimSpace(inspected.RawCommandLine) == "" {
			continue
		}
		if !shared.IsServeCommand(inspected.RawCommandLine) {
			continue
		}
		internalPID := detectInternalPID(ctx, containerID)
		records = append(records, containerRecord{
			id:          containerID,
			name:        containerName,
			internalPID: internalPID,
			inspected:   inspected,
		})
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("parse docker ps output: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return candidatesFromRecords(preferredRecords(records, trimmedName))
}

func preferredRecords(records []containerRecord, name string) []containerRecord {
	trimmedName := strings.TrimSpace(name)
	if trimmedName != "" {
		exact := make([]containerRecord, 0)
		for _, record := range records {
			if strings.EqualFold(record.name, trimmedName) {
				exact = append(exact, record)
			}
		}
		if len(exact) > 0 {
			return exact
		}
	}
	return records
}

type containerRecord struct {
	id          string
	name        string
	internalPID int32
	inspected   inspectedContainer
}

type inspectOutput []struct {
	Config struct {
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
		Env        []string `json:"Env"`
	} `json:"Config"`
	Path            string `json:"Path"`
	Args            []string
	NetworkSettings struct {
		Ports map[string][]portBinding `json:"Ports"`
	} `json:"NetworkSettings"`
	State struct {
		PID int `json:"Pid"`
	} `json:"State"`
}

type portBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type inspectedContainer struct {
	RawCommandLine string
	PID            int32
	Env            []string
	Ports          map[string][]portBinding
}

func inspectContainer(ctx context.Context, containerID string) (inspectedContainer, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	out, err := cmd.Output()
	if err != nil {
		return inspectedContainer{}, err
	}
	return parseInspectContainer(out)
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
	result := inspectedContainer{
		RawCommandLine: command,
		Env:            item.Config.Env,
		Ports:          item.NetworkSettings.Ports,
	}
	if item.State.PID > 0 {
		result.PID = int32(item.State.PID)
	}
	return result, nil
}

func candidatesFromRecords(records []containerRecord) ([]shared.Candidate, error) {
	items := make([]shared.Candidate, 0, len(records))
	for _, record := range records {
		port := shared.InferMetricsPort(record.inspected.RawCommandLine, record.inspected.Env)
		endpoint, ok := publishedMetricsEndpoint(record.inspected.Ports, port)
		if !ok {
			return nil, shared.MissingPublishedPortError(record.name, port)
		}
		items = append(items, shared.Candidate{
			Source:          "docker",
			PID:             record.inspected.PID,
			InternalPID:     record.internalPID,
			ContainerID:     record.id,
			RawCommandLine:  record.inspected.RawCommandLine,
			MetricsEndpoint: endpoint,
			Executable:      "docker-container:" + record.name,
		})
	}
	return items, nil
}

func detectInternalPID(ctx context.Context, containerID string) int32 {
	out, err := exec.CommandContext(
		ctx,
		"docker",
		"exec",
		containerID,
		"sh",
		"-c",
		shared.ProcListScript,
	).Output()
	if err != nil {
		return 0
	}
	return shared.FirstVLLMProcessPID(shared.ParseProcList(string(out)))
}

func publishedMetricsEndpoint(ports map[string][]portBinding, port int) (string, bool) {
	for _, binding := range ports[fmt.Sprintf("%d/tcp", port)] {
		hostPort, err := strconv.Atoi(strings.TrimSpace(binding.HostPort))
		if err != nil || hostPort <= 0 {
			continue
		}
		return shared.MetricsEndpoint(publishedHost(binding.HostIP), hostPort), true
	}
	return "", false
}

func publishedHost(hostIP string) string {
	host := strings.TrimSpace(hostIP)
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
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
