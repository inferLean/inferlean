//go:build !linux

package collector

import (
	"context"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func collectNVMLSamples(_ context.Context, _ time.Duration, _ string) (*nvmlSnapshot, contracts.SourceCoverage, error) {
	return nil, contracts.SourceCoverage{}, nil
}
