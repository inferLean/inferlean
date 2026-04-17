package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLoginCommand() *cobra.Command {
	var backendURL string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login via browser OIDC flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			application := appFromContext(cmd.Context())
			cfg, err := application.cfgStore.Ensure()
			if err != nil {
				return err
			}
			authState, err := application.auth.Login(cmd.Context(), backendURL)
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
	cmd.Flags().StringVar(&backendURL, "backend-url", "", "backend base URL")
	_ = cmd.MarkFlagRequired("backend-url")
	return cmd
}
