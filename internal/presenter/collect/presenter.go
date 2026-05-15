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
	"github.com/inferLean/inferlean-main/cli/internal/interrupt"
	configstore "github.com/inferLean/inferlean-main/cli/internal/storage/configuration"
	"github.com/inferLean/inferlean-main/cli/internal/storage/observation"
	"github.com/inferLean/inferlean-main/cli/internal/storage/processio"
	runstore "github.com/inferLean/inferlean-main/cli/internal/storage/run"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	collectionui "github.com/inferLean/inferlean-main/cli/internal/ui/collection"
	intentui "github.com/inferLean/inferlean-main/cli/internal/ui/intent"
	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Options struct {
	Target                  vllmdiscovery.Candidate
	CollectFor              time.Duration
	ScrapeEvery             time.Duration
	OutputPath              string
	DCGMEndpoint            string
	AllowDCGMEstimation     bool
	CollectorVersion        string
	DeclaredWorkloadMode    string
	DeclaredWorkloadTarget  string
	PrefixHeavy             *bool
	Multimodal              *bool
	RepeatedMultimodalMedia *bool
	NonInteractive          bool
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
	interrupts  interrupt.Publisher
}

const (
	collectionAdjustShortStep     = 15 * time.Second
	collectionAdjustLongStep      = time.Minute
	minInteractiveCollectDuration = time.Second
	maxInteractiveCollectDuration = 24 * time.Hour
)

type collectionDurationAction int

const (
	collectionActionUnknown collectionDurationAction = iota
	collectionActionIncreaseMinute
	collectionActionDecreaseMinute
	collectionActionIncreaseSeconds
	collectionActionDecreaseSeconds
	collectionActionStopAndAnalyze
)

func NewPresenter(collectView collectionui.View, intentView intentui.View, cfgStore *configstore.Store, interrupts interrupt.Publisher) Presenter {
	return Presenter{
		collectView: collectView,
		intentView:  intentView,
		runStore:    runstore.NewStore(),
		obsStore:    observation.NewStore(),
		pioStore:    processio.NewStore(),
		cfgStore:    cfgStore,
		interrupts:  interrupts,
	}
}

func (p Presenter) Run(ctx context.Context, opts Options) (Result, error) {
	progressDone := false
	defer func() {
		if !progressDone {
			p.collectView.Abort()
		}
	}()
	p.collectView.SetNonInteractive(opts.NonInteractive)
	if err := validateDurations(opts.CollectFor, opts.ScrapeEvery); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opts.CollectorVersion) == "" {
		opts.CollectorVersion = "dev"
	}
	intent, err := p.resolveIntent(opts)
	if err != nil {
		return Result{}, err
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

	artifact, err := buildArtifact(ctx, buildInput{
		RunID:            runID,
		InstallationID:   cfg.InstallationID,
		CollectorVersion: opts.CollectorVersion,
		StartedAt:        start,
		FinishedAt:       time.Now().UTC(),
		Target:           evidence.target,
		Intent:           intent,
		PromResult:       evidence.promResult,
		Sources:          evidence.sources,
		StaticNvidiaSMI:  evidence.staticSMI,
		ProcessIODir:     paths.ProcessIO,
	})
	if err != nil {
		return Result{}, err
	}
	if err := p.runStore.SaveArtifact(paths.ArtifactPath, artifact); err != nil {
		return Result{}, err
	}
	p.collectView.ShowDone(runID)
	progressDone = true
	return Result{Artifact: artifact, ArtifactPath: paths.ArtifactPath, RunDir: paths.RunDir}, nil
}

type evidence struct {
	promResult promcollector.Result
	sources    collectionSources
	staticSMI  string
	target     vllmdiscovery.Candidate
}

func (p Presenter) collectEvidence(ctx context.Context, opts Options, paths runstore.Paths) (evidence, error) {
	interactive := interactiveCollectionEnabled(opts.NonInteractive)
	collectCtx, cancelCollect := context.WithCancel(ctx)
	defer cancelCollect()

	p.collectView.ShowStart(opts.CollectFor.Seconds())
	actions, stopListening := startCollectionDurationListener(interactive, p.interrupts)
	defer stopListening()

	p.collectView.ShowStep("starting exporters and local bridges")
	sources, err := startSources(collectCtx, opts)
	if err != nil {
		return evidence{}, err
	}
	stoppedSources := false
	stopAllSources := func() {
		if stoppedSources {
			return
		}
		stoppedSources = true
		bridgeRaw := stopSources(context.Background(), p, paths, sources)
		if strings.TrimSpace(bridgeRaw) != "" {
			_, _ = p.obsStore.SaveRaw(paths.Observations, "nvidia-smi.csv", []byte(bridgeRaw))
		}
	}
	defer stopAllSources()
	if err := requireDCGMSource(opts, sources); err != nil {
		return evidence{}, err
	}
	if ctx.Err() != nil {
		return evidence{}, fmt.Errorf("collection interrupted")
	}
	if !opts.AllowDCGMEstimation {
		p.collectView.ShowStep("checking dcgm-exporter profiler metrics")
	}
	if err := requireDCGMPreflight(collectCtx, opts, sources); err != nil {
		return evidence{}, err
	}
	p.collectView.ShowMetricsCollectionStart(opts.CollectFor)

	targets := buildPromTargets(sources)
	stopCountdown := p.startCollectionCountdown(
		collectCtx,
		opts.CollectFor,
		collectorDurationWindow(opts.CollectFor, interactive),
		actions,
		cancelCollect,
	)
	promRes := promcollector.NewCollector().CollectTargets(
		collectCtx,
		targets,
		collectorDurationWindow(opts.CollectFor, interactive),
		opts.ScrapeEvery,
	)
	close(stopCountdown)
	stopListening()
	savePrometheusObservations(p, paths, promRes)
	if err := requireDCGMMetrics(opts, promRes); err != nil {
		return evidence{}, err
	}
	if ctx.Err() != nil {
		return evidence{}, fmt.Errorf("collection interrupted")
	}
	p.collectView.ShowStep("collecting nvidia-smi process output")
	staticSMI := readStaticNvidiaSMI(ctx)
	if staticSMI != "" {
		_, _ = p.pioStore.Save(paths.ProcessIO, "nvidia-smi-static.txt", []byte(staticSMI))
	}
	stopAllSources()
	return evidence{
		promResult: promRes,
		sources:    sources,
		staticSMI:  staticSMI,
		target:     opts.Target,
	}, nil
}

