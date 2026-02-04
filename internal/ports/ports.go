package ports

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// PortInfo contains information about a port extracted from a command
type PortInfo struct {
	Port     int
	Found    bool
	Pattern  string // The pattern that matched (e.g., "--port 3000", ":3000")
	Original string // The original matched string
}

// Common port patterns in run commands
var portPatterns = []*regexp.Regexp{
	// --port 3000, --port=3000, -p 3000, -p=3000
	regexp.MustCompile(`(?:--port[=\s]|--PORT[=\s]|-p[=\s])(\d+)`),
	// PORT=3000
	regexp.MustCompile(`(?:PORT=)(\d+)`),
	// Java/Spring Boot: -Dserver.port=8080
	regexp.MustCompile(`-Dserver\.port=(\d+)`),
	// :3000 (common in URLs and host:port patterns)
	regexp.MustCompile(`:(\d{4,5})(?:\s|$|/)`),
	// localhost:3000
	regexp.MustCompile(`localhost:(\d+)`),
	// 127.0.0.1:3000
	regexp.MustCompile(`127\.0\.0\.1:(\d+)`),
	// 0.0.0.0:3000
	regexp.MustCompile(`0\.0\.0\.0:(\d+)`),
}

// Default ports for common frameworks/tools
var defaultPorts = map[string]int{
	"npm start":                   3000,
	"npm run dev":                 3000,
	"yarn start":                  3000,
	"yarn dev":                    3000,
	"python manage.py runserver": 8000,
	"flask run":                   5000,
	"rails server":                3000,
	"bundle exec rails server":   3000,
	"go run":                      8080,
	"cargo run":                   8080,
	"mvn spring-boot:run":        8080,
	"./gradlew bootRun":           8080,
}

// IsPortAvailable checks if a port is available for binding
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// FindAvailablePort finds the next available port starting from the given port
func FindAvailablePort(startPort int) int {
	maxAttempts := 100 // Don't search forever
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		if IsPortAvailable(port) {
			return port
		}
	}
	return 0 // No available port found
}

// ExtractPort attempts to extract a port number from a run command
func ExtractPort(runCommand string) PortInfo {
	info := PortInfo{Found: false}

	// Try each pattern
	for _, pattern := range portPatterns {
		matches := pattern.FindStringSubmatch(runCommand)
		if len(matches) >= 2 {
			port, err := strconv.Atoi(matches[1])
			if err == nil && port > 0 && port < 65536 {
				info.Port = port
				info.Found = true
				info.Pattern = pattern.String()
				info.Original = matches[0]
				return info
			}
		}
	}

	// Check for default ports based on command patterns
	cmdLower := strings.ToLower(runCommand)
	for pattern, port := range defaultPorts {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			info.Port = port
			info.Found = true
			info.Pattern = "default"
			return info
		}
	}

	return info
}

