//go:build linux

package collector

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	nvmlSuccess           = 0
	nvmlErrorNotSupported = 3
	nvmlClockSM           = 1
	nvmlClockMem          = 2
	nvmlTempGPU           = 0
	nvmlPcieTxBytes       = 0
	nvmlPcieRxBytes       = 1
)

type nvmlDeviceHandle uintptr

type nvmlMemory struct {
	Total uint64
	Free  uint64
	Used  uint64
}

type nvmlUtilization struct {
	GPU    uint32
	Memory uint32
}

type nvmlLibrary struct {
	handle                uintptr
	initFn                func() int32
	shutdownFn            func() int32
	driverVersionFn       func(*byte, uint32) int32
	deviceCountFn         func(*uint32) int32
	deviceHandleByIndexFn func(uint32, *nvmlDeviceHandle) int32
	deviceNameFn          func(nvmlDeviceHandle, *byte, uint32) int32
	utilizationFn         func(nvmlDeviceHandle, *nvmlUtilization) int32
	memoryFn              func(nvmlDeviceHandle, *nvmlMemory) int32
	clockFn               func(nvmlDeviceHandle, uint32, *uint32) int32
	powerFn               func(nvmlDeviceHandle, *uint32) int32
	temperatureFn         func(nvmlDeviceHandle, uint32, *uint32) int32
	pcieThroughputFn      func(nvmlDeviceHandle, uint32, *uint32) int32
}

func openNVML() (*nvmlLibrary, error) {
	handle, err := purego.Dlopen("libnvidia-ml.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}

	lib := &nvmlLibrary{handle: handle}
	loaders := []func() error{
		func() error { return registerSymbol(handle, &lib.initFn, "nvmlInit_v2") },
		func() error { return registerSymbol(handle, &lib.shutdownFn, "nvmlShutdown") },
		func() error { return registerSymbol(handle, &lib.driverVersionFn, "nvmlSystemGetDriverVersion") },
		func() error {
			return registerFirst(handle, &lib.deviceCountFn, "nvmlDeviceGetCount_v2", "nvmlDeviceGetCount")
		},
		func() error {
			return registerFirst(handle, &lib.deviceHandleByIndexFn, "nvmlDeviceGetHandleByIndex_v2", "nvmlDeviceGetHandleByIndex")
		},
		func() error { return registerSymbol(handle, &lib.deviceNameFn, "nvmlDeviceGetName") },
		func() error { return registerSymbol(handle, &lib.utilizationFn, "nvmlDeviceGetUtilizationRates") },
		func() error { return registerSymbol(handle, &lib.memoryFn, "nvmlDeviceGetMemoryInfo") },
		func() error { return registerSymbol(handle, &lib.clockFn, "nvmlDeviceGetClockInfo") },
		func() error { return registerSymbol(handle, &lib.powerFn, "nvmlDeviceGetPowerUsage") },
		func() error { return registerSymbol(handle, &lib.temperatureFn, "nvmlDeviceGetTemperature") },
		func() error { return registerSymbol(handle, &lib.pcieThroughputFn, "nvmlDeviceGetPcieThroughput") },
	}
	for _, loader := range loaders {
		if err := loader(); err != nil {
			_ = purego.Dlclose(handle)
			return nil, err
		}
	}

	if ret := lib.initFn(); ret != nvmlSuccess {
		_ = purego.Dlclose(handle)
		return nil, fmt.Errorf("nvmlInit_v2 failed with code %d", ret)
	}
	return lib, nil
}

func registerSymbol(handle uintptr, target any, symbol string) error {
	addr, err := purego.Dlsym(handle, symbol)
	if err != nil {
		return err
	}
	purego.RegisterFunc(target, addr)
	return nil
}

func registerFirst(handle uintptr, target any, symbols ...string) error {
	var firstErr error
	for _, symbol := range symbols {
		if err := registerSymbol(handle, target, symbol); err == nil {
			return nil
		} else if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (l *nvmlLibrary) Close() {
	if l == nil || l.handle == 0 {
		return
	}
	if l.shutdownFn != nil {
		_ = l.shutdownFn()
	}
	_ = purego.Dlclose(l.handle)
}

func (l *nvmlLibrary) DriverVersion() (string, error) {
	buffer := make([]byte, 96)
	if ret := l.driverVersionFn(&buffer[0], uint32(len(buffer))); ret != nvmlSuccess {
		return "", fmt.Errorf("driver version failed with code %d", ret)
	}
	return cString(buffer), nil
}

func (l *nvmlLibrary) Devices() ([]nvmlDeviceHandle, error) {
	var count uint32
	if ret := l.deviceCountFn(&count); ret != nvmlSuccess {
		return nil, fmt.Errorf("device count failed with code %d", ret)
	}
	devices := make([]nvmlDeviceHandle, 0, count)
	for idx := uint32(0); idx < count; idx++ {
		var handle nvmlDeviceHandle
		if ret := l.deviceHandleByIndexFn(idx, &handle); ret == nvmlSuccess {
			devices = append(devices, handle)
		}
	}
	return devices, nil
}

func cString(buffer []byte) string {
	for idx, value := range buffer {
		if value == 0 {
			return string(buffer[:idx])
		}
	}
	return string(buffer)
}

func bytesPointer(buffer []byte) *byte {
	if len(buffer) == 0 {
		return (*byte)(unsafe.Pointer(uintptr(0)))
	}
	return &buffer[0]
}
