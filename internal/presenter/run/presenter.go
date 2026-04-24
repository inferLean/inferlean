package run

import (
	"context"
	"fmt"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	collectpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/collect"
	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
	reportpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/report"
	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

type Options struct {
	Discover                vllmdiscovery.DiscoverOptions
	CollectFor              time.Duration
	ScrapeEvery             time.Duration
	OutputPath              string
	Version                 string
	DeclaredWorkloadMode    string
	DeclaredWorkloadTarget  string
	PrefixHeavy             *bool
	Multimodal              *bool
	RepeatedMultimodalMedia *bool
	NoInteractive           bool
	BackendURL              string
	RequireUpload           bool
}

type Result struct {
	ArtifactPath string
	RunID        string
	Uploaded     bool
	UploadErr    error
}

type Presenter struct {
	discover discoverpresenter.Presenter
	collect  collectpresenter.Presenter
	upload   uploadpresenter.Presenter
	report   reportpresenter.Presenter
}

func NewPresenter(
	d discoverpresenter.Presenter,
	c collectpresenter.Presenter,
	u uploadpresenter.Presenter,
	r reportpresenter.Presenter,
) Presenter {
	return Presenter{discover: d, collect: c, upload: u, report: r}
}

func (p Presenter) Run(ctx context.Context, opts Options) (Result, error) {
	target, _, err := p.discover.Run(ctx, opts.Discover)
	if err != nil {
		return Result{}, err
	}
	collectRes, err := p.collect.Run(ctx, collectpresenter.Options{
		Target:                  target,
		CollectFor:              opts.CollectFor,
		ScrapeEvery:             opts.ScrapeEvery,
		OutputPath:              opts.OutputPath,
		CollectorVersion:        opts.Version,
		DeclaredWorkloadMode:    opts.DeclaredWorkloadMode,
		DeclaredWorkloadTarget:  opts.DeclaredWorkloadTarget,
		PrefixHeavy:             opts.PrefixHeavy,
		Multimodal:              opts.Multimodal,
		RepeatedMultimodalMedia: opts.RepeatedMultimodalMedia,
		NoInteractive:           opts.NoInteractive,
	})
	if err != nil {
		return Result{}, err
	}
	result := Result{ArtifactPath: collectRes.ArtifactPath, RunID: collectRes.Artifact.Job.RunID}
	if opts.BackendURL == "" {
		opts.BackendURL = defaults.BackendURL
	}
	return p.handleUpload(ctx, opts, result)
}

func (p Presenter) handleUpload(ctx context.Context, opts Options, result Result) (Result, error) {
	uploadRes, err := p.upload.Run(ctx, uploadpresenter.Options{
		BackendURL:    opts.BackendURL,
		ArtifactPath:  result.ArtifactPath,
		RequireReport: opts.RequireUpload,
	})
	if err != nil {
		if opts.RequireUpload {
			return result, err
		}
		result.UploadErr = err
		fmt.Printf("[run] upload/report skipped with error: %v\n", err)
		return result, nil
	}
	result.Uploaded = uploadRes.Uploaded
	if uploadRes.RunID != "" {
		result.RunID = uploadRes.RunID
	}
	if len(uploadRes.Report) > 0 {
		p.report.Run(uploadRes.Report)
	}
	return result, nil
}
