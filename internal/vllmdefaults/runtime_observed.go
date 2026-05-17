package vllmdefaults

import "strings"

func applyTrustedRuntimeObservedDefaults(out *Output, effective map[string]any) int {
	if out == nil || len(effective) == 0 {
		return 0
	}
	if out.Args == nil {
		out.Args = map[string]string{}
	}
	if out.ArgSources == nil {
		out.ArgSources = map[string]string{}
	}
	effectiveSources := normalizeEffectiveSources(effective["_sources"])
	applied := 0
	for rawKey, rawValue := range effective {
		if strings.HasPrefix(strings.TrimSpace(rawKey), "_") {
			continue
		}
		key := normalizeKey(strings.ReplaceAll(rawKey, "_", "-"))
		if key == "" || !allowedEffectiveKeys[key] {
			continue
		}
		if _, exists := out.Args[key]; exists {
			continue
		}
		source := sourceLabel(effectiveSources[key])
		if !trustedRuntimeObservedSource(source) {
			continue
		}
		value := stringifyValue(rawValue)
		if strings.TrimSpace(value) == "" {
			continue
		}
		out.Args[key] = value
		out.ArgSources[key] = source
		applied++
	}
	return applied
}

func trustedRuntimeObservedSource(source string) bool {
	return strings.HasPrefix(strings.TrimSpace(source), "runtime_import.")
}
