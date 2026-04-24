package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inferLean/inferlean-main/cli/internal/api"
	"github.com/inferLean/inferlean-main/cli/internal/defaults"
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
	Ack            types.UploadAck
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
	backend := opts.BackendURL
	if backend == "" {
		backend = cfg.Auth.BackendURL
	}
	if backend == "" {
		backend = defaults.BackendURL
	}
	if backend == "" {
		return Result{}, fmt.Errorf("backend URL is required")
	}
	if opts.RunID != "" {
		p.uploadView.ShowReportFetchStart(opts.RunID)
		report, err := p.apiClient.GetRunReport(ctx, backend, opts.RunID, cfg.Auth)
		if err != nil {
			p.uploadView.ShowFailure(err)
			return Result{}, err
		}
		p.uploadView.ShowReportFetchSuccess(opts.RunID)
		return Result{
			Report:         report,
			RunID:          opts.RunID,
			InstallationID: installationIDFromReport(report),
		}, nil
	}
	artifact, err := readArtifact(opts.ArtifactPath)
	if err != nil {
		return Result{}, err
	}
	p.uploadView.ShowUploadStart()
	ack, err := p.apiClient.UploadArtifact(ctx, backend, artifact, cfg.Auth)
	if err != nil {
		p.uploadView.ShowFailure(err)
		return Result{}, err
	}
	p.uploadView.ShowUploadSuccess()
	result := Result{
		Ack:            ack,
		RunID:          ack.RunID,
		InstallationID: ack.InstallationID,
		Uploaded:       true,
	}
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
	reportPath := filepath.Join(runDir, "report.json")
	if err := p.runStore.SaveReport(reportPath, report); err != nil {
		return Result{}, err
	}
	return result, nil
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

func artifactRunDir(path string) string {
	return filepath.Dir(path)
}

func installationIDFromReport(report map[string]any) string {
	job, ok := report["job"].(map[string]any)
	if !ok {
		return ""
	}
	installationID, _ := job["installation_id"].(string)
	return installationID
}
