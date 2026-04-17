package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
)

func newUploadCommand() *cobra.Command {
	var backendURL string
	var requireReport bool
	cmd := &cobra.Command{
		Use:   "upload <artifact-path>",
		Short: "Upload an artifact to backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			application := appFromContext(cmd.Context())
			result, err := application.upload.Run(cmd.Context(), uploadpresenter.Options{
				BackendURL:    backendURL,
				ArtifactPath:  args[0],
				RequireReport: requireReport,
			})
			if err != nil {
				return err
			}
			fmt.Printf("upload accepted: run_id=%s upload_id=%s\n", result.Ack.RunID, result.Ack.UploadID)
			if len(result.Report) > 0 {
				application.report.Run(result.Report)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&backendURL, "backend-url", "", "backend base URL")
	cmd.Flags().BoolVar(&requireReport, "require-report", false, "require report retrieval after upload")
	return cmd
}
