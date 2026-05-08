package contracts

func (w MetricWindow) HasData() bool {
	return w.Latest != nil || w.Min != nil || w.Max != nil || w.Avg != nil || len(w.Samples) > 0
}

func (d DistributionSnapshot) HasData() bool {
	return d.Count != nil || d.Sum != nil || len(d.Buckets) > 0
}

func (d DeltaSnapshot) HasData() bool {
	return d.Total != nil || len(d.Series) > 0
}

func (c CacheSnapshot) HasData() bool {
	return c.Hits != nil || c.Queries != nil || c.HitRate != nil
}

func (m MemoryMetrics) HasData() bool {
	return m.Used.HasData() || m.Free.HasData() || m.Reserved.HasData() || m.Total.HasData()
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

func (s SchedulerConfig) HasData() bool {
	return s.AsyncScheduling != nil ||
		s.Policy != "" ||
		s.MaxNumPartialPrefills != 0 ||
		s.MaxLongPartialPrefills != 0 ||
		s.LongPrefillTokenThreshold != 0 ||
		s.MaxNumScheduledTokens != 0 ||
		s.MaxNumEncoderInputTokens != 0 ||
		s.SchedulerReserveFullISL != nil ||
		s.DisableChunkedMMInput != nil ||
		s.DisableHybridKVCacheManager != nil
}

func (c CacheConfig) HasData() bool {
	return c.BlockSize != 0 ||
		c.CacheDType != "" ||
		c.NumGPUBlocks != 0 ||
		c.NumCPUBlocks != 0 ||
		c.KVCacheMemoryBytes != 0 ||
		c.KVOffloadingBackend != "" ||
		c.KVOffloadingSizeBytes != 0 ||
		c.KVSharingFastPrefill != nil ||
		c.SlidingWindow != 0 ||
		c.PrefixCachingHashAlgo != "" ||
		c.CalculateKVScales != nil ||
		c.GPUMemoryUtilization != 0
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (r CapacityRates) HasData() bool {
	return r.PromptTokensPerSecond != nil || r.GenerationTokensPerSecond != nil || r.RequestThroughput != nil
}

func (p CapacityPressures) HasData() bool {
	return p.Compute != "" || p.MemoryBandwidth != "" || p.KV != "" || p.Queue != "" || p.Host != ""
}

func (s CapacitySnapshot) HasData() bool {
	return s.Pressures.HasData() || s.Observed.HasData() || s.CurrentFrontier.HasData() || s.Confidence != "" || s.Summary != "" || len(s.Notes) > 0
}
