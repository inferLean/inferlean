package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean-main/cli/internal/api"
	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	"github.com/inferLean/inferlean-main/cli/internal/interrupt"
	"github.com/inferLean/inferlean-main/cli/internal/logging"
	collectpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/collect"
	discoverpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/discover"
	reportpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/report"
	runpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/run"
	uploadpresenter "github.com/inferLean/inferlean-main/cli/internal/presenter/upload"
	configstore "github.com/inferLean/inferlean-main/cli/internal/storage/configuration"
	collectionui "github.com/inferLean/inferlean-main/cli/internal/ui/collection"
	discoveryui "github.com/inferLean/inferlean-main/cli/internal/ui/discovery"
	intentui "github.com/inferLean/inferlean-main/cli/internal/ui/intent"
	reportui "github.com/inferLean/inferlean-main/cli/internal/ui/report"
	uploadui "github.com/inferLean/inferlean-main/cli/internal/ui/upload"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

var version = "dev"

type rootOptions struct {
	backendURL     string
	debug          bool
	debugFile      string
	nonInteractive bool
}

type app struct {
	backendURL     string
	nonInteractive bool
	cfgStore       *configstore.Store
	discoverySvc   vllmdiscovery.Service
	interrupts     *interrupt.Bus
	discover       discoverpresenter.Presenter
	collect        collectpresenter.Presenter
	upload         uploadpresenter.Presenter
	report         reportpresenter.Presenter
	run            runpresenter.Presenter
	auth           api.AuthManager
	logger         *logging.Logger
	closeLoggerFn  func() error
}

func Execute() error {
	ctx := context.Background()
	return newRootCommand(ctx).Execute()
}

func newRootCommand(ctx context.Context) *cobra.Command {
	opts := &rootOptions{}
	runFlags := &runFlags{}
	cmd := &cobra.Command{
		Use:           "inferlean",
		Short:         runShort,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithOptions(cmd, runFlags.options())
		},
	}
	cmd.SetContext(ctx)
	cmd.PersistentFlags().StringVar(&opts.backendURL, "backend-url", defaults.AppBaseURL, "backend base URL")
	cmd.PersistentFlags().StringVar(&opts.backendURL, "app-url", defaults.AppBaseURL, "deprecated alias for --backend-url")
	_ = cmd.PersistentFlags().MarkDeprecated("app-url", "use --backend-url instead")
	cmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "show debug output")
	cmd.PersistentFlags().StringVar(&opts.debugFile, "debug-file", "", "write debug output to a file")
	cmd.PersistentFlags().BoolVar(&opts.nonInteractive, "non-interactive", false, "disable interactive prompts and viewers")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		maybePrintHeader(cmd, opts.nonInteractive)
		application, err := newApp(cmd.Context(), opts)
		if err != nil {
			return err
		}
		cmd.SetContext(context.WithValue(application.interrupts.Context(), appKey{}, application))
		return nil
	}
	cmd.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
		application := appFromContext(cmd.Context())
		if application.interrupts != nil {
			application.interrupts.Close()
		}
		if application.closeLoggerFn != nil {
			_ = application.closeLoggerFn()
		}
	}

	bindRunFlags(cmd, runFlags)

	cmd.AddCommand(newDiscoverCommand())
	cmd.AddCommand(newCollectCommand())
	cmd.AddCommand(newUploadCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newLoginCommand())
	cmd.AddCommand(newLogoutCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}

func newApp(ctx context.Context, opts *rootOptions) (app, error) {
	cfgStore, err := configstore.NewStore()
	if err != nil {
		return app{}, err
	}
	logger, closeFn, err := logging.New(opts.debug, opts.debugFile)
	if err != nil {
		return app{}, err
	}
	discoverySvc := vllmdiscovery.NewService()
	interrupts := interrupt.NewBus(ctx)
	discoverPresenter := discoverpresenter.NewPresenter(discoverySvc, discoveryui.NewView(), interrupts)
	collectPresenter := collectpresenter.NewPresenter(collectionui.NewView(), intentui.NewView(), cfgStore, interrupts)
	uploadPresenter := uploadpresenter.NewPresenter(cfgStore, uploadui.NewView())
	reportPresenter := reportpresenter.NewPresenter(reportui.NewView())
	runPresenter := runpresenter.NewPresenter(discoverPresenter, collectPresenter, uploadPresenter, reportPresenter)
	return app{
		backendURL:     opts.backendURL,
		nonInteractive: opts.nonInteractive,
		cfgStore:       cfgStore,
		discoverySvc:   discoverySvc,
		interrupts:     interrupts,
		discover:       discoverPresenter,
		collect:        collectPresenter,
		upload:         uploadPresenter,
		report:         reportPresenter,
		run:            runPresenter,
		auth:           api.NewAuthManager(),
		logger:         logger,
		closeLoggerFn:  closeFn,
	}, nil
}

type appKey struct{}

func appFromContext(ctx context.Context) app {
	value, ok := ctx.Value(appKey{}).(app)
	if !ok {
		panic("app not initialized")
	}
	return value
}

func parseOptionalBool(value string) (*bool, error) {
	switch value {
	case "", "auto":
		return nil, nil
	case "true", "1", "yes", "y":
		v := true
		return &v, nil
	case "false", "0", "no", "n":
		v := false
		return &v, nil
	default:
		return nil, fmt.Errorf("invalid bool value %q, use true/false/auto", value)
	}
}
