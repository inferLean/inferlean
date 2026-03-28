package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/runs"
	"github.com/inferLean/inferlean/internal/ui/runbrowser"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func newRunsCommand() *cobra.Command {
	var (
		backendURL    string
		noInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List runs assigned to the logged-in user",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := config.NewStore()
			if err != nil {
				return err
			}
			cfg, err := store.Ensure()
			if err != nil {
				return err
			}
			if cfg.Auth == nil || !cfg.Auth.HasSession() {
				return fmt.Errorf("login required to list runs; run inferlean login first")
			}

			baseURL, err := resolveBackendURL(backendURL, cfg.Auth)
			if err != nil {
				return err
			}

			service := runs.NewService()
			session := *cfg.Auth
			runList, updatedSession, err := service.List(cmd.Context(), baseURL, session)
			if err != nil {
				return err
			}
			cfg.Auth = &updatedSession

			if err := store.Save(cfg); err != nil {
				return err
			}

			sort.Slice(runList, func(i, j int) bool {
				return runList[i].ReceivedAt.After(runList[j].ReceivedAt)
			})

			if !isInteractiveTerminal(noInteractive) {
				renderRunList(cmd, runList)
				return nil
			}

			err = runbrowser.Browse(runList, func(runID string) (contracts.RunDetailResponse, error) {
				detail, nextSession, err := service.Get(cmd.Context(), baseURL, runID, *cfg.Auth)
				if err != nil {
					return contracts.RunDetailResponse{}, err
				}
				cfg.Auth = &nextSession
				_ = store.Save(cfg)
				return detail, nil
			})
			if err != nil {
				return err
			}

			if err := store.Save(cfg); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&backendURL, "backend-url", "", "InferLean backend base URL")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "disable the interactive run browser")

	return cmd
}

func renderRunList(cmd *cobra.Command, runs []contracts.RunSummary) {
	if len(runs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No runs found.")
		return
	}

	for _, run := range runs {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n",
			run.RunID,
			run.ReceivedAt.Local().Format("2006-01-02 15:04:05"),
			run.InstallationID,
			run.CollectorVersion,
		)
	}
}
