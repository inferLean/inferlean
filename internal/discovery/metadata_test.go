package discovery

import "testing"

func TestParseDockerPorts(t *testing.T) {
	t.Parallel()

	bindings := parseDockerPorts("0.0.0.0:8000->8000/tcp, [::]:8000->8000/tcp")
	if len(bindings) != 2 {
		t.Fatalf("binding count = %d, want 2", len(bindings))
	}
	if bindings[0].HostIP != "0.0.0.0" || bindings[0].HostPort != 8000 || bindings[0].ContainerPort != 8000 || bindings[0].Protocol != "tcp" {
		t.Fatalf("first binding = %+v, want 0.0.0.0:8000->8000/tcp", bindings[0])
	}
	if bindings[1].HostIP != "::" || bindings[1].HostPort != 8000 || bindings[1].ContainerPort != 8000 || bindings[1].Protocol != "tcp" {
		t.Fatalf("second binding = %+v, want [::]:8000->8000/tcp", bindings[1])
	}
}

func TestApplyDockerPortBindingFillsMissingRuntimePort(t *testing.T) {
	t.Parallel()

	group := CandidateGroup{}
	applyDockerPortBinding(&group, dockerContainer{
		Ports: []dockerPortBinding{
			{HostIP: "127.0.0.1", HostPort: 18000, ContainerPort: 8000, Protocol: "tcp"},
		},
	})

	if group.RuntimeConfig.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", group.RuntimeConfig.Host)
	}
	if group.RuntimeConfig.Port != 18000 {
		t.Fatalf("port = %d, want 18000", group.RuntimeConfig.Port)
	}
}

func TestApplyDockerPortBindingPreservesExplicitRuntimePort(t *testing.T) {
	t.Parallel()

	group := CandidateGroup{RuntimeConfig: RuntimeConfig{Host: "0.0.0.0", Port: 8001}}
	applyDockerPortBinding(&group, dockerContainer{
		Ports: []dockerPortBinding{
			{HostIP: "127.0.0.1", HostPort: 18000, ContainerPort: 8000, Protocol: "tcp"},
		},
	})

	if group.RuntimeConfig.Host != "0.0.0.0" || group.RuntimeConfig.Port != 8001 {
		t.Fatalf("runtime config = %+v, want explicit host/port preserved", group.RuntimeConfig)
	}
}
