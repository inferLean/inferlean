package contracts

func (w MetricWindow) HasData() bool {
	return w.Latest != nil || w.Min != nil || w.Max != nil || w.Avg != nil || len(w.Samples) > 0
}

func (d DistributionSnapshot) HasData() bool {
	return d.Count != nil || d.Sum != nil || len(d.Buckets) > 0
}

func (c CacheSnapshot) HasData() bool {
	return c.Hits != nil || c.Queries != nil || c.HitRate != nil
}

func (m MemoryMetrics) HasData() bool {
	return m.Used.HasData() || m.Free.HasData() || m.Total.HasData()
}

func (c ClockMetrics) HasData() bool {
	return c.SM.HasData() || c.Memory.HasData()
}

func (t ThroughputMetrics) HasData() bool {
	return t.RX.HasData() || t.TX.HasData()
}

func (r ReliabilityMetrics) HasData() bool {
	return r.XID.HasData() || r.ECC.HasData()
}

func (c SourceCoverage) MarksField(name string) bool {
	return containsString(c.MissingFields, name) || containsString(c.UnsupportedFields, name)
}

func (c SourceCoverage) HasField(name string) bool {
	return containsString(c.PresentFields, name)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
