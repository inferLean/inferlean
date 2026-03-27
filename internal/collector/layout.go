package collector

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func newRunID() (string, error) {
	var buf [runIDSize]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func prepareRunLayout(runID, outputPath string) (string, string, runtimeArtifacts, error) {
	artifactPath, err := resolveArtifactPath(runID, outputPath)
	if err != nil {
		return "", "", runtimeArtifacts{}, err
	}

	runDir := filepath.Dir(artifactPath)
	if err := os.MkdirAll(runDir, defaultCollectDirMode); err != nil {
		return "", "", runtimeArtifacts{}, fmt.Errorf("create run directory: %w", err)
	}
	return runDir, artifactPath, buildRuntimeArtifacts(runDir), nil
}

func resolveArtifactPath(runID, outputPath string) (string, error) {
	if outputPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		outputPath = filepath.Join(home, ".inferlean", "runs", runID, "artifact.json")
	}

	artifactPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	return artifactPath, nil
}

func buildRuntimeArtifacts(runDir string) runtimeArtifacts {
	raw := filepath.Join(runDir, "raw")
	return runtimeArtifacts{
		prometheusConfig: filepath.Join(raw, "prometheus.yml"),
		vllmRaw:          filepath.Join(raw, "vllm.metrics"),
		hostRaw:          filepath.Join(raw, "node_exporter.metrics"),
		dcgmRaw:          filepath.Join(raw, "dcgm.metrics"),
		nvidiaRaw:        filepath.Join(raw, "nvidia-smi.txt"),
		processRaw:       filepath.Join(raw, "process-inspection.json"),
		prometheusStdout: filepath.Join(raw, "prometheus.stdout.log"),
		prometheusStderr: filepath.Join(raw, "prometheus.stderr.log"),
		nodeStdout:       filepath.Join(raw, "node_exporter.stdout.log"),
		nodeStderr:       filepath.Join(raw, "node_exporter.stderr.log"),
	}
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), defaultCollectDirMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func mergeMaps(parts ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, part := range parts {
		for key, value := range part {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func relativeArtifact(path, runDir string) string {
	rel, err := filepath.Rel(runDir, path)
	if err != nil {
		return path
	}
	return rel
}

func relativeRawArtifact(path string) string {
	return relativeArtifact(path, filepath.Dir(filepath.Dir(path)))
}

func rawDir(path string) string {
	return filepath.Dir(path)
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	clone := value.UTC()
	return &clone
}
