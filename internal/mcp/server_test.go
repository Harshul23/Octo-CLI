package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerInitialize(t *testing.T) {
	s := NewServer()

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}` + "\n"
	in := strings.NewReader(req)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	if result.ProtocolVersion != ProtocolVersion {
		t.Fatalf("protocolVersion=%q, want %q", result.ProtocolVersion, ProtocolVersion)
	}
	if result.ServerInfo.Name != "octo" {
		t.Fatalf("server name=%q, want octo", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil || result.Capabilities.Resources == nil {
		t.Fatal("expected tools and resources capabilities")
	}
}

func TestServerPingAndNotifications(t *testing.T) {
	s := NewServer()

	// Notification followed by a ping
	input := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":42,"method":"ping"}` + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.ID != float64(42) {
		t.Fatalf("id=%v, want 42", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServerToolsList(t *testing.T) {
	s := NewServer()

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	in := strings.NewReader(req)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal(resp.Result)
	var res struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}

	if len(res.Tools) < 8 {
		t.Fatalf("expected at least 8 tools, got %d", len(res.Tools))
	}

	toolMap := make(map[string]bool)
	for _, tool := range res.Tools {
		toolMap[tool.Name] = true
	}

	for _, expected := range []string{"octo_inspect", "octo_topology", "octo_plan", "octo_run_and_verify", "octo_verify", "octo_diagnose", "octo_env", "octo_preview"} {
		if !toolMap[expected] {
			t.Fatalf("missing expected tool: %s", expected)
		}
	}
}

func TestServerToolsCallInspectAndPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()

	// 1. Test octo_inspect
	inspectReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_inspect",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	inspectData, _ := json.Marshal(inspectReq)

	// 2. Test octo_plan
	planReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_plan",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	planData, _ := json.Marshal(planReq)

	input := string(inspectData) + "\n" + string(planData) + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(&out)

	// Verify inspect response
	var resp1 Response
	if err := dec.Decode(&resp1); err != nil {
		t.Fatal(err)
	}
	if resp1.Error != nil {
		t.Fatalf("inspect error: %v", resp1.Error)
	}
	data1, _ := json.Marshal(resp1.Result)
	var toolRes1 CallToolResult
	_ = json.Unmarshal(data1, &toolRes1)
	if toolRes1.IsError || len(toolRes1.Content) == 0 {
		t.Fatalf("unexpected inspect result: %#v", toolRes1)
	}
	if !strings.Contains(toolRes1.Content[0].Text, "demo") {
		t.Fatalf("expected output to contain project name 'demo': %s", toolRes1.Content[0].Text)
	}

	// Verify plan response
	var resp2 Response
	if err := dec.Decode(&resp2); err != nil {
		t.Fatal(err)
	}
	if resp2.Error != nil {
		t.Fatalf("plan error: %v", resp2.Error)
	}
	data2, _ := json.Marshal(resp2.Result)
	var toolRes2 CallToolResult
	_ = json.Unmarshal(data2, &toolRes2)
	if toolRes2.IsError || len(toolRes2.Content) == 0 {
		t.Fatalf("unexpected plan result: %#v", toolRes2)
	}
	if !strings.Contains(toolRes2.Content[0].Text, "go run") {
		t.Fatalf("expected plan output to contain 'go run': %s", toolRes2.Content[0].Text)
	}
}

func TestServerResources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()

	// 1. List resources
	listReq := `{"jsonrpc":"2.0","id":20,"method":"resources/list"}` + "\n"

	// 2. Read topology resource
	readReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      21,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": "octo://topology?path=" + root,
		},
	}
	readData, _ := json.Marshal(readReq)

	input := listReq + string(readData) + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(&out)

	var respList Response
	if err := dec.Decode(&respList); err != nil {
		t.Fatal(err)
	}
	if respList.Error != nil {
		t.Fatalf("list error: %v", respList.Error)
	}

	var respRead Response
	if err := dec.Decode(&respRead); err != nil {
		t.Fatal(err)
	}
	if respRead.Error != nil {
		t.Fatalf("read error: %v", respRead.Error)
	}

	dataRead, _ := json.Marshal(respRead.Result)
	var readResult ReadResourceResult
	_ = json.Unmarshal(dataRead, &readResult)
	if len(readResult.Contents) == 0 {
		t.Fatal("expected resource contents")
	}
	if !strings.Contains(readResult.Contents[0].Text, "component:demo") {
		t.Fatalf("expected topology to contain 'component:demo': %s", readResult.Contents[0].Text)
	}
}