// ShiftPort updates a run command to use a new port
func ShiftPort(runCommand string, oldPort, newPort int) string {
	oldPortStr := strconv.Itoa(oldPort)
	newPortStr := strconv.Itoa(newPort)

	// Try specific patterns first for more accurate replacement
	replacements := []struct {
		pattern *regexp.Regexp
		replace string
	}{
		// --port 3000 -> --port 3001
		{regexp.MustCompile(`(--port[=\s])` + oldPortStr + `\b`), "${1}" + newPortStr},
		// --PORT 3000 -> --PORT 3001
		{regexp.MustCompile(`(--PORT[=\s])` + oldPortStr + `\b`), "${1}" + newPortStr},
		// -p 3000 -> -p 3001
		{regexp.MustCompile(`(-p[=\s])` + oldPortStr + `\b`), "${1}" + newPortStr},
		// PORT=3000 -> PORT=3001
		{regexp.MustCompile(`(PORT=)` + oldPortStr + `\b`), "${1}" + newPortStr},
		// Java/Spring Boot: -Dserver.port=8080 -> -Dserver.port=8081
		{regexp.MustCompile(`(-Dserver\.port=)` + oldPortStr + `\b`), "${1}" + newPortStr},
		// localhost:3000 -> localhost:3001
		{regexp.MustCompile(`(localhost:)` + oldPortStr + `\b`), "${1}" + newPortStr},
		// 127.0.0.1:3000 -> 127.0.0.1:3001
		{regexp.MustCompile(`(127\.0\.0\.1:)` + oldPortStr + `\b`), "${1}" + newPortStr},
		// 0.0.0.0:3000 -> 0.0.0.0:3001
		{regexp.MustCompile(`(0\.0\.0\.0:)` + oldPortStr + `\b`), "${1}" + newPortStr},
		// :3000 -> :3001 (generic host:port)
		{regexp.MustCompile(`(:)` + oldPortStr + `\b`), "${1}" + newPortStr},
	}

	result := runCommand
	for _, r := range replacements {
		if r.pattern.MatchString(result) {
			result = r.pattern.ReplaceAllString(result, r.replace)
			return result
		}
	}

	// If no pattern matched but we detected a default port, try to add the port flag
	if strings.Contains(strings.ToLower(runCommand), "npm") ||
		strings.Contains(strings.ToLower(runCommand), "yarn") ||
		strings.Contains(strings.ToLower(runCommand), "pnpm") {
		// For npm/yarn/pnpm, use PORT environment variable (universally supported)
		// This works with Turbo, Vite, Next.js, etc.
		if !strings.HasPrefix(runCommand, "PORT=") {
			result = "PORT=" + newPortStr + " " + runCommand
		}
	} else if strings.Contains(strings.ToLower(runCommand), "python") {
		// For Python (Flask/Django), add --port flag
		if strings.Contains(runCommand, "flask") {
			result = runCommand + " --port " + newPortStr
		} else if strings.Contains(runCommand, "manage.py") {
			// Django uses 0.0.0.0:port format
			result = runCommand + " 0.0.0.0:" + newPortStr
		}
	} else if strings.Contains(strings.ToLower(runCommand), "rails") ||
		strings.Contains(strings.ToLower(runCommand), "bundle exec") {
		result = runCommand + " -p " + newPortStr
	} else if strings.Contains(runCommand, "mvn") || strings.Contains(runCommand, "gradle") {
		// Java/Spring Boot: append -Dserver.port
		if !strings.Contains(runCommand, "-Dserver.port") {
			result = runCommand + " -Dserver.port=" + newPortStr
		}
	} else if strings.Contains(runCommand, "java") {
		// Generic Java: append -Dserver.port before -jar or at the end
		if !strings.Contains(runCommand, "-Dserver.port") {
			if strings.Contains(runCommand, "-jar") {
				result = strings.Replace(runCommand, "-jar", "-Dserver.port="+newPortStr+" -jar", 1)
			} else {
				result = runCommand + " -Dserver.port=" + newPortStr
			}
		}
	}

	return result
}

// CheckAndShift checks if a port is in use and returns a shifted command if needed
// Returns: (newCommand, newPort, wasShifted, error)
func CheckAndShift(runCommand string) (string, int, bool, error) {
	portInfo := ExtractPort(runCommand)

	if !portInfo.Found {
		// No port detected, can't check for conflicts
		return runCommand, 0, false, nil
	}

	if IsPortAvailable(portInfo.Port) {
		// Port is available, no shift needed
		return runCommand, portInfo.Port, false, nil
	}

	// Port is in use, find a new one
	newPort := FindAvailablePort(portInfo.Port + 1)
	if newPort == 0 {
		return "", 0, false, fmt.Errorf("could not find an available port after %d", portInfo.Port)
	}

	// Shift the command to use the new port
	newCommand := ShiftPort(runCommand, portInfo.Port, newPort)

	return newCommand, newPort, true, nil
}

// GetPortStatus returns a human-readable status of a port
func GetPortStatus(port int) string {
	if IsPortAvailable(port) {
		return fmt.Sprintf("Port %d is available", port)
	}
	return fmt.Sprintf("Port %d is in use", port)
}

// AppendPortFlag appends the appropriate port flag for a language to a command
func AppendPortFlag(runCommand string, language string, port int) string {
	portStr := strconv.Itoa(port)
	
	switch strings.ToLower(language) {
	case "node", "nodejs", "javascript", "typescript":
		// Node.js: use PORT environment variable (universally supported)
		// This works with Turbo, Vite, Next.js, Create React App, etc.
		if !strings.HasPrefix(runCommand, "PORT=") {
			return "PORT=" + portStr + " " + runCommand
		}
		return runCommand
		
	case "python":
		// Python: Flask uses --port, Django uses host:port
		if strings.Contains(runCommand, "flask") {
			return runCommand + " --port " + portStr
		} else if strings.Contains(runCommand, "manage.py") {
			return runCommand + " 0.0.0.0:" + portStr
		}
		return runCommand + " --port " + portStr
		
	case "java":
		// Java/Spring Boot: use -Dserver.port
		if strings.Contains(runCommand, "-jar") {
			return strings.Replace(runCommand, "-jar", "-Dserver.port="+portStr+" -jar", 1)
		}
		return runCommand + " -Dserver.port=" + portStr
		
	case "ruby":
		return runCommand + " -p " + portStr
		
	case "go", "golang":
		return runCommand + " --port " + portStr
		
	default:
		return runCommand + " --port " + portStr
	}
}
