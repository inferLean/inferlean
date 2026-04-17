package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inferLean/inferlean-main/new-cli/internal/api"
	configstore "github.com/inferLean/inferlean-main/new-cli/internal/storage/configuration"
	runstore "github.com/inferLean/inferlean-main/new-cli/internal/storage/run"
	"github.com/inferLean/inferlean-main/new-cli/internal/types"
	uploadui "github.com/inferLean/inferlean-main/new-cli/internal/ui/upload"
)

type Options struct {
	BackendURL    string
	ArtifactPath  string
	RequireReport bool
}

type Result struct {
	Ack      types.UploadAck
	Report   map[string]any
	Uploaded bool
}

type Presenter struct {
	apiClient  api.Client
	cfgStore   *configstore.Store
	runStore   runstore.Store
	uploadView uploadui.View
}

func NewPresenter(cfgStore *configstore.Store, view uploadui.View) Presenter {
	return Presenter{apiClient: api.NewClient(), cfgStore: cfgStore, runStore: runstore.NewStore(), uploadView: view}
}

func (p Presenter) Run(ctx context.Context, opts Options) (Result, error) {
	artifact, err := readArtifact(opts.ArtifactPath)
	if err != nil {
		return Result{}, err
	}
	cfg, err := p.cfgStore.Load()
	if err != nil {
		return Result{}, err
	}
	backend := opts.BackendURL
	if backend == "" {
		backend = cfg.Auth.BackendURL
	}
	if backend == "" {
		return Result{}, fmt.Errorf("backend URL is required")
	}
	p.uploadView.ShowStart()
	ack, err := p.apiClient.UploadArtifact(ctx, backend, artifact, cfg.Auth)
	if err != nil {
		p.uploadView.ShowFailure(err)
		return Result{}, err
	}
	p.uploadView.ShowSuccess(ack.ReportURL)
	result := Result{Ack: ack, Uploaded: true}
	if ack.ReportURL == "" {
		if opts.RequireReport {
			return Result{}, fmt.Errorf("upload succeeded but report_url was empty")
		}
		return result, nil
	}
	report, err := p.apiClient.GetReport(ctx, ack.ReportURL, cfg.Auth)
	if err != nil {
		if opts.RequireReport {
			return Result{}, err
		}
		return result, nil
	}
	result.Report = report
	runDir := artifactRunDir(opts.ArtifactPath)
	reportPath := runDir + "/report.json"
	if err := p.runStore.SaveReport(reportPath, report); err != nil {
		return Result{}, err
	}
	return result, nil
}

func readArtifact(path string) (types.Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return types.Artifact{}, fmt.Errorf("read artifact: %w", err)
	}
	var artifact types.Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return types.Artifact{}, fmt.Errorf("parse artifact: %w", err)
	}
	return artifact, nil
}

func artifactRunDir(path string) string {
	return filepath.Dir(path)
}
