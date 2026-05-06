package contracts

type DiagnosticLenses struct {
	Quantization *QuantizationLens `json:"quantization,omitempty"`
}

type QuantizationLens struct {
	ID                string                      `json:"id,omitempty"`
	CurrentPosture    QuantizationCurrentPosture  `json:"current_posture"`
	SelectedCandidate QuantizationCandidate       `json:"selected_candidate"`
	Recommendation    *Recommendation             `json:"recommendation,omitempty"`
	TargetOverlay     QuantizationScenarioOverlay `json:"target_overlay"`
	Confidence        string                      `json:"confidence,omitempty"`
	Caveats           []string                    `json:"caveats,omitempty"`
}

type QuantizationCurrentPosture struct {
	ModelID      string   `json:"model_id,omitempty"`
	DType        string   `json:"dtype,omitempty"`
	Quantization string   `json:"quantization,omitempty"`
	KVCacheDType string   `json:"kv_cache_dtype,omitempty"`
	GPUFamily    string   `json:"gpu_family,omitempty"`
	SupportNotes []string `json:"support_notes,omitempty"`
}

type QuantizationCandidate struct {
	Family         string   `json:"family"`
	Repo           string   `json:"repo,omitempty"`
	Source         string   `json:"source,omitempty"`
	SourceURL      string   `json:"source_url,omitempty"`
	HardwareFamily string   `json:"hardware_family,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
	Caveats        []string `json:"caveats,omitempty"`
}

type QuantizationScenarioOverlay struct {
	Target           string    `json:"target,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	GainRange        GainRange `json:"gain_range"`
	KVHeadroomEffect string    `json:"kv_headroom_effect,omitempty"`
	ComputeEffect    string    `json:"compute_effect,omitempty"`
	BandwidthEffect  string    `json:"bandwidth_effect,omitempty"`
	Confidence       string    `json:"confidence,omitempty"`
	Caveats          []string  `json:"caveats,omitempty"`
}