func TestServerToolsCallRunAndDiagnose(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()

	// 1. Diagnose
	diagReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      30,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_diagnose",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	diagData, _ := json.Marshal(diagReq)

	// 2. Run and verify
	runReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      31,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_run_and_verify",
			"arguments": map[string]interface{}{
				"path":   root,
				"detach": true,
			},
		},
	}
	runData, _ := json.Marshal(runReq)

	input := string(diagData) + "\n" + string(runData) + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(&out)

	var respDiag Response
	if err := dec.Decode(&respDiag); err != nil {
		t.Fatal(err)
	}
	if respDiag.Error != nil {
		t.Fatalf("diagnose error: %v", respDiag.Error)
	}
	dataDiag, _ := json.Marshal(respDiag.Result)
	var toolDiag CallToolResult
	_ = json.Unmarshal(dataDiag, &toolDiag)
	if toolDiag.IsError || len(toolDiag.Content) == 0 {
		t.Fatalf("unexpected diagnose result: %#v", toolDiag)
	}
	if !strings.Contains(toolDiag.Content[0].Text, "language") {
		t.Fatalf("expected diagnosis to explain language: %s", toolDiag.Content[0].Text)
	}

	var respRun Response
	if err := dec.Decode(&respRun); err != nil {
		t.Fatal(err)
	}
	if respRun.Error != nil {
		t.Fatalf("run error: %v", respRun.Error)
	}
	dataRun, _ := json.Marshal(respRun.Result)
	var toolRun CallToolResult
	_ = json.Unmarshal(dataRun, &toolRun)
	if toolRun.IsError || len(toolRun.Content) == 0 {
		t.Fatalf("unexpected run result: %#v", toolRun)
	}
	if !strings.Contains(toolRun.Content[0].Text, `"success": true`) {
		t.Fatalf("expected run output to show success: %s", toolRun.Content[0].Text)
	}
}

func TestServerToolsCallTopologyAndVerify(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()

	// 1. Topology
	topoReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      40,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_topology",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	topoData, _ := json.Marshal(topoReq)

	// 2. Verify
	verifyReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      41,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_verify",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	verifyData, _ := json.Marshal(verifyReq)

	input := string(topoData) + "\n" + string(verifyData) + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(&out)

	var respTopo Response
	if err := dec.Decode(&respTopo); err != nil {
		t.Fatal(err)
	}
	if respTopo.Error != nil {
		t.Fatalf("topology error: %v", respTopo.Error)
	}
	dataTopo, _ := json.Marshal(respTopo.Result)
	var toolTopo CallToolResult
	_ = json.Unmarshal(dataTopo, &toolTopo)
	if toolTopo.IsError || len(toolTopo.Content) == 0 {
		t.Fatalf("unexpected topology result: %#v", toolTopo)
	}
	if !strings.Contains(toolTopo.Content[0].Text, "component:demo") {
		t.Fatalf("expected topology to contain 'component:demo': %s", toolTopo.Content[0].Text)
	}

	var respVerify Response
	if err := dec.Decode(&respVerify); err != nil {
		t.Fatal(err)
	}
	if respVerify.Error != nil {
		t.Fatalf("verify error: %v", respVerify.Error)
	}
	dataVerify, _ := json.Marshal(respVerify.Result)
	var toolVerify CallToolResult
	_ = json.Unmarshal(dataVerify, &toolVerify)
	if toolVerify.IsError || len(toolVerify.Content) == 0 {
		t.Fatalf("unexpected verify result: %#v", toolVerify)
	}
	if !strings.Contains(toolVerify.Content[0].Text, `"project_name": "demo"`) {
		t.Fatalf("expected verify result to contain project name 'demo': %s", toolVerify.Content[0].Text)
	}
}

func TestServerToolsCallEnvAndPreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer()

	// 1. Env
	envReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      50,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_env",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	envData, _ := json.Marshal(envReq)

	// 2. Preview
	prevReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      51,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "octo_preview",
			"arguments": map[string]interface{}{
				"path": root,
			},
		},
	}
	prevData, _ := json.Marshal(prevReq)

	input := string(envData) + "\n" + string(prevData) + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(&out)

	var respEnv Response
	if err := dec.Decode(&respEnv); err != nil {
		t.Fatal(err)
	}
	if respEnv.Error != nil {
		t.Fatalf("env error: %v", respEnv.Error)
	}
	dataEnv, _ := json.Marshal(respEnv.Result)
	var toolEnv CallToolResult
	_ = json.Unmarshal(dataEnv, &toolEnv)
	if toolEnv.IsError || len(toolEnv.Content) == 0 {
		t.Fatalf("unexpected env result: %#v", toolEnv)
	}
	if !strings.Contains(toolEnv.Content[0].Text, "total_variables") {
		t.Fatalf("expected env result to contain 'total_variables': %s", toolEnv.Content[0].Text)
	}

	var respPrev Response
	if err := dec.Decode(&respPrev); err != nil {
		t.Fatal(err)
	}
	if respPrev.Error != nil {
		t.Fatalf("preview error: %v", respPrev.Error)
	}
	dataPrev, _ := json.Marshal(respPrev.Result)
	var toolPrev CallToolResult
	_ = json.Unmarshal(dataPrev, &toolPrev)
	if toolPrev.IsError || len(toolPrev.Content) == 0 {
		t.Fatalf("unexpected preview result: %#v", toolPrev)
	}
	if !strings.Contains(toolPrev.Content[0].Text, `"project_name": "demo"`) {
		t.Fatalf("expected preview result to contain project name 'demo': %s", toolPrev.Content[0].Text)
	}
	if !strings.Contains(toolPrev.Content[0].Text, "runtime_check") {
		t.Fatalf("expected preview result to contain runtime_check: %s", toolPrev.Content[0].Text)
	}
}

func TestServerUnknownMethod(t *testing.T) {
	s := NewServer()

	req := `{"jsonrpc":"2.0","id":99,"method":"foo/bar"}` + "\n"
	in := strings.NewReader(req)
	var out bytes.Buffer

	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Fatalf("error code=%d, want %d", resp.Error.Code, CodeMethodNotFound)
	}
}
