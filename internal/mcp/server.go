package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type Server struct {
	mu        sync.Mutex
	tools     []Tool
	resources []Resource
}

func NewServer() *Server {
	return &Server{
		tools:     DefaultTools(),
		resources: DefaultResources(),
	}
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	enc := json.NewEncoder(out)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(line) == 0 {
					return nil
				}
			} else {
				return err
			}
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(enc, NewErrorResponse(nil, CodeParseError, "Parse error: "+err.Error(), nil))
			continue
		}

		// Handle notifications (messages without an ID)
		if req.ID == nil {
			s.handleNotification(ctx, req)
			continue
		}

		resp := s.handleRequest(ctx, req)
		if err := s.writeResponse(enc, resp); err != nil {
			return err
		}
	}
}

func (s *Server) writeResponse(enc *json.Encoder, resp Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return enc.Encode(resp)
}

func (s *Server) handleNotification(ctx context.Context, req Request) {
	// Handled notifications like "notifications/initialized" require no response
	switch req.Method {
	case "notifications/initialized":
		// Handshake complete
	default:
		// Silently ignore unknown notifications as per JSON-RPC spec
	}
}

func (s *Server) handleRequest(ctx context.Context, req Request) Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "ping":
		return NewSuccessResponse(req.ID, map[string]interface{}{})
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(ctx, req)
	default:
		return NewErrorResponse(req.ID, CodeMethodNotFound, fmt.Sprintf("method %q not found", req.Method), nil)
	}
}

func (s *Server) handleInitialize(req Request) Response {
	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "octo",
			Version: "1.0.0",
		},
	}
	return NewSuccessResponse(req.ID, result)
}

func (s *Server) handleToolsList(req Request) Response {
	return NewSuccessResponse(req.ID, map[string]interface{}{
		"tools": s.tools,
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params CallToolParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, CodeInvalidParams, "invalid tools/call params: "+err.Error(), nil)
		}
	}

	result, err := ExecuteTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return NewErrorResponse(req.ID, CodeInternalError, "tool execution failed: "+err.Error(), nil)
	}

	return NewSuccessResponse(req.ID, result)
}

func (s *Server) handleResourcesList(req Request) Response {
	return NewSuccessResponse(req.ID, map[string]interface{}{
		"resources": s.resources,
	})
}

func (s *Server) handleResourcesRead(ctx context.Context, req Request) Response {
	var params ReadResourceParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, CodeInvalidParams, "invalid resources/read params: "+err.Error(), nil)
		}
	}

	result, err := ReadResource(ctx, params.URI)
	if err != nil {
		return NewErrorResponse(req.ID, CodeInvalidParams, err.Error(), nil)
	}

	return NewSuccessResponse(req.ID, result)
}
