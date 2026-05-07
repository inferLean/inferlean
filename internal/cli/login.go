package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login via browser",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			cfg, err := application.cfgStore.Ensure()
			if err != nil {
				return err
			}
			authState, err := application.auth.Login(cmd.Context(), application.backendURL)
			if err != nil {
				return err
			}
			cfg.Auth = authState
			if err := application.cfgStore.Save(cfg); err != nil {
				return err
			}
			fmt.Println("login succeeded")
			return nil
		},
	}
	return cmd
}
