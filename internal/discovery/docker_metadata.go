package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

type dockerContainer struct {
	ID    string
	Name  string
	Ports []dockerPortBinding
}

type dockerPortBinding struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Protocol      string
}

func loadDockerInventory(ctx context.Context) (map[string]dockerContainer, []dockerContainer, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, nil, errors.New("docker was not found in PATH")
	}

	output, err := exec.CommandContext(ctx, "docker", "ps", "--no-trunc", "--format", "{{.ID}}\t{{.Names}}\t{{.Ports}}").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("list docker containers: %w", err)
	}

	index := map[string]dockerContainer{}
	containers := []dockerContainer{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		container := dockerContainerFromPSLine(line)
		index[container.ID] = container
		containers = append(containers, container)
	}

	return index, containers, nil
}

func dockerContainerFromPSLine(line string) dockerContainer {
	parts := strings.SplitN(line, "\t", 3)
	container := dockerContainer{ID: strings.TrimSpace(parts[0])}
	if len(parts) >= 2 {
		container.Name = strings.TrimSpace(parts[1])
	}
	if len(parts) == 3 {
		container.Ports = parseDockerPorts(parts[2])
	}
	return container
}

func applyDockerPortBinding(group *CandidateGroup, container dockerContainer) {
	if group.RuntimeConfig.Port != 0 {
		return
	}
	binding, ok := selectVLLMPortBinding(container.Ports)
	if !ok {
		return
	}
	group.RuntimeConfig.Port = binding.HostPort
	if group.RuntimeConfig.Host == "" {
		group.RuntimeConfig.Host = dockerHostIP(binding.HostIP)
	}
}

func selectVLLMPortBinding(bindings []dockerPortBinding) (dockerPortBinding, bool) {
	var fallback dockerPortBinding
	for _, binding := range bindings {
		if binding.HostPort == 0 || !strings.EqualFold(binding.Protocol, "tcp") {
			continue
		}
		if binding.ContainerPort == 8000 {
			return binding, true
		}
		if fallback.HostPort == 0 {
			fallback = binding
		}
	}
	return fallback, fallback.HostPort != 0
}

func parseDockerPorts(raw string) []dockerPortBinding {
	bindings := []dockerPortBinding{}
	for _, part := range strings.Split(raw, ",") {
		binding, ok := parseDockerPortBinding(strings.TrimSpace(part))
		if ok {
			bindings = append(bindings, binding)
		}
	}
	return bindings
}

func parseDockerPortBinding(raw string) (dockerPortBinding, bool) {
	left, right, ok := strings.Cut(raw, "->")
	if !ok {
		return dockerPortBinding{}, false
	}
	containerPort, protocol, ok := parseDockerContainerPort(right)
	if !ok {
		return dockerPortBinding{}, false
	}
	hostIP, hostPort, ok := parseDockerHostPort(left)
	if !ok {
		return dockerPortBinding{}, false
	}
	return dockerPortBinding{
		HostIP:        hostIP,
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      protocol,
	}, true
}

func parseDockerContainerPort(raw string) (int, string, bool) {
	portText, protocol, ok := strings.Cut(strings.TrimSpace(raw), "/")
	if !ok {
		return 0, "", false
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, "", false
	}
	return port, protocol, true
}

func parseDockerHostPort(raw string) (string, int, bool) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, false
	}
	return strings.Trim(host, "[]"), port, true
}

func dockerHostIP(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0.0.0.0"
	}
	return strings.Trim(value, "[]")
}
