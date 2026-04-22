package vllmdefaults

import (
	"fmt"
	"path/filepath"
)

const defaultsDirEnv = "INFERLEAN_VLLM_DEFAULTS_DIR"

type Input struct {
	RawCommandLine string
	ExplicitArgs   map[string]string
	VLLMVersion    string
	GPUModel       string
	GPUMemoryMiB   float64
}

type Output struct {
	Args              map[string]string
	SelectedTag       string
	SelectedProfile   string
	SelectedModel     string
	RequestedVersion  string
	AppliedDefaults   int
	DefaultsDir       string
	RuntimeSource     string
	RuntimeDumpPath   string
	RuntimeScriptPath string
	RuntimePID        int32
	ResolvedVersion   string
	RuntimeWarnings   string
	RuntimeErrors     string
}

type manifestFile struct {
	Generator struct {
		ProcessedTags []string `json:"processed_tags"`
	} `json:"generator"`
}

type tagDefaultsFile struct {
	Profiles map[string]profileDefaults `json:"profiles"`
}

type profileDefaults struct {
	Models   map[string]modelDefaults `json:"models"`
	Resolved map[string]any           `json:"resolved"`
}

type modelDefaults struct {
	Resolved map[string]any `json:"resolved"`
}

type semanticVersion struct {
	Major       int
	Minor       int
	Patch       int
	PreRelease  string
	PreReleaseN int
}

func Resolve(input Input) (Output, error) {
	defaultsDir, err := discoverDefaultsDir()
	if err != nil {
		return Output{}, err
	}
	return ResolveWithDir(defaultsDir, input)
}

func ResolveWithDir(defaultsDir string, input Input) (Output, error) {
	manifestPath := filepath.Join(defaultsDir, "simple_cuda_by_tag", "manifest.json")
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return Output{}, err
	}
	if len(manifest.Generator.ProcessedTags) == 0 {
		return Output{}, fmt.Errorf("vLLM defaults manifest has no tags")
	}

	explicit := normalizeArgs(input.ExplicitArgs)
	model := inferModel(explicit, input.RawCommandLine)
	requestedVersion := inferRequestedVersion(input, explicit)
	selectedTag := selectTag(manifest.Generator.ProcessedTags, requestedVersion)
	tagFilePath := filepath.Join(defaultsDir, "simple_cuda_by_tag", "tags", selectedTag+".json")
	tagDefaults, err := loadTagDefaults(tagFilePath)
	if err != nil {
		return Output{}, err
	}
	selectedProfile := selectProfile(tagDefaults.Profiles, input)
	if selectedProfile == "" {
		return Output{}, fmt.Errorf("no profile found for tag %s", selectedTag)
	}

	resolved := copyStringMap(explicit)
	applied := applyDefaults(resolved, tagDefaults.Profiles[selectedProfile], model)

	return Output{
		Args:             resolved,
		SelectedTag:      selectedTag,
		SelectedProfile:  selectedProfile,
		SelectedModel:    model,
		RequestedVersion: requestedVersion,
		AppliedDefaults:  applied,
		DefaultsDir:      defaultsDir,
	}, nil
}