func (p Presenter) startCollectionCountdown(
	ctx context.Context,
	collectFor time.Duration,
	maxDuration time.Duration,
	actions <-chan collectionDurationAction,
	cancelCollect context.CancelFunc,
) chan struct{} {
	stop := make(chan struct{})
	go func() {
		started := time.Now()
		deadline := started.Add(collectFor)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case action := <-actions:
				nextDeadline, stopNow, changed := applyCollectionDurationAction(deadline, started, time.Now(), action, maxDuration)
				if stopNow {
					p.collectView.ShowMetricsCollectionStopped()
					cancelCollect()
					return
				}
				if !changed {
					continue
				}
				deadline = nextDeadline
				remaining := time.Until(deadline)
				if remaining <= 0 {
					cancelCollect()
					return
				}
				p.collectView.ShowMetricsCollectionCountdown(remaining)
			case <-ticker.C:
				remaining := time.Until(deadline)
				if remaining <= 0 {
					cancelCollect()
					return
				}
				p.collectView.ShowMetricsCollectionCountdown(remaining)
			}
		}
	}()
	return stop
}

func interactiveCollectionEnabled(nonInteractive bool) bool {
	return progress.InteractiveTTY() && !nonInteractive
}

func collectorDurationWindow(collectFor time.Duration, interactive bool) time.Duration {
	if !interactive {
		return collectFor
	}
	if collectFor > maxInteractiveCollectDuration {
		return collectFor
	}
	return maxInteractiveCollectDuration
}

func applyCollectionDurationAction(
	deadline, started, now time.Time,
	action collectionDurationAction,
	maxDuration time.Duration,
) (time.Time, bool, bool) {
	nextDeadline := deadline
	switch action {
	case collectionActionIncreaseMinute:
		nextDeadline = nextDeadline.Add(collectionAdjustLongStep)
	case collectionActionDecreaseMinute:
		nextDeadline = nextDeadline.Add(-collectionAdjustLongStep)
	case collectionActionIncreaseSeconds:
		nextDeadline = nextDeadline.Add(collectionAdjustShortStep)
	case collectionActionDecreaseSeconds:
		nextDeadline = nextDeadline.Add(-collectionAdjustShortStep)
	case collectionActionStopAndAnalyze:
		return deadline, true, false
	default:
		return deadline, false, false
	}
	minDeadline := started.Add(minInteractiveCollectDuration)
	if nextDeadline.Before(minDeadline) {
		nextDeadline = minDeadline
	}
	maxDeadline := started.Add(maxDuration)
	if nextDeadline.After(maxDeadline) {
		nextDeadline = maxDeadline
	}
	if !nextDeadline.After(now) {
		return nextDeadline, false, true
	}
	return nextDeadline, false, true
}

func (p Presenter) resolveIntent(opts Options) (types.UserIntent, error) {
	intentSeed, _ := noninteractive.Resolve(noninteractive.Input{
		DeclaredWorkloadMode:    opts.DeclaredWorkloadMode,
		DeclaredWorkloadTarget:  opts.DeclaredWorkloadTarget,
		PrefixHeavy:             opts.PrefixHeavy,
		Multimodal:              opts.Multimodal,
		RepeatedMultimodalMedia: opts.RepeatedMultimodalMedia,
	})
	if opts.NonInteractive {
		if err := requireCompleteIntent(opts, intentSeed); err != nil {
			return types.UserIntent{}, err
		}
		p.intentView.ShowResolved(intentSeed)
		return intentSeed, nil
	}
	if hasCompleteIntent(opts, intentSeed) {
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

func requireCompleteIntent(opts Options, seed types.UserIntent) error {
	missing := missingIntentFields(opts, seed)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("non-interactive collection requires explicit user intent flags: %s", strings.Join(missing, ", "))
}

func missingIntentFields(opts Options, seed types.UserIntent) []string {
	missing := []string{}
	if strings.TrimSpace(seed.DeclaredWorkloadMode) == "" {
		missing = append(missing, "--workload-mode")
	}
	if strings.TrimSpace(seed.DeclaredWorkloadTarget) == "" {
		missing = append(missing, "--workload-target")
	}
	if opts.PrefixHeavy == nil {
		missing = append(missing, "--prefix-heavy")
	}
	if opts.Multimodal == nil {
		missing = append(missing, "--multimodal")
	}
	if opts.RepeatedMultimodalMedia == nil {
		missing = append(missing, "--repeated-multimodal-media")
	}
	return missing
}

func hasCompleteIntent(opts Options, seed types.UserIntent) bool {
	return len(missingIntentFields(opts, seed)) == 0
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
