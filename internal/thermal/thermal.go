package thermal

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Config holds thermal and resource management settings
type Config struct {
	// Concurrency is the maximum number of concurrent operations
	Concurrency int `yaml:"concurrency,omitempty"`
	// BatchSize is the number of projects to process in each batch
	BatchSize int `yaml:"batch_size,omitempty"`
	// CoolDownMs is the delay between batches in milliseconds
	CoolDownMs int `yaml:"cool_down_ms,omitempty"`
	// ThermalMode enables thermal-aware scheduling ("auto", "cool", "performance")
	ThermalMode string `yaml:"thermal_mode,omitempty"`
}

// HardwareInfo contains detected hardware information
type HardwareInfo struct {
	NumCPU         int
	IsDarwin       bool
	IsMacBookAir   bool
	IsAppleSilicon bool
	ModelName      string
}

// DefaultBatchThreshold is the project count threshold for enabling batching
const DefaultBatchThreshold = 5

// DefaultCoolDownMs is the default cool-down period between batches
const DefaultCoolDownMs = 500

// DetectHardware detects the current hardware configuration
func DetectHardware() HardwareInfo {
	info := HardwareInfo{
		NumCPU:   runtime.NumCPU(),
		IsDarwin: runtime.GOOS == "darwin",
	}

	if info.IsDarwin {
		info.ModelName = detectMacModel()
		info.IsMacBookAir = strings.Contains(strings.ToLower(info.ModelName), "macbook air")
		info.IsAppleSilicon = detectAppleSilicon()
	}

	return info
}

// detectMacModel returns the Mac model identifier
func detectMacModel() string {
	cmd := exec.Command("sysctl", "-n", "hw.model")
	output, err := cmd.Output()
	if err != nil {
		// Fallback: try to get marketing name
		cmd = exec.Command("system_profiler", "SPHardwareDataType")
		output, err = cmd.Output()
		if err != nil {
			return ""
		}
		// Parse for Model Name
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Model Name:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
		return ""
	}
	return strings.TrimSpace(string(output))
}

// detectAppleSilicon checks if the Mac has Apple Silicon
func detectAppleSilicon() bool {
	// Check architecture
	if runtime.GOARCH == "arm64" {
		return true
	}

	// Additional check via sysctl
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	brand := strings.ToLower(string(output))
	return strings.Contains(brand, "apple")
}

// GetOptimalConcurrency returns the optimal concurrency level based on hardware
func GetOptimalConcurrency(hw HardwareInfo, configConcurrency int) int {
	// If explicitly configured, use that value
	if configConcurrency > 0 {
		return configConcurrency
	}

	// Default: use all cores
	optimal := hw.NumCPU

	// On MacBook Air (especially Apple Silicon), reduce to prevent thermal throttling
	if hw.IsMacBookAir {
		// MacBook Air has passive cooling, so we need to be conservative
		optimal = hw.NumCPU / 2
		if optimal < 2 {
			optimal = 2
		}
	} else if hw.IsDarwin && hw.IsAppleSilicon {
		// Other Apple Silicon Macs (Pro/Max) have better cooling but still benefit
		// from slightly reduced concurrency for sustained workloads
		optimal = (hw.NumCPU * 3) / 4
		if optimal < 2 {
			optimal = 2
		}
	}

	return optimal
}

// GetOptimalBatchSize returns the optimal batch size for project processing
func GetOptimalBatchSize(hw HardwareInfo, projectCount int, configBatchSize int) int {
	// If explicitly configured, use that value
	if configBatchSize > 0 {
		return configBatchSize
	}

	// If project count is below threshold, no batching needed
	if projectCount <= DefaultBatchThreshold {
		return projectCount
	}

	// Base batch size on hardware
	batchSize := 3 // Conservative default

	if hw.IsMacBookAir {
		// Very conservative for passive cooling
		batchSize = 2
	} else if hw.IsDarwin && hw.IsAppleSilicon {
		// Moderate for active cooling Apple Silicon
		batchSize = 4
	} else if hw.NumCPU >= 8 {
		// More generous for higher core count machines
		batchSize = 5
	}

	return batchSize
}

// ToolConcurrencyFlags contains concurrency flag mappings for known tools
type ToolConcurrencyFlags struct {
	// FlagFormat is the format string for the concurrency flag (e.g., "--concurrency=%d")
	FlagFormat string
	// Position indicates where to insert the flag ("append", "after-command", "before-args")
	Position string
}

// KnownTools maps tool names to their concurrency flag formats
var KnownTools = map[string]ToolConcurrencyFlags{
	"pnpm": {
		FlagFormat: "--network-concurrency=%d",
		Position:   "append",
	},
	"turbo": {
		FlagFormat: "--concurrency=%d",
		Position:   "append",
	},
	"turborepo": {
		FlagFormat: "--concurrency=%d",
		Position:   "append",
	},
	"npm": {
		FlagFormat: "--maxsockets=%d",
		Position:   "append",
	},
	"yarn": {
		FlagFormat: "--network-concurrency=%d",
		Position:   "append",
	},
	"lerna": {
		FlagFormat: "--concurrency=%d",
		Position:   "append",
	},
	"nx": {
		FlagFormat: "--parallel=%d",
		Position:   "append",
	},
	"rush": {
		FlagFormat: "--parallelism=%d",
		Position:   "append",
	},
	"make": {
		FlagFormat: "-j%d",
		Position:   "after-command",
	},
	"cargo": {
		FlagFormat: "-j%d",
		Position:   "append",
	},
	"go": {
		FlagFormat: "-p=%d",
		Position:   "append",
	},
}

