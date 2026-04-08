package discovery

import (
	"fmt"
	"strings"
)

type TargetKind string

const (
	TargetKindHost       TargetKind = "host"
	TargetKindDocker     TargetKind = "docker"
	TargetKindKubernetes TargetKind = "kubernetes"
)

type TargetRef struct {
	Kind                    TargetKind
	DockerContainerID       string
	DockerContainerName     string
	KubernetesNamespace     string
	KubernetesPodName       string
	KubernetesContainerName string
}

func (g CandidateGroup) IdentityLabel() string {
	return g.Target.IdentityLabel(g.PrimaryPID)
}

func (g CandidateGroup) LocationLabel() string {
	return g.Target.LocationLabel()
}

func (t TargetRef) normalizedKind() TargetKind {
	if t.Kind == "" {
		return TargetKindHost
	}
	return t.Kind
}

func (t TargetRef) IdentityLabel(pid int32) string {
	switch t.normalizedKind() {
	case TargetKindDocker:
		return "container " + t.containerLabel()
	case TargetKindKubernetes:
		return fmt.Sprintf("pod %s/%s", defaultNamespace(t.KubernetesNamespace), strings.TrimSpace(t.KubernetesPodName))
	default:
		return fmt.Sprintf("PID %d", pid)
	}
}

func (t TargetRef) IsHost() bool {
	return t.normalizedKind() == TargetKindHost
}

func (t TargetRef) LocationLabel() string {
	switch t.normalizedKind() {
	case TargetKindDocker:
		return "Docker container " + t.containerLabel()
	case TargetKindKubernetes:
		return fmt.Sprintf("Kubernetes pod %s/%s", defaultNamespace(t.KubernetesNamespace), strings.TrimSpace(t.KubernetesPodName))
	default:
		return "Host process"
	}
}

func (t TargetRef) MatchesContainer(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	return strings.EqualFold(t.DockerContainerName, query) || strings.HasPrefix(t.DockerContainerID, query)
}

func (t TargetRef) MatchesPod(name, namespace string) bool {
	return strings.EqualFold(strings.TrimSpace(t.KubernetesPodName), strings.TrimSpace(name)) &&
		strings.EqualFold(defaultNamespace(t.KubernetesNamespace), defaultNamespace(namespace))
}

func (t TargetRef) containerLabel() string {
	name := strings.TrimSpace(t.DockerContainerName)
	shortID := shortenContainerID(t.DockerContainerID)
	switch {
	case name != "" && shortID != "":
		return fmt.Sprintf("%s (%s)", name, shortID)
	case name != "":
		return name
	case shortID != "":
		return shortID
	default:
		return "unknown"
	}
}

func defaultNamespace(value string) string {
	if strings.TrimSpace(value) == "" {
		return "default"
	}
	return value
}

func shortenContainerID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
