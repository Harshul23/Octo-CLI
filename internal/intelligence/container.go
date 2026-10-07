package intelligence

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ContainerAdapter executes plan steps inside ephemeral, reproducible containers (Docker or Podman).
type ContainerAdapter struct {
	Engine         string // "docker" or "podman"
	Root           string // host root directory
	Language       string
	RuntimeVersion string
}

// NewContainerAdapter creates a container adapter with the preferred available container engine.
func NewContainerAdapter(root string) ContainerAdapter {
	engine := "docker"
	if _, err := exec.LookPath("docker"); err != nil {
		if _, err := exec.LookPath("podman"); err == nil {
			engine = "podman"
		}
	}
	return ContainerAdapter{
		Engine: engine,
		Root:   root,
	}
}

func (a ContainerAdapter) Name() string { return "container" }

func (a ContainerAdapter) Supports(step ExecutionStep) bool {
	// Container adapter supports all non-compose execution steps
	return step.Command != "" && !strings.HasPrefix(step.Command, "docker compose ")
}

func (a ContainerAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if step.Command == "" {
		return fmt.Errorf("step %q has no command", step.ID)
	}

	containerName := generateContainerName(step.Component, string(step.Phase))
	args, err := a.BuildRunArgs(step, env, false, containerName)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, a.Engine, args...)
	if a.Root != "" {
		cmd.Dir = a.Root
	}

	if isSilentExecution(ctx) {
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Run(); err != nil {
			detail := strings.TrimSpace(output.String())
			if detail != "" {
				return fmt.Errorf("containerized step %q failed: %w\n%s", step.ID, err, detail)
			}
			return fmt.Errorf("containerized step %q failed: %w", step.ID, err)
		}
		return nil
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("containerized step %q failed: %w", step.ID, err)
	}
	return nil
}

func (a ContainerAdapter) Start(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) (RunningProcess, error) {
	if step.Command == "" {
		return nil, fmt.Errorf("step %q has no command", step.ID)
	}

	containerName := generateContainerName(step.Component, string(step.Phase))
	args, err := a.BuildRunArgs(step, env, true, containerName)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(a.Engine, args...)
	if a.Root != "" {
		cmd.Dir = a.Root
	}

	workDir := a.Root
	if workDir == "" {
		workDir = "."
	}
	logDir := filepath.Join(workDir, ".octo", "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logName := step.Component
	if logName == "" {
		logName = "container-process"
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, logName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else if isSilentExecution(ctx) {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("containerized step %q failed to start: %w", step.ID, err)
	}

	return &containerProcess{
		engine:        a.Engine,
		containerName: containerName,
		cmd:           cmd,
	}, nil
}

// BuildRunArgs compiles the command-line arguments for running the containerized step.
func (a ContainerAdapter) BuildRunArgs(step ExecutionStep, env ResolvedEnvironment, detach bool, containerName string) ([]string, error) {
	args := []string{"run", "--rm"}
	if detach {
		args = append(args, "-d")
	}
	if containerName != "" {
		args = append(args, "--name", containerName)
	}

	// 1. Volume mount and working directory
	hostRoot := a.Root
	if hostRoot == "" {
		hostRoot = "."
	}
	absHostRoot, err := filepath.Abs(hostRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve host root: %w", err)
	}
	args = append(args, "-v", fmt.Sprintf("%s:/app", absHostRoot))

	workDir := "/app"
	if step.WorkDir != "" && step.WorkDir != "." {
		relWorkDir := filepath.ToSlash(step.WorkDir)
		relWorkDir = strings.TrimPrefix(relWorkDir, "./")
		workDir = fmt.Sprintf("/app/%s", relWorkDir)
	}
	args = append(args, "-w", workDir)

	// 2. Port bindings
	if portStr, ok := step.Environment["PORT"]; ok && portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			args = append(args, "-p", fmt.Sprintf("%d:%d", p, p))
		}
	}

	// 3. Environment variables
	mergedEnv := make(map[string]string)
	stepEnv := env.ForStep(step)
	for k, v := range stepEnv.Values {
		mergedEnv[k] = v
	}
	for k, v := range step.Environment {
		mergedEnv[k] = v
	}

	var envKeys []string
	for k := range mergedEnv {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, mergedEnv[k]))
	}

	// 4. Base Image
	image := a.resolveImageForStep(step)
	args = append(args, image, "sh", "-c", step.Command)

	return args, nil
}

