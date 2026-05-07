package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inferLean/inferlean-main/cli/internal/api"
	"github.com/inferLean/inferlean-main/cli/internal/evidencegate"
	configstore "github.com/inferLean/inferlean-main/cli/internal/storage/configuration"
	runstore "github.com/inferLean/inferlean-main/cli/internal/storage/run"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	uploadui "github.com/inferLean/inferlean-main/cli/internal/ui/upload"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Options struct {
	BackendURL    string
	ArtifactPath  string
	RunID         string
	RequireReport bool
}

type Result struct {
	Report         map[string]any
	RunID          string
	InstallationID string
	Uploaded       bool
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
	cfg, err := p.cfgStore.Load()
	if err != nil {
		return Result{}, err
	}
	artifactPath, err := p.artifactPath(opts)
	if err != nil {
		return Result{}, err
	}
	artifact, err := readArtifact(artifactPath)
	if err != nil {
		return Result{}, err
	}
	if failure, ok := evidencegate.Check(artifact); !ok {
		err := evidencegate.Error{Failure: failure}
		p.uploadView.ShowFailure(err)
		return Result{}, err
	}
	p.uploadView.ShowUploadStart()
	ack, err := p.apiClient.UploadArtifact(ctx, opts.BackendURL, artifact, cfg.Auth)
	if err != nil {
		p.uploadView.ShowFailure(err)
		return Result{}, err
	}
	p.uploadView.ShowUploadSuccess()
	result := Result{
		RunID:          ack.RunID,
		InstallationID: ack.InstallationID,
		Uploaded:       true,
	}
	report, err := p.loadReport(ctx, ack.ReportURL, cfg.Auth, artifactPath, opts.RequireReport)
	if err != nil {
		return Result{}, err
	}
	result.Report = report
	return result, nil
}

func (p Presenter) artifactPath(opts Options) (string, error) {
	if opts.RunID == "" {
		return opts.ArtifactPath, nil
	}
	return p.runStore.ArtifactPath(opts.RunID)
}

func (p Presenter) loadReport(ctx context.Context, reportURL string, auth types.AuthState, artifactPath string, required bool) (map[string]any, error) {
	if reportURL == "" {
		if required {
			return nil, fmt.Errorf("upload succeeded but report_url was empty")
		}
		return nil, nil
	}
	report, err := p.apiClient.GetReport(ctx, reportURL, auth)
	if err != nil {
		if required {
			return nil, err
		}
		return nil, nil
	}
	if err := p.runStore.SaveReport(filepath.Join(filepath.Dir(artifactPath), "report.json"), report); err != nil {
		return nil, err
	}
	return report, nil
}

func readArtifact(path string) (contracts.RunArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return contracts.RunArtifact{}, fmt.Errorf("read artifact: %w", err)
	}
	var artifact contracts.RunArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return contracts.RunArtifact{}, fmt.Errorf("decode artifact: %w", err)
	}
	if err := artifact.Validate(); err != nil {
		return contracts.RunArtifact{}, fmt.Errorf("validate artifact: %w", err)
	}
	return artifact, nil
}
