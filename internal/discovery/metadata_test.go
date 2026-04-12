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

func TestKubernetesInventoryNamespaceUsesExplicitPodNamespace(t *testing.T) {
	t.Parallel()

	namespace := kubernetesInventoryNamespace(Options{
		Pod:       "vllm-llm-0",
		Namespace: "nortal",
	})

	if namespace != "nortal" {
		t.Fatalf("namespace = %q, want nortal", namespace)
	}
}

func TestKubernetesInventoryNamespaceParsesPodNamespace(t *testing.T) {
	t.Parallel()

	namespace := kubernetesInventoryNamespace(Options{Pod: "nortal/vllm-llm-0"})

	if namespace != "nortal" {
		t.Fatalf("namespace = %q, want nortal", namespace)
	}
}

func TestKubernetesCandidateGroupsAddsVLLMImagePod(t *testing.T) {
	t.Parallel()

	groups := kubernetesCandidateGroups(t.Context(), []kubernetesPod{{
		Namespace: "nortal",
		Name:      "vllm-llm-0",
		PodIP:     "192.168.73.168",
		Containers: []kubernetesContainer{{
			Name:  "vllm-llm",
			Image: "vllm/vllm-openai:v0.13.0",
			Args: []string{
				"Qwen/Qwen3-32B",
				"--gpu-memory-utilization",
				"0.95",
				"--max-model-len",
				"32768",
				"--dtype",
				"bfloat16",
				"--max-num-seqs",
				"32",
				"--tensor-parallel-size",
				"2",
				"--generation-config",
				"vllm",
			},
			Ports: []int{8000},
		}},
	}}, nil)

	if len(groups) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(groups))
	}
	group := groups[0]
	if group.Target.Kind != TargetKindKubernetes || group.Target.KubernetesPodName != "vllm-llm-0" {
		t.Fatalf("target = %+v, want kubernetes pod", group.Target)
	}
	if group.RuntimeConfig.Model != "Qwen/Qwen3-32B" {
		t.Fatalf("model = %q, want Qwen/Qwen3-32B", group.RuntimeConfig.Model)
	}
	if group.RuntimeConfig.Port != 8000 || group.RuntimeConfig.Host != "192.168.73.168" {
		t.Fatalf("listen = %s:%d, want 192.168.73.168:8000", group.RuntimeConfig.Host, group.RuntimeConfig.Port)
	}
	if group.RuntimeConfig.TensorParallelSize != 2 || group.RuntimeConfig.MaxModelLen != 32768 {
		t.Fatalf("runtime config = %+v, want parsed pod args", group.RuntimeConfig)
	}
}

func TestKubernetesCandidateGroupsResolvesConfigMapEnvRefs(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GENERATIVE_MODEL_NAME": "Qwen/Qwen3-32B",
		"MAX_MODEL_LEN":         "32768",
	}
	args := resolveKubernetesArgs([]string{"$(GENERATIVE_MODEL_NAME)", "--max-model-len", "$(MAX_MODEL_LEN)"}, env)

	if args[0] != "Qwen/Qwen3-32B" || args[2] != "32768" {
		t.Fatalf("resolved args = %#v", args)
	}
}