func (a ContainerAdapter) resolveImageForStep(step ExecutionStep) string {
	lang := a.Language
	ver := a.RuntimeVersion
	if lang == "" {
		lang = inferLanguageFromCommand(step.Command)
	}
	if lang == "" {
		lang = step.Component
	}
	return ResolveContainerImage(lang, ver)
}

func inferLanguageFromCommand(cmd string) string {
	lower := strings.ToLower(cmd)
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return ""
	}
	first := fields[0]
	switch first {
	case "npm", "yarn", "pnpm", "node", "bun", "npx", "next", "vite":
		return "node"
	case "python", "python3", "pip", "pip3", "poetry", "pytest", "uv":
		return "python"
	case "go":
		return "go"
	case "cargo", "rustc":
		return "rust"
	case "ruby", "bundle", "rake":
		return "ruby"
	case "mvn", "gradle", "java":
		return "java"
	default:
		return ""
	}
}

type containerProcess struct {
	engine        string
	containerName string
	cmd           *exec.Cmd
}

func (p *containerProcess) Wait() error {
	if p.cmd != nil {
		_ = p.cmd.Wait()
	}
	cmd := exec.Command(p.engine, "wait", p.containerName)
	return cmd.Run()
}

func (p *containerProcess) Stop() error {
	return p.GracefulStop(2 * time.Second)
}

func (p *containerProcess) GracefulStop(timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if secs <= 0 {
		secs = 2
	}
	cmd := exec.Command(p.engine, "stop", "-t", fmt.Sprintf("%d", secs), p.containerName)
	_ = cmd.Run()
	rmCmd := exec.Command(p.engine, "rm", "-f", p.containerName)
	_ = rmCmd.Run()
	return nil
}

func (p *containerProcess) Pid() int {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// ResolveContainerImage determines the base container image based on language and version.
func ResolveContainerImage(language, runtimeVersion string) string {
	lang := strings.ToLower(language)
	major := parseMajorVersion(runtimeVersion)

	switch lang {
	case "node", "javascript", "typescript":
		if major != "" {
			return fmt.Sprintf("node:%s-alpine", major)
		}
		return "node:20-alpine"
	case "go", "golang":
		if runtimeVersion != "" {
			ver := strings.TrimPrefix(runtimeVersion, "go")
			return fmt.Sprintf("golang:%s-alpine", ver)
		}
		return "golang:1.24-alpine"
	case "python":
		pyVer := parseMajorMinorVersion(runtimeVersion)
		if pyVer != "" {
			return fmt.Sprintf("python:%s-slim", pyVer)
		}
		return "python:3.12-slim"
	case "rust":
		return "rust:alpine"
	case "java":
		if major != "" {
			return fmt.Sprintf("eclipse-temurin:%s-jdk-alpine", major)
		}
		return "eclipse-temurin:21-jdk-alpine"
	case "ruby":
		if major != "" {
			return fmt.Sprintf("ruby:%s-alpine", major)
		}
		return "ruby:3.3-alpine"
	case "php":
		if major != "" {
			return fmt.Sprintf("php:%s-cli-alpine", major)
		}
		return "php:8.3-cli-alpine"
	case "elixir":
		return "elixir:1.16-alpine"
	default:
		return "alpine:latest"
	}
}

func cleanVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "^")
	v = strings.TrimPrefix(v, ">=")
	v = strings.TrimPrefix(v, "~")
	return v
}

func parseMajorVersion(version string) string {
	v := cleanVersion(version)
	parts := strings.Split(v, ".")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

func parseMajorMinorVersion(version string) string {
	v := cleanVersion(version)
	parts := strings.Split(v, ".")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return parts[0] + "." + parts[1]
	}
	if len(parts) == 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

func generateContainerName(component, phase string) string {
	clean := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "/", "-")
		s = strings.ReplaceAll(s, "_", "-")
		s = strings.ReplaceAll(s, ".", "-")
		return s
	}
	c := clean(component)
	if c == "" {
		c = "app"
	}
	p := clean(phase)
	if p == "" {
		p = "run"
	}
	return fmt.Sprintf("octo-%s-%s-%d", c, p, time.Now().UnixNano()%1000000)
}
