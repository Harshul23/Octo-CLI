package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestMcpCommandStartsAndResponds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := mcpCmd
	var in bytes.Buffer
	var out bytes.Buffer

	in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n")

	cmd.SetIn(&in)
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetContext(ctx)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, `"protocolVersion":"2024-11-05"`) {
		t.Fatalf("expected protocolVersion in response, got: %s", output)
	}
	if !strings.Contains(output, `"name":"octo"`) {
		t.Fatalf("expected server name 'octo' in response, got: %s", output)
	}
}
