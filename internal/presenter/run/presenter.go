package run

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/evidencegate"
	collectpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/collect"
	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
	reportpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/report"
	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
)

type Options struct {
	Discover discoverpresenter.Options
	Collect  collectpresenter.Options
	Upload   uploadpresenter.Options
	Report   reportpresenter.Options
}

type Result struct {
	ArtifactPath   string
	RunID          string
	InstallationID string
	Uploaded       bool
	Failed         bool
	FailureReason  string
	FailureHint    string
	UploadErr      error
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
	collectOpts := opts.Collect
	collectOpts.Target = target
	collectRes, err := p.collect.Run(ctx, collectOpts)
	if err != nil {
		return Result{}, err
	}
	result := Result{ArtifactPath: collectRes.ArtifactPath, RunID: collectRes.Artifact.Job.RunID}

	uploadOpts := opts.Upload
	uploadOpts.ArtifactPath = result.ArtifactPath
	uploadRes, err := p.upload.Run(ctx, uploadOpts)
	if err != nil {
		if failedResult, ok := evidenceFailureResult(result, err); ok {
			return failedResult, finish(failedResult)
		}
		if opts.Upload.RequireReport {
			return result, err
		}
		result.UploadErr = err
		return result, finish(result)
	}
	result.Uploaded = uploadRes.Uploaded
	if uploadRes.RunID != "" {
		result.RunID = uploadRes.RunID
	}
	if uploadRes.InstallationID != "" {
		result.InstallationID = uploadRes.InstallationID
	}
	reportOpts := opts.Report
	reportOpts.Payload = uploadRes.Report
	reportOpts.RunID = result.RunID
	reportOpts.InstallationID = result.InstallationID
	p.report.Run(reportOpts)
	return result, finish(result)
}

func evidenceFailureResult(result Result, err error) (Result, bool) {
	var evidenceErr evidencegate.Error
	if !errors.As(err, &evidenceErr) {
		return result, false
	}
	result.Failed = true
	result.FailureReason = evidenceErr.Failure.Reason
	result.FailureHint = evidenceErr.Failure.Hint
	return result, true
}

func finish(result Result) error {
	if result.Failed {
		fmt.Println("status: fail")
		if strings.TrimSpace(result.FailureReason) != "" {
			fmt.Printf("reason: %s\n", result.FailureReason)
		}
		if strings.TrimSpace(result.FailureHint) != "" {
			fmt.Printf("hint: %s\n", result.FailureHint)
		}
		return fmt.Errorf("run failed: %s", strings.TrimSpace(result.FailureReason))
	}
	if result.UploadErr != nil {
		fmt.Printf("run upload warning: %v\n", result.UploadErr)
	}
	return nil
}
