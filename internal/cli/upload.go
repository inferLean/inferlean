package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	reportpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/report"
	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
)

func newUploadCommand() *cobra.Command {
	opts := &UploadFlags{}
	cmd := &cobra.Command{
		Use:   "upload [artifact-path]",
		Short: "Upload an artifact or re-upload a local run artifact",
		Args: func(_ *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.RunID) != "" {
				if len(args) > 0 {
					return fmt.Errorf("artifact-path cannot be used with --run-id")
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("artifact-path is required unless --run-id is set")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			application := appFromContext(cmd.Context())
			artifactPath := ""
			if len(args) > 0 {
				artifactPath = args[0]
			}
			result, err := application.upload.Run(cmd.Context(), uploadpresenter.Options{
				BackendURL:    application.appURL,
				ArtifactPath:  artifactPath,
				RunID:         strings.TrimSpace(opts.RunID),
				RequireReport: opts.RequireReport,
			})
			if err != nil {
				return err
			}
			application.report.Run(reportpresenter.Options{
				BackendURL:     application.appURL,
				Payload:        result.Report,
				RunID:          result.RunID,
				InstallationID: result.InstallationID,
				NonInteractive: application.nonInteractive,
			})
			return nil
		},
	}
	bindUploadFlags(cmd, opts)
	return cmd
}

func bindUploadFlags(cmd *cobra.Command, opts *UploadFlags) {
	cmd.Flags().BoolVar(&opts.RequireReport, "require-report", false, "require report retrieval after upload")
	cmd.Flags().StringVar(&opts.RunID, "run-id", "", "re-upload the local artifact for an existing run id")
}