// InjectConcurrencyFlag injects a concurrency flag into a command if the tool supports it
func InjectConcurrencyFlag(command string, concurrency int) string {
	if concurrency <= 0 {
		return command
	}

	// Parse the command to find the tool
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command
	}

	// Get the base command (handle paths like /usr/bin/pnpm)
	baseTool := parts[0]
	if idx := strings.LastIndex(baseTool, "/"); idx >= 0 {
		baseTool = baseTool[idx+1:]
	}

	// Special handling for package manager "run" commands
	// These typically invoke other tools (like turbo) that have their own concurrency handling
	// We should not inject flags for "pnpm run", "npm run", "yarn run" commands
	if (baseTool == "pnpm" || baseTool == "npm" || baseTool == "yarn") && len(parts) > 1 {
		subCmd := parts[1]
		// Don't inject for run/exec commands - let the underlying tool handle concurrency
		if subCmd == "run" || subCmd == "exec" || subCmd == "dlx" || subCmd == "npx" {
			return command
		}
	}

	// Check if this tool supports concurrency flags
	toolConfig, exists := KnownTools[baseTool]
	if !exists {
		return command
	}

	// Check if a concurrency flag is already present
	if hasConcurrencyFlag(command, baseTool) {
		return command
	}

	// Format the concurrency flag
	flag := fmt.Sprintf(toolConfig.FlagFormat, concurrency)

	// Inject the flag based on position
	switch toolConfig.Position {
	case "after-command":
		// Insert after the command name (e.g., make -j4 build)
		if len(parts) > 1 {
			return parts[0] + " " + flag + " " + strings.Join(parts[1:], " ")
		}
		return command + " " + flag
	case "append":
		fallthrough
	default:
		// Append to end of command
		return command + " " + flag
	}
}

// hasConcurrencyFlag checks if the command already has a concurrency flag
func hasConcurrencyFlag(command string, tool string) bool {
	lowerCmd := strings.ToLower(command)

	switch tool {
	case "pnpm":
		return strings.Contains(lowerCmd, "--network-concurrency")
	case "turbo", "turborepo":
		return strings.Contains(lowerCmd, "--concurrency")
	case "npm":
		return strings.Contains(lowerCmd, "--maxsockets")
	case "yarn":
		return strings.Contains(lowerCmd, "--network-concurrency")
	case "lerna":
		return strings.Contains(lowerCmd, "--concurrency")
	case "nx":
		return strings.Contains(lowerCmd, "--parallel")
	case "rush":
		return strings.Contains(lowerCmd, "--parallelism")
	case "make":
		return strings.Contains(command, "-j")
	case "cargo":
		return strings.Contains(command, "-j")
	case "go":
		return strings.Contains(command, "-p=") || strings.Contains(command, "-p ")
	}

	return false
}

// ThermalStatus represents the current thermal state
type ThermalStatus struct {
	// Level is the thermal level ("cool", "warm", "hot", "critical")
	Level string
	// RecommendedConcurrency is the recommended concurrency based on thermal state
	RecommendedConcurrency int
	// Message is a human-readable status message
	Message string
}

// GetThermalStatus returns the current thermal status (macOS only)
func GetThermalStatus(hw HardwareInfo) ThermalStatus {
	status := ThermalStatus{
		Level:                  "cool",
		RecommendedConcurrency: hw.NumCPU,
		Message:                "System is running cool",
	}

	if !hw.IsDarwin {
		return status
	}

	// Try to get thermal state from pmset
	cmd := exec.Command("pmset", "-g", "therm")
	output, err := cmd.Output()
	if err != nil {
		return status
	}

	outputStr := strings.ToLower(string(output))

	// Parse thermal state
	if strings.Contains(outputStr, "cpu_speed_limit") {
		// Check for speed limiting (indicates thermal throttling)
		if strings.Contains(outputStr, "cpu_speed_limit\t\t100") ||
			strings.Contains(outputStr, "cpu_speed_limit		100") {
			status.Level = "cool"
			status.Message = "CPU running at full speed"
		} else {
			status.Level = "warm"
			status.RecommendedConcurrency = hw.NumCPU / 2
			status.Message = "CPU is being throttled due to thermal pressure"
		}
	}

	// Check for thermal pressure indicators
	if strings.Contains(outputStr, "thermal_level") {
		if strings.Contains(outputStr, "thermal_level\t\t0") ||
			strings.Contains(outputStr, "thermal_level		0") {
			status.Level = "cool"
		} else {
			status.Level = "warm"
			status.RecommendedConcurrency = hw.NumCPU / 2
			status.Message = "System thermal pressure detected"
		}
	}

	return status
}

// FormatHardwareInfo returns a human-readable hardware description
func FormatHardwareInfo(hw HardwareInfo) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%d cores", hw.NumCPU))

	if hw.IsDarwin {
		if hw.ModelName != "" {
			parts = append(parts, hw.ModelName)
		}
		if hw.IsAppleSilicon {
			parts = append(parts, "Apple Silicon")
		}
	} else {
		parts = append(parts, runtime.GOOS)
	}

	return strings.Join(parts, ", ")
}
