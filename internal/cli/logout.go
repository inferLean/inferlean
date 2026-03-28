package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/config"
)

func newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved InferLean login session",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore()
			if err != nil {
				return err
			}
			cfg, err := store.Ensure()
			if err != nil {
				return err
			}

			if cfg.Auth == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "InferLean is already logged out.")
				return nil
			}

			cfg.Auth = nil
			if err := store.Save(cfg); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "InferLean login session cleared.")
			fmt.Fprintf(cmd.OutOrStdout(), "  Installation ID: %s\n", cfg.InstallationID)
			return nil
		},
	}
}
