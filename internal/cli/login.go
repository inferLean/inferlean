package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/auth"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/events"
)

func newLoginCommand() *cobra.Command {
	var backendURL string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in through the configured Dex identity provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore()
			if err != nil {
				return err
			}
			cfg, err := store.Ensure()
			if err != nil {
				return err
			}

			baseURL, err := resolveBackendURL(backendURL, cfg.Auth)
			if err != nil {
				return err
			}

			manager := auth.NewManager()
			session, err := manager.Login(cmd.Context(), baseURL, func(url string) {
				fmt.Fprintf(cmd.OutOrStdout(), "Open this URL if your browser does not launch automatically:\n  %s\n\n", url)
			})
			if err != nil {
				return err
			}

			claim, session, err := manager.ClaimInstallation(cmd.Context(), session, cfg.InstallationID)
			if err != nil {
				return err
			}

			cfg.Auth = &session
			if err := store.Save(cfg); err != nil {
				return err
			}
			if emitter, err := events.NewEmitter(); err == nil {
				_ = emitter.Flush(cmd.Context(), session.BackendURL, session)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "InferLean login complete.")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "  Backend: %s\n", session.BackendURL)
			fmt.Fprintf(cmd.OutOrStdout(), "  Issuer: %s\n", session.Issuer)
			fmt.Fprintf(cmd.OutOrStdout(), "  Installation ID: %s\n", cfg.InstallationID)
			fmt.Fprintf(cmd.OutOrStdout(), "  Assigned previous runs: %d\n", claim.AssignedRunCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&backendURL, "backend-url", "", "InferLean backend base URL")

	return cmd
}
