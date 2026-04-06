package cli

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/auth"
	"github.com/inferLean/inferlean/internal/collector"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/internal/events"
)

type scanOptions struct {
	PID            int32
	NoInteractive  bool
	CollectFor     time.Duration
	ScrapeEvery    time.Duration
	WorkloadMode   string
	WorkloadTarget string
	OutputPath     string
	BackendURL     string
}

func newScanCommand() *cobra.Command {
	var pid int32
	var noInteractive bool
	var collectFor time.Duration
	var scrapeEvery time.Duration
	var workloadMode string
	var workloadTarget string
	var outputPath string
	var backendURL string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run an authenticated end-to-end InferLean scan and open the report",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, scanOptions{
				PID:            pid,
				NoInteractive:  noInteractive,
				CollectFor:     collectFor,
				ScrapeEvery:    scrapeEvery,
				WorkloadMode:   workloadMode,
				WorkloadTarget: workloadTarget,
				OutputPath:     outputPath,
				BackendURL:     backendURL,
			})
		},
	}

	cmd.Flags().Int32Var(&pid, "pid", 0, "select a specific vLLM process by pid")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "disable interactive target selection and report rendering")
	cmd.Flags().DurationVar(&collectFor, "collect-for", defaultCollectFor, "how long to collect metrics before building the artifact")
	cmd.Flags().DurationVar(&collectFor, "collect-interval", defaultCollectFor, "alias for --collect-for")
	cmd.Flags().DurationVar(&scrapeEvery, "scrape-every", defaultScrapeEvery, "how often Prometheus scrapes configured targets during collection")
	cmd.Flags().StringVar(&workloadMode, "workload-mode", "", "workload mode for this run: realtime_chat, batch_processing, or mixed")
	cmd.Flags().StringVar(&workloadTarget, "workload-target", "", "optimization target for this run: latency, balanced, or throughput")
	cmd.Flags().StringVar(&outputPath, "output", "", "write the artifact to a specific path")
	cmd.Flags().StringVar(&backendURL, "backend-url", "", "InferLean backend base URL")
	_ = cmd.Flags().MarkHidden("collect-interval")

	return cmd
}

func runScan(cmd *cobra.Command, opts scanOptions) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("scan requires Linux collection support; use inferlean collect for local-only artifact capture on supported hosts")
	}

	workloadMode, workloadTarget, err := normalizeScanWorkload(opts)
	if err != nil {
		return err
	}

	interactive := isInteractiveTerminal(opts.NoInteractive)
	store, cfg, baseURL, session, err := prepareScanSession(cmd, opts, interactive)
	if err != nil {
		return err
	}
	target, err := resolveTarget(cmd, targetResolutionOptions{PID: opts.PID, NoInteractive: opts.NoInteractive})
	if err != nil {
		return err
	}
	collectResult, err := runScanCollection(cmd, opts, target, workloadMode, workloadTarget, interactive)
	if err != nil {
		return err
	}
	publishResult, err := runScanPublish(cmd, baseURL, session, collectResult, interactive)
	if err != nil {
		return err
	}
	return finalizeScan(cmd, store, cfg, baseURL, target, collectResult, publishResult, interactive)
}

func ensureScanSession(
	cmd *cobra.Command,
	store *config.Store,
	cfg config.Config,
	baseURL string,
	interactive bool,
) (config.AuthState, error) {
	manager := auth.NewManager()

	if cfg.Auth != nil && cfg.Auth.HasSession() {
		session := *cfg.Auth
		session.BackendURL = baseURL
		updated, err := manager.EnsureValid(cmd.Context(), session)
		if err == nil {
			cfg.Auth = &updated
			if err := store.Save(cfg); err != nil {
				return config.AuthState{}, err
			}
			return updated, nil
		}
		if !interactive {
			return config.AuthState{}, fmt.Errorf(
				"saved login is not usable non-interactively: %w; run inferlean login first or use inferlean collect for local-only work",
				err,
			)
		}
	}

	if !interactive {
		return config.AuthState{}, fmt.Errorf("login required; run inferlean login first or use inferlean collect for local-only work")
	}

	return loginAndClaimScanSession(cmd, manager, store, cfg, baseURL)
}

func loginAndClaimScanSession(
	cmd *cobra.Command,
	manager *auth.Manager,
	store *config.Store,
	cfg config.Config,
	baseURL string,
) (config.AuthState, error) {
	session, err := manager.Login(cmd.Context(), baseURL, func(url string) {
		fmt.Fprintf(cmd.OutOrStdout(), "Open this URL if your browser does not launch automatically:\n  %s\n\n", url)
	})
	if err != nil {
		return config.AuthState{}, err
	}

	claim, updated, err := manager.ClaimInstallation(cmd.Context(), session, cfg.InstallationID)
	if err != nil {
		return config.AuthState{}, err
	}

	cfg.Auth = &updated
	if err := store.Save(cfg); err != nil {
		return config.AuthState{}, err
	}
	if emitter, err := events.NewEmitter(); err == nil {
		_ = emitter.Flush(cmd.Context(), updated.BackendURL, updated)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "InferLean authenticated installation %s and linked %d previous runs.\n\n", cfg.InstallationID, claim.AssignedRunCount)
	return updated, nil
}

func normalizeScanWorkload(opts scanOptions) (string, string, error) {
	if err := collector.ValidateDurations(opts.CollectFor, opts.ScrapeEvery); err != nil {
		return "", "", err
	}
	workloadMode, err := collector.NormalizeWorkloadMode(opts.WorkloadMode)
	if err != nil {
		return "", "", err
	}
	workloadTarget, err := collector.NormalizeWorkloadTarget(opts.WorkloadTarget)
	if err != nil {
		return "", "", err
	}
	return workloadMode, workloadTarget, nil
}

func prepareScanSession(
	cmd *cobra.Command,
	opts scanOptions,
	interactive bool,
) (*config.Store, config.Config, string, config.AuthState, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, config.Config{}, "", config.AuthState{}, err
	}
	cfg, err := store.Ensure()
	if err != nil {
		return nil, config.Config{}, "", config.AuthState{}, err
	}
	baseURL, err := resolveBackendURL(opts.BackendURL, cfg.Auth)
	if err != nil {
		return nil, config.Config{}, "", config.AuthState{}, err
	}
	session, err := ensureScanSession(cmd, store, cfg, baseURL, interactive)
	if err != nil {
		return nil, config.Config{}, "", config.AuthState{}, err
	}
	return store, cfg, baseURL, session, nil
}
