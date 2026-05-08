package artifactnormalize

import (
	"testing"
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestMemoryWindowsDerivesFreeMemoryFromAlignedSamples(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	used := withSamples([]contracts.MetricSample{
		{Timestamp: now, Value: 40},
		{Timestamp: now.Add(time.Second), Value: 50},
	})
	total := withSamples([]contracts.MetricSample{
		{Timestamp: now, Value: 100},
		{Timestamp: now.Add(time.Second), Value: 100},
	})

	memory := memoryWindows(used, contracts.MetricWindow{}, contracts.MetricWindow{}, total)

	if got, want := len(memory.Free.Samples), 2; got != want {
		t.Fatalf("free sample count = %d, want %d", got, want)
	}
	if got, want := memory.Free.Samples[0].Timestamp, now; !got.Equal(want) {
		t.Fatalf("free timestamp = %s, want %s", got, want)
	}
	if got, want := memory.Free.Samples[0].Value, 60.0; got != want {
		t.Fatalf("free value = %v, want %v", got, want)
	}
	if got, want := *memory.Free.Latest, 50.0; got != want {
		t.Fatalf("free latest = %v, want %v", got, want)
	}
}

func TestMemoryWindowsSkipsUnalignedAndNegativeFreeSamples(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	used := withSamples([]contracts.MetricSample{
		{Timestamp: now, Value: 120},
		{Timestamp: now.Add(time.Second), Value: 50},
	})
	total := withSamples([]contracts.MetricSample{
		{Timestamp: now, Value: 100},
		{Timestamp: now.Add(2 * time.Second), Value: 100},
	})

	memory := memoryWindows(used, contracts.MetricWindow{}, contracts.MetricWindow{}, total)

	if memory.Free.HasData() {
		t.Fatalf("free memory should be empty for negative or unaligned samples: %+v", memory.Free)
	}
}
