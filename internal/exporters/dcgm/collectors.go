package dcgm

import (
	"os"
	"strings"
)

func writeCollectorsFile() (string, error) {
	file, err := os.CreateTemp("", "inferlean-dcgm-collectors-*.csv")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(defaultCollectorsCSV()); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func defaultCollectorsCSV() string {
	lines := []string{
		"DCGM_FI_DEV_GPU_UTIL, gauge, GPU utilization.",
		"DCGM_FI_DEV_FB_USED, gauge, Framebuffer memory used.",
		"DCGM_FI_DEV_FB_FREE, gauge, Framebuffer memory free.",
		"DCGM_FI_DEV_FB_RESERVED, gauge, Framebuffer memory reserved.",
		"DCGM_FI_DEV_FB_TOTAL, gauge, Total framebuffer memory.",
		"DCGM_FI_PROF_SM_ACTIVE, gauge, Ratio of cycles an SM has at least one warp assigned.",
		"DCGM_FI_PROF_SM_OCCUPANCY, gauge, Ratio of resident warps to theoretical maximum.",
		"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE, gauge, Ratio of cycles any tensor pipe is active.",
		"DCGM_FI_PROF_DRAM_ACTIVE, gauge, Ratio of cycles the device memory interface is active.",
		"DCGM_FI_PROF_PCIE_RX_BYTES, counter, Active PCIe receive bytes.",
		"DCGM_FI_PROF_PCIE_TX_BYTES, counter, Active PCIe transmit bytes.",
		"DCGM_FI_PROF_NVLINK_RX_BYTES, counter, Active NVLink receive bytes.",
		"DCGM_FI_PROF_NVLINK_TX_BYTES, counter, Active NVLink transmit bytes.",
		"DCGM_FI_DEV_POWER_USAGE, gauge, GPU power usage.",
		"DCGM_FI_DEV_GPU_TEMP, gauge, GPU temperature.",
		"DCGM_FI_DEV_SM_CLOCK, gauge, SM clock frequency.",
		"DCGM_FI_DEV_MEM_CLOCK, gauge, Memory clock frequency.",
		"DCGM_FI_DEV_CLOCK_THROTTLE_REASONS, gauge, Clock throttle reason bitmap.",
		"DCGM_FI_DEV_XID_ERRORS, gauge, XID errors.",
		"DCGM_FI_DEV_ECC_DBE_VOL_TOTAL, counter, Volatile double-bit ECC errors.",
		"DCGM_FI_DEV_ECC_SBE_VOL_TOTAL, counter, Volatile single-bit ECC errors.",
	}
	return strings.Join(lines, "\n") + "\n"
}
