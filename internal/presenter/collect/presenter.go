package collect

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/identity"
	"github.com/inferLean/inferlean-main/cli/internal/intentresolver/interactive"
	"github.com/inferLean/inferlean-main/cli/internal/intentresolver/noninteractive"
	configstore "github.com/inferLean/inferlean-main/cli/internal/storage/configuration"
	"github.com/inferLean/inferlean-main/cli/internal/storage/observation"
	"github.com/inferLean/inferlean-main/cli/internal/storage/processio"
	runstore "github.com/inferLean/inferlean-main/cli/internal/storage/run"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	collectionui "github.com/inferLean/inferlean-main/cli/internal/ui/collection"
	intentui "github.com/inferLean/inferlean-main/cli/internal/ui/intent"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Options struct {
	Target           vllmdiscovery.Candidate
	CollectFor       time.Duration
	ScrapeEvery      time.Duration
	OutputPath       string
	CollectorVersion string
	WorkloadMode     string
	WorkloadTarget   string
	PrefixHeavy      *bool
	Multimodal       *bool
	MultimodalCache  *bool
	NoInteractive    bool
}

type Result struct {
	Artifact     contracts.RunArtifact
	ArtifactPath string
	RunDir       string
}

type Presenter struct {
	collectView collectionui.View
	intentView  intentui.View
	runStore    runstore.Store
	obsStore    observation.Store
	pioStore    processio.Store
	cfgStore    *configstore.Store
}

func NewPresenter(collectView collectionui.View, intentView intentui.View, cfgStore *configstore.Store) Presenter {
	return Presenter{
		collectView: collectView,
		intentView:  intentView,
		runStore:    runstore.NewStore(),
		obsStore:    observation.NewStore(),
		pioStore:    processio.NewStore(),
		cfgStore:    cfgStore,
	}
}

func (p Presenter) Run(ctx context.Context, opts Options) (Result, error) {
	if err := validateDurations(opts.CollectFor, opts.ScrapeEvery); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opts.CollectorVersion) == "" {
		opts.CollectorVersion = "dev"
	}
	cfg, err := p.cfgStore.Ensure()
	if err != nil {
		return Result{}, err
	}
	runID, err := identity.NewRunID()
	if err != nil {
		return Result{}, err
	}
	paths, err := p.runStore.Prepare(runID, opts.OutputPath)
	if err != nil {
		return Result{}, err
	}

	start := time.Now().UTC()
	evidence, err := p.collectEvidence(ctx, opts, paths)
	if err != nil {
		return Result{}, err
	}
	intent, err := p.resolveIntent(opts)
	if err != nil {
		return Result{}, err
	}

	artifact, err := buildArtifact(ctx, buildInput{
		RunID:            runID,
		InstallationID:   cfg.InstallationID,
		CollectorVersion: opts.CollectorVersion,
		StartedAt:        start,
		FinishedAt:       time.Now().UTC(),
		Target:           opts.Target,
		Intent:           intent,
		PromResult:       evidence.promResult,
		StaticNvidiaSMI:  evidence.staticSMI,
	})
	if err != nil {
		return Result{}, err
	}
	if err := p.runStore.SaveArtifact(paths.ArtifactPath, artifact); err != nil {
		return Result{}, err
	}
	p.collectView.ShowDone(paths.ArtifactPath)
	return Result{Artifact: artifact, ArtifactPath: paths.ArtifactPath, RunDir: paths.RunDir}, nil
}

type evidence struct {
	promResult promcollector.Result
	staticSMI  string
}

func (p Presenter) collectEvidence(ctx context.Context, opts Options, paths runstore.Paths) (evidence, error) {
	p.collectView.ShowStart(opts.CollectFor.Seconds())
	p.collectView.ShowStep("starting exporters and local bridges")
	sources := startSources(ctx)
	p.collectView.ShowStep("collecting metrics through prometheus scrape manager")
	targets := buildPromTargets(opts, sources)
	promRes := promcollector.NewCollector().CollectTargets(ctx, targets, opts.CollectFor, opts.ScrapeEvery)
	savePrometheusObservations(p, paths, promRes)
	p.collectView.ShowStep("collecting nvidia-smi process output")
	staticSMI := readStaticNvidiaSMI(ctx)
	if staticSMI != "" {
		_, _ = p.pioStore.Save(paths.ProcessIO, "nvidia-smi-static.txt", []byte(staticSMI))
	}
	bridgeRaw := stopSources(context.Background(), p, paths, sources)
	if strings.TrimSpace(bridgeRaw) != "" {
		_, _ = p.obsStore.SaveRaw(paths.Observations, "nvidia-smi.csv", []byte(bridgeRaw))
	}
	return evidence{
		promResult: promRes,
		staticSMI:  staticSMI,
	}, nil
}

func (p Presenter) resolveIntent(opts Options) (types.UserIntent, error) {
	intentSeed, _ := noninteractive.Resolve(noninteractive.Input{
		WorkloadMode:    opts.WorkloadMode,
		WorkloadTarget:  opts.WorkloadTarget,
		PrefixHeavy:     opts.PrefixHeavy,
		Multimodal:      opts.Multimodal,
		MultimodalCache: opts.MultimodalCache,
	})
	if opts.NoInteractive || hasCompleteIntent(opts, intentSeed) {
		p.intentView.ShowResolved(intentSeed)
		return intentSeed, nil
	}
	intent, err := interactive.Resolve(intentSeed)
	if err != nil {
		return types.UserIntent{}, err
	}
	p.intentView.ShowResolved(intent)
	return intent, nil
}

func hasCompleteIntent(opts Options, seed types.UserIntent) bool {
	return strings.TrimSpace(seed.WorkloadMode) != "" &&
		strings.TrimSpace(seed.WorkloadTarget) != "" &&
		opts.PrefixHeavy != nil &&
		opts.Multimodal != nil &&
		opts.MultimodalCache != nil
}

func validateDurations(collectFor, scrapeEvery time.Duration) error {
	if collectFor <= 0 {
		return fmt.Errorf("collect-for must be > 0")
	}
	if scrapeEvery <= 0 {
		return fmt.Errorf("scrape-every must be > 0")
	}
	if scrapeEvery > collectFor {
		return fmt.Errorf("scrape-every must be <= collect-for")
	}
	return nil
}

func readStaticNvidiaSMI(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "nvidia-smi")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}
