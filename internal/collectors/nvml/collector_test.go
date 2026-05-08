package nvml

import "testing"

func TestParseSampleIncludesClocks(t *testing.T) {
	sample, ok := parseSample("0, 12, 4749, 6144, 80.5, 55, 1400, 7001, 90, P0, Not Active")
	if !ok {
		t.Fatal("parseSample() ok = false")
	}
	if sample.SMClock != 1400 {
		t.Fatalf("SMClock = %f, want 1400", sample.SMClock)
	}
	if sample.MemoryClock != 7001 {
		t.Fatalf("MemoryClock = %f, want 7001", sample.MemoryClock)
	}
	if sample.PowerLimit == nil || *sample.PowerLimit != 90 {
		t.Fatalf("PowerLimit = %v, want 90", sample.PowerLimit)
	}
	if sample.PState != "P0" {
		t.Fatalf("PState = %q, want P0", sample.PState)
	}
	if got := throttleReasons(sample.Throttle); len(got) != 1 || got[0] != "none" {
		t.Fatalf("throttleReasons = %v, want [none]", got)
	}
}
