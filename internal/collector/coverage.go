package collector

import (
	"sort"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type coverageBuilder struct {
	rawEvidenceRef string
	present        map[string]struct{}
	missing        map[string]struct{}
	unsupported    map[string]struct{}
	derived        map[string]struct{}
}

func newCoverageBuilder(rawEvidenceRef string) *coverageBuilder {
	return &coverageBuilder{
		rawEvidenceRef: rawEvidenceRef,
		present:        map[string]struct{}{},
		missing:        map[string]struct{}{},
		unsupported:    map[string]struct{}{},
		derived:        map[string]struct{}{},
	}
}

func (b *coverageBuilder) Present(name string) {
	delete(b.missing, name)
	delete(b.unsupported, name)
	b.present[name] = struct{}{}
}

func (b *coverageBuilder) Missing(name string) {
	if !b.has(name) {
		b.missing[name] = struct{}{}
	}
}

func (b *coverageBuilder) Unsupported(name string) {
	if !b.has(name) {
		b.unsupported[name] = struct{}{}
	}
}

func (b *coverageBuilder) Derived(name string) {
	b.derived[name] = struct{}{}
}

func (b *coverageBuilder) has(name string) bool {
	_, present := b.present[name]
	_, missing := b.missing[name]
	_, unsupported := b.unsupported[name]
	return present || missing || unsupported
}

func (b *coverageBuilder) Build() contracts.SourceCoverage {
	return contracts.SourceCoverage{
		PresentFields:     sortedSetKeys(b.present),
		MissingFields:     sortedSetKeys(b.missing),
		UnsupportedFields: sortedSetKeys(b.unsupported),
		DerivedFields:     sortedSetKeys(b.derived),
		RawEvidenceRef:    b.rawEvidenceRef,
	}
}

func (b *coverageBuilder) presentNames() []string {
	return sortedSetKeys(b.present)
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func captureFromCoverage(coverage contracts.SourceCoverage, artifacts []string, reason string, required []string) sourceCapture {
	switch {
	case len(coverage.PresentFields) == 0:
		return sourceCapture{Status: "missing", Reason: reason, Artifacts: artifacts}
	case hasMissingRequired(coverage, required):
		return sourceCapture{Status: "degraded", Reason: reason, Artifacts: artifacts}
	default:
		return sourceCapture{Status: "ok", Artifacts: artifacts}
	}
}

func hasMissingRequired(coverage contracts.SourceCoverage, required []string) bool {
	for _, name := range required {
		if containsCoverageName(coverage.UnsupportedFields, name) {
			continue
		}
		if containsCoverageName(coverage.MissingFields, name) {
			return true
		}
		if !containsCoverageName(coverage.PresentFields, name) {
			return true
		}
	}
	return false
}

func containsCoverageName(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
