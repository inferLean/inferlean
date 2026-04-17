package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear local auth session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			cfg, err := application.cfgStore.Ensure()
			if err != nil {
				return err
			}
			cfg.Auth = cfg.Auth
			cfg.Auth.AccessToken = ""
			cfg.Auth.IDToken = ""
			cfg.Auth.RefreshToken = ""
			cfg.Auth.TokenType = ""
			cfg.Auth.ExpiresAt = cfg.Auth.ExpiresAt.AddDate(-100, 0, 0)
			if err := application.cfgStore.Save(cfg); err != nil {
				return err
			}
			fmt.Println("logout succeeded")
			return nil
		},
	}
	return cmd
}
