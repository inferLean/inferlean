package collect

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
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
	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Options struct {
	Target                  vllmdiscovery.Candidate
	CollectFor              time.Duration
	ScrapeEvery             time.Duration
	OutputPath              string
	CollectorVersion        string
	DeclaredWorkloadMode    string
	DeclaredWorkloadTarget  string
	PrefixHeavy             *bool
	Multimodal              *bool
	RepeatedMultimodalMedia *bool
	NoInteractive           bool
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
	progressDone := false
	defer func() {
		if !progressDone {
			p.collectView.Abort()
		}
	}()
	p.collectView.SetNoInteractive(opts.NoInteractive)
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

	intent, err := p.resolveIntent(opts)
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
		Target:           opts.Target,
		Intent:           intent,
		PromResult:       evidence.promResult,
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
	staticSMI  string
}

func (p Presenter) collectEvidence(ctx context.Context, opts Options, paths runstore.Paths) (evidence, error) {
	interactive := interactiveCollectionEnabled(opts.NoInteractive)
	collectCtx, cancelCollect := context.WithCancel(ctx)
	defer cancelCollect()

	p.collectView.ShowStart(opts.CollectFor.Seconds())
	actions, interrupt, stopListening := startCollectionDurationListener(interactive)
	defer stopListening()
	var interrupted atomic.Bool
	doneInterrupt := make(chan struct{})
	go func() {
		defer close(doneInterrupt)
		select {
		case <-interrupt:
			interrupted.Store(true)
			cancelCollect()
		case <-collectCtx.Done():
		}
	}()
	defer func() {
		cancelCollect()
		<-doneInterrupt
	}()

	p.collectView.ShowStep("starting exporters and local bridges")
	sources := startSources(collectCtx)
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
	if interrupted.Load() {
		return evidence{}, fmt.Errorf("collection interrupted")
	}
	p.collectView.ShowMetricsCollectionStart(opts.CollectFor)

	targets := buildPromTargets(opts, sources)
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
	savePrometheusObservations(p, paths, promRes)
	if interrupted.Load() {
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
		staticSMI:  staticSMI,
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

func interactiveCollectionEnabled(noInteractive bool) bool {
	return progress.InteractiveTTY() && !noInteractive
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
	return strings.TrimSpace(seed.DeclaredWorkloadMode) != "" &&
		strings.TrimSpace(seed.DeclaredWorkloadTarget) != "" &&
		opts.PrefixHeavy != nil &&
		opts.Multimodal != nil &&
		opts.RepeatedMultimodalMedia != nil
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
