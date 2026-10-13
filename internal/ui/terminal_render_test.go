package ui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

func ansiToHTML(input string) string {
	var sb strings.Builder
	ansiRe := regexp.MustCompile(`\x1b\[([0-9;]*)m`)

	parts := ansiRe.Split(input, -1)
	matches := ansiRe.FindAllStringSubmatch(input, -1)

	openSpans := 0
	for i, part := range parts {
		sb.WriteString(html.EscapeString(part))
		if i < len(matches) {
			codesStr := matches[i][1]
			if codesStr == "" || codesStr == "0" {
				for openSpans > 0 {
					sb.WriteString("</span>")
					openSpans--
				}
				continue
			}

			codes := strings.Split(codesStr, ";")
			var styles []string
			for j := 0; j < len(codes); j++ {
				c := codes[j]
				switch c {
				case "1":
					styles = append(styles, "font-weight: 700")
				case "38":
					if j+4 < len(codes) && codes[j+1] == "2" {
						r, _ := strconv.Atoi(codes[j+2])
						g, _ := strconv.Atoi(codes[j+3])
						b, _ := strconv.Atoi(codes[j+4])
						styles = append(styles, fmt.Sprintf("color: rgb(%d,%d,%d)", r, g, b))
						j += 4
					}
				case "48":
					if j+4 < len(codes) && codes[j+1] == "2" {
						r, _ := strconv.Atoi(codes[j+2])
						g, _ := strconv.Atoi(codes[j+3])
						b, _ := strconv.Atoi(codes[j+4])
						styles = append(styles, fmt.Sprintf("background-color: rgb(%d,%d,%d)", r, g, b))
						j += 4
					}
				}
			}
			if len(styles) > 0 {
				sb.WriteString(fmt.Sprintf(`<span style="%s">`, strings.Join(styles, "; ")))
				openSpans++
			}
		}
	}
	for openSpans > 0 {
		sb.WriteString("</span>")
		openSpans--
	}
	return sb.String()
}

func TestRenderTerminalSelectorHTML(t *testing.T) {
	model := intelligence.ProjectModel{
		Name:     "monorepo-smoke",
		Monorepo: true,
		Components: []intelligence.Component{
			{
				Name:      "web",
				Path:      "apps/web",
				Language:  "Node",
				Framework: "React",
				Port:      3000,
				ExecutionCandidates: []intelligence.ExecutionCandidate{
					{
						Command:    "npm run dev",
						Kind:       "dev",
						Confidence: 0.95,
					},
				},
			},
			{
				Name:      "api",
				Path:      "apps/api",
				Language:  "Node",
				Framework: "Express",
				Port:      8080,
				DependsOn: []string{"postgres"},
				ExecutionCandidates: []intelligence.ExecutionCandidate{
					{
						Command:    "npm run start",
						Kind:       "run",
						Confidence: 0.95,
					},
				},
			},
		},
		Services: []intelligence.Service{
			{
				Name:  "postgres",
				Image: "postgres:15-alpine",
				Ports: []string{"5432:5432"},
			},
		},
	}

	selModel := newComponentSelectorModel(model)
	viewANSI := selModel.View()

	htmlBody := ansiToHTML(viewANSI)

	htmlPage := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Octo Terminal Selector</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    background: #06090e;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 30px;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  }
  .terminal-window {
    width: 820px;
    background: #0d1117;
    border-radius: 10px;
    box-shadow: 0 25px 60px rgba(0,0,0,0.8), 0 0 20px rgba(121, 40, 202, 0.35);
    border: 1px solid rgba(121, 40, 202, 0.4);
    overflow: hidden;
  }
  .terminal-bar {
    background: #161b22;
    padding: 11px 16px;
    display: flex;
    align-items: center;
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }
  .traffic-lights {
    display: flex;
    gap: 8px;
  }
  .dot {
    width: 12px;
    height: 12px;
    border-radius: 50%%;
    display: inline-block;
  }
  .dot.red { background: #ff5f56; }
  .dot.yellow { background: #ffbd2e; }
  .dot.green { background: #27c93f; }
  .terminal-title {
    flex: 1;
    text-align: center;
    color: #8b949e;
    font-size: 12px;
    font-weight: 500;
    letter-spacing: 0.3px;
    margin-right: 52px;
  }
  .terminal-body {
    padding: 24px 28px;
    font-family: "SF Mono", "JetBrains Mono", Menlo, Monaco, Consolas, monospace;
    font-size: 13.5px;
    line-height: 1.5;
    color: #e6edf3;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
</head>
<body>
  <div class="terminal-window" id="terminal">
    <div class="terminal-bar">
      <div class="traffic-lights">
        <span class="dot red"></span>
        <span class="dot yellow"></span>
        <span class="dot green"></span>
      </div>
      <div class="terminal-title">octo run — monorepo-smoke — 92×26</div>
    </div>
    <div class="terminal-body">%s</div>
  </div>
</body>
</html>`, htmlBody)

	outDir := t.TempDir()
	if custom := os.Getenv("OCTO_UI_EXPORT_DIR"); custom != "" {
		outDir = custom
		_ = os.MkdirAll(outDir, 0o755)
	}

	outPath := filepath.Join(outDir, "terminal_selector.html")
	if err := os.WriteFile(outPath, []byte(htmlPage), 0o644); err != nil {
		t.Fatalf("failed to write HTML file: %v", err)
	}
	t.Logf("Rendered selector HTML to %s", outPath)
}
