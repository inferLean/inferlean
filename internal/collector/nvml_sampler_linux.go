//go:build linux

package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func collectNVMLSamples(ctx context.Context, interval time.Duration, rawPath string) (*nvmlSnapshot, contracts.SourceCoverage, error) {
	lib, err := openNVML()
	if err != nil {
		coverage := missingCoverage(gpuRequiredFields, relativeRawArtifact(rawPath))
		payload := map[string]any{"supported": false, "error": err.Error(), "coverage": coverage}
		if writeErr := writeJSONFile(rawPath, payload); writeErr != nil {
			return nil, contracts.SourceCoverage{}, writeErr
		}
		return nil, coverage, nil
	}
	defer lib.Close()

	snapshot, err := runNVMLSampler(ctx, lib, interval)
	if err != nil {
		return nil, contracts.SourceCoverage{}, err
	}
	if err := writeJSONFile(rawPath, snapshot); err != nil {
		return nil, contracts.SourceCoverage{}, fmt.Errorf("write nvml samples: %w", err)
	}
	coverage := nvmlCoverage(snapshot, relativeRawArtifact(rawPath))
	return snapshot, coverage, nil
}

func runNVMLSampler(ctx context.Context, lib *nvmlLibrary, interval time.Duration) (*nvmlSnapshot, error) {
	driver, _ := lib.DriverVersion()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	snapshot := &nvmlSnapshot{DriverVersion: driver}
	for {
		select {
		case <-ctx.Done():
			return snapshot, nil
		case <-ticker.C:
			sample, err := sampleNVML(lib)
			if err != nil {
				return snapshot, err
			}
			snapshot.Samples = append(snapshot.Samples, sample)
		}
	}
}

func sampleNVML(lib *nvmlLibrary) (nvmlSample, error) {
	devices, err := lib.Devices()
	if err != nil {
		return nvmlSample{}, err
	}
	driver, _ := lib.DriverVersion()
	sample := nvmlSample{Timestamp: time.Now().UTC(), Driver: driver}
	for _, device := range devices {
		sample.GPUs = append(sample.GPUs, sampleNVMLDevice(lib, device))
	}
	return sample, nil
}

func sampleNVMLDevice(lib *nvmlLibrary, device nvmlDeviceHandle) nvmlGPU {
	return nvmlGPU{
		Name:             nvmlDeviceName(lib, device),
		Utilization:      nvmlDeviceUtilization(lib, device),
		MemoryUsedBytes:  nvmlDeviceMemoryUsed(lib, device),
		MemoryFreeBytes:  nvmlDeviceMemoryFree(lib, device),
		MemoryTotalBytes: nvmlDeviceMemoryTotal(lib, device),
		SMClockMHz:       nvmlDeviceClock(lib, device, nvmlClockSM),
		MemClockMHz:      nvmlDeviceClock(lib, device, nvmlClockMem),
		PowerDrawWatts:   nvmlDevicePower(lib, device),
		TemperatureC:     nvmlDeviceTemperature(lib, device),
		PCIeRxKBs:        nvmlDevicePCIe(lib, device, nvmlPcieRxBytes),
		PCIeTxKBs:        nvmlDevicePCIe(lib, device, nvmlPcieTxBytes),
	}
}

func nvmlDeviceName(lib *nvmlLibrary, device nvmlDeviceHandle) string {
	buffer := make([]byte, 96)
	if ret := lib.deviceNameFn(device, bytesPointer(buffer), uint32(len(buffer))); ret != nvmlSuccess {
		return ""
	}
	return cString(buffer)
}

func nvmlDeviceUtilization(lib *nvmlLibrary, device nvmlDeviceHandle) float64 {
	var util nvmlUtilization
	if ret := lib.utilizationFn(device, &util); ret != nvmlSuccess {
		return 0
	}
	return float64(util.GPU)
}

func nvmlDeviceMemoryUsed(lib *nvmlLibrary, device nvmlDeviceHandle) float64 {
	memory, ok := nvmlDeviceMemory(lib, device)
	if !ok {
		return 0
	}
	return float64(memory.Used)
}

func nvmlDeviceMemoryFree(lib *nvmlLibrary, device nvmlDeviceHandle) float64 {
	memory, ok := nvmlDeviceMemory(lib, device)
	if !ok {
		return 0
	}
	return float64(memory.Free)
}

func nvmlDeviceMemoryTotal(lib *nvmlLibrary, device nvmlDeviceHandle) float64 {
	memory, ok := nvmlDeviceMemory(lib, device)
	if !ok {
		return 0
	}
	return float64(memory.Total)
}

func nvmlDeviceMemory(lib *nvmlLibrary, device nvmlDeviceHandle) (nvmlMemory, bool) {
	var memory nvmlMemory
	if ret := lib.memoryFn(device, &memory); ret != nvmlSuccess {
		return nvmlMemory{}, false
	}
	return memory, true
}

func nvmlDeviceClock(lib *nvmlLibrary, device nvmlDeviceHandle, clockType uint32) float64 {
	var value uint32
	if ret := lib.clockFn(device, clockType, &value); ret != nvmlSuccess {
		return 0
	}
	return float64(value)
}

func nvmlDevicePower(lib *nvmlLibrary, device nvmlDeviceHandle) float64 {
	var value uint32
	if ret := lib.powerFn(device, &value); ret != nvmlSuccess {
		return 0
	}
	return float64(value) / 1000
}

func nvmlDeviceTemperature(lib *nvmlLibrary, device nvmlDeviceHandle) float64 {
	var value uint32
	if ret := lib.temperatureFn(device, nvmlTempGPU, &value); ret != nvmlSuccess {
		return 0
	}
	return float64(value)
}

func nvmlDevicePCIe(lib *nvmlLibrary, device nvmlDeviceHandle, counter uint32) float64 {
	var value uint32
	ret := lib.pcieThroughputFn(device, counter, &value)
	if ret != nvmlSuccess && ret != nvmlErrorNotSupported {
		return 0
	}
	return float64(value)
}

func nvmlCoverage(snapshot *nvmlSnapshot, rawRef string) contracts.SourceCoverage {
	coverage := newCoverageBuilder(rawRef)
	if snapshot == nil || len(snapshot.Samples) == 0 {
		for _, field := range gpuRequiredFields {
			coverage.Missing(field)
		}
		return coverage.Build()
	}

	coverage.Present("gpu_utilization_or_sm_activity")
	coverage.Present("framebuffer_memory")
	coverage.Unsupported("memory_bandwidth")
	coverage.Present("clocks")
	coverage.Present("power")
	coverage.Present("temperature")
	coverage.Present("pcie_throughput")
	coverage.Unsupported("nvlink_throughput")
	coverage.Unsupported("reliability_errors")
	return coverage.Build()
}
