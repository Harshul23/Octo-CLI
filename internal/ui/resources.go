package ui

import (
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// GetResourceStats fetches current system resource statistics
func GetResourceStats() ResourceStats {
	stats := ResourceStats{
		CPUTemp: -1, // Default to -1 (unavailable)
	}

	// Get CPU percentage
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		stats.CPUPercent = cpuPercent[0]
	}

	// Get memory stats
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		stats.MemoryUsed = memInfo.Used
		stats.MemoryTotal = memInfo.Total
		stats.MemPercent = memInfo.UsedPercent
	}

	// Get CPU temperature (platform-specific)
	stats.CPUTemp = getCPUTemperature()

	return stats
}

// getCPUTemperature attempts to get CPU temperature
// This is platform-specific and may not work on all systems
func getCPUTemperature() float64 {
	// Try to get temperature from host sensors
	temps, err := host.SensorsTemperatures()
	if err != nil {
		return -1
	}

	// Look for CPU temperature sensors
	// Different systems report this differently
	for _, temp := range temps {
		// Common CPU temperature sensor names
		switch {
		case contains(temp.SensorKey, "cpu", "coretemp", "k10temp", "CPU"):
			if temp.Temperature > 0 {
				return temp.Temperature
			}
		}
	}

	// On macOS with Apple Silicon, try to find any thermal sensor
	if runtime.GOOS == "darwin" {
		for _, temp := range temps {
			if temp.Temperature > 0 && temp.Temperature < 120 {
				// Return the first reasonable temperature
				return temp.Temperature
			}
		}
	}

	// If no CPU sensor found, try to return any reasonable temperature
	for _, temp := range temps {
		if temp.Temperature > 0 && temp.Temperature < 120 {
			return temp.Temperature
		}
	}

	return -1
}

// contains checks if the string contains any of the substrings (case-insensitive)
func contains(s string, substrs ...string) bool {
	sLower := toLowerCase(s)
	for _, sub := range substrs {
		if containsStr(sLower, toLowerCase(sub)) {
			return true
		}
	}
	return false
}

// toLowerCase converts a string to lowercase
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// containsStr checks if s contains substr
func containsStr(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// FormatBytes formats bytes into a human-readable string
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/GB) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/MB) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/KB) + " KB"
	default:
		return formatInt(int(bytes)) + " B"
	}
}

// formatFloat formats a float to 1 decimal place
func formatFloat(f float64) string {
	// Simple formatting without fmt.Sprintf
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return formatInt(whole) + "." + formatInt(frac)
}

// formatInt converts an int to string
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	
	negative := n < 0
	if negative {
		n = -n
	}
	
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
