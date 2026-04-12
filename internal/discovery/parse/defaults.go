package parse

const defaultVLLMServePort = 8000

type explicitRuntimeFlags struct {
	Port bool
}

func applyRuntimeDefaults(cfg *RuntimeConfig, entryPoint string, explicit explicitRuntimeFlags) {
	if entryPoint == "vllm serve" && !explicit.Port {
		cfg.Port = defaultVLLMServePort
		cfg.PortDefaulted = true
	}
}
