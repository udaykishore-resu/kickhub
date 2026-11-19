// Package mcp is a minimal Model Context Protocol server over stdio, so
// coding agents can browse the catalog and scaffold components.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/udaykishore-resu/kickhub/internal/catalog"
	"github.com/udaykishore-resu/kickhub/internal/engine"
)

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

var tools = []map[string]any{
	{
		"name":        "list_templates",
		"description": "Search the kickhub template catalog. Returns ids, targets, resources and parameters.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
			"query":  map[string]any{"type": "string", "description": "Free text, e.g. 'queue worker'"},
			"target": map[string]any{"type": "string", "enum": []string{"aws", "gcp", "local"}},
		}},
	},
	{
		"name":        "create_component",
		"description": "Render a template into a new kickstart repository directory.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"template", "target", "name", "out_dir"}, "properties": map[string]any{
			"template": map[string]any{"type": "string"},
			"target":   map[string]any{"type": "string", "enum": []string{"aws", "gcp", "local"}},
			"name":     map[string]any{"type": "string", "description": "Component name, lowercase-with-dashes"},
			"out_dir":  map[string]any{"type": "string", "description": "Directory to create; must be empty or missing"},
			"params":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
		}},
	},
}

// Serve handles newline-delimited JSON-RPC until in is closed.
func Serve(c *catalog.Catalog, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		if len(req.ID) == 0 {
			continue // notification: no reply expected
		}
		resp := response{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2024-11-05"
			}
			resp.Result = map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "kickhub", "version": "0.1.0"},
			}
		case "ping":
			resp.Result = map[string]any{}
		case "tools/list":
			resp.Result = map[string]any{"tools": tools}
		case "tools/call":
			text, err := call(c, req.Params)
			if err != nil {
				text = "error: " + err.Error()
			}
			resp.Result = map[string]any{
				"content": []map[string]any{{"type": "text", "text": text}},
				"isError": err != nil,
			}
		default:
			resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func call(c *catalog.Catalog, raw json.RawMessage) (string, error) {
	var p struct {
		Name      string `json:"name"`
		Arguments struct {
			Query    string            `json:"query"`
			Target   string            `json:"target"`
			Template string            `json:"template"`
			Name     string            `json:"name"`
			OutDir   string            `json:"out_dir"`
			Params   map[string]string `json:"params"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", err
	}
	a := p.Arguments
	switch p.Name {
	case "list_templates":
		b, err := json.MarshalIndent(c.Search(a.Query, a.Target), "", "  ")
		return string(b), err
	case "create_component":
		if a.OutDir == "" {
			return "", fmt.Errorf("out_dir is required")
		}
		res, err := engine.Render(c, engine.Request{Template: a.Template, Target: a.Target, Name: a.Name, Params: a.Params})
		if err != nil {
			return "", err
		}
		if err := res.WriteDir(a.OutDir); err != nil {
			return "", err
		}
		return fmt.Sprintf("Created %s (%s, target %s) in %s:\n%s", a.Name, a.Template, a.Target, a.OutDir, strings.Join(res.Paths(), "\n")), nil
	}
	return "", fmt.Errorf("unknown tool %q", p.Name)
}
