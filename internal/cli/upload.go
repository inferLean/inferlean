package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
)

func newUploadCommand() *cobra.Command {
	backendURL := defaults.BackendURL
	var requireReport bool
	var runID string
	cmd := &cobra.Command{
		Use:   "upload [artifact-path]",
		Short: "Upload an artifact or load a report by run id",
		Args: func(_ *cobra.Command, args []string) error {
			if strings.TrimSpace(runID) != "" {
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
				BackendURL:    backendURL,
				ArtifactPath:  artifactPath,
				RunID:         strings.TrimSpace(runID),
				RequireReport: requireReport,
			})
			if err != nil {
				return err
			}
			if len(result.Report) > 0 {
				application.report.Run(result.Report)
			}
			if strings.TrimSpace(result.RunID) != "" {
				fmt.Printf("run_id: %s\n", result.RunID)
				fmt.Printf("view again: inferlean upload --run-id %s\n", result.RunID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&backendURL, "backend-url", defaults.BackendURL, "backend base URL")
	cmd.Flags().BoolVar(&requireReport, "require-report", false, "require report retrieval after upload")
	cmd.Flags().StringVar(&runID, "run-id", "", "load and render report for an existing run id")
	return cmd
}
