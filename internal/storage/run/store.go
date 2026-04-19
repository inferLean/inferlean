package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

const (
	dirPerm  = 0o700
	filePerm = 0o600
)

type Paths struct {
	RunDir       string
	ArtifactPath string
	ReportPath   string
	Observations string
	ProcessIO    string
}

type Store struct{}

func NewStore() Store {
	return Store{}
}

func (Store) Prepare(runID string, outputPath string) (Paths, error) {
	artifactPath, err := resolveArtifactPath(runID, outputPath)
	if err != nil {
		return Paths{}, err
	}
	runDir := filepath.Dir(artifactPath)
	paths := Paths{
		RunDir:       runDir,
		ArtifactPath: artifactPath,
		ReportPath:   filepath.Join(runDir, "report.json"),
		Observations: filepath.Join(runDir, "observations"),
		ProcessIO:    filepath.Join(runDir, "process-io"),
	}
	if err := os.MkdirAll(paths.Observations, dirPerm); err != nil {
		return Paths{}, fmt.Errorf("create observations dir: %w", err)
	}
	if err := os.MkdirAll(paths.ProcessIO, dirPerm); err != nil {
		return Paths{}, fmt.Errorf("create process-io dir: %w", err)
	}
	return paths, nil
}

func resolveArtifactPath(runID, outputPath string) (string, error) {
	if outputPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		outputPath = filepath.Join(home, ".inferlean", "runs", runID, "artifact.json")
	}
	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	return abs, nil
}

func (Store) SaveArtifact(path string, artifact contracts.RunArtifact) error {
	return writeJSON(path, artifact)
}

func (Store) SaveReport(path string, report map[string]any) error {
	return writeJSON(path, report)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, filePerm)
}
