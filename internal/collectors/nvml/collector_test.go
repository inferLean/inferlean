package nvml

import "testing"

func TestParseSampleIncludesClocks(t *testing.T) {
	sample, ok := parseSample("0, 12, 4749, 6144, 80.5, 55, 1400, 7001")
	if !ok {
		t.Fatal("parseSample() ok = false")
	}
	if sample.SMClock != 1400 {
		t.Fatalf("SMClock = %f, want 1400", sample.SMClock)
	}
	if sample.MemoryClock != 7001 {
		t.Fatalf("MemoryClock = %f, want 7001", sample.MemoryClock)
	}
}
