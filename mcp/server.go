// Package mcp provides a Model Context Protocol server for tidal.
//
// This allows AI agents (Claude Code, etc.) to use tidal natively
// without shelling out to the CLI.
//
// Usage in Claude Code settings:
//
//	{
//	  "mcpServers": {
//	    "tidal": {
//	      "command": "tidal",
//	      "args": ["mcp"]
//	    }
//	  }
//	}
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/oSEAItic/tidal/internal/config"
	"github.com/oSEAItic/tidal/internal/runner"
)

// Tool describes an MCP tool.
type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema JSONSchema `json:"inputSchema"`
}

type JSONSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]SchemaProperty `json:"properties,omitempty"`
}

type SchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Request is a JSON-RPC request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var tools = []Tool{
	{
		Name:        "tidal_status",
		Description: "Show configured capabilities for this repo",
		InputSchema: JSONSchema{Type: "object"},
	},
	{
		Name:        "tidal_test",
		Description: "Run test tasks and return structured results",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"name": {Type: "string", Description: "Specific test to run (optional)"},
			},
		},
	},
	{
		Name:        "tidal_lint",
		Description: "Run lint/rule checks and return structured results",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"name": {Type: "string", Description: "Specific lint to run (optional)"},
			},
		},
	},
	{
		Name:        "tidal_review",
		Description: "Analyze current changes before shipping (diff, secrets, TODOs)",
		InputSchema: JSONSchema{Type: "object"},
	},
	{
		Name:        "tidal_observe",
		Description: "Observe logs, issues, or CI status",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"kind": {Type: "string", Description: "What to observe: logs, issues, ci"},
			},
		},
	},
	{
		Name:        "tidal_grade",
		Description: "Run quality scoring tasks and return metrics",
		InputSchema: JSONSchema{Type: "object"},
	},
	{
		Name:        "tidal_topology",
		Description: "Show project service topology, paths, and external dependencies",
		InputSchema: JSONSchema{Type: "object"},
	},
}

// Serve runs the MCP server on stdin/stdout (JSON-RPC over stdio).
func Serve() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := handle(req)
		data, _ := json.Marshal(resp)
		fmt.Fprintf(os.Stdout, "%s\n", data)
	}
	return scanner.Err()
}

func handle(req Request) Response {
	switch req.Method {
	case "initialize":
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":   map[string]interface{}{"tools": map[string]bool{}},
				"serverInfo":     map[string]string{"name": "tidal", "version": "0.1.0"},
			},
		}

	case "tools/list":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"tools": tools}}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		json.Unmarshal(req.Params, &params)
		result, err := callTool(params.Name, params.Arguments)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -1, Message: err.Error()}}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": result},
			},
		}}

	default:
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	}
}

func callTool(name string, args map[string]interface{}) (string, error) {
	cfg, err := config.Load("tidal.yaml")
	if err != nil {
		return "", fmt.Errorf("cannot load tidal.yaml: %w", err)
	}

	var tasks []runner.Task
	command := ""

	switch name {
	case "tidal_status":
		// return status as JSON directly
		status := map[string]interface{}{
			"name": cfg.Name, "lang": cfg.Lang,
			"test": len(cfg.Test) > 0, "lint": len(cfg.Lint) > 0,
			"review": len(cfg.Review) > 0, "grade": len(cfg.Grade) > 0,
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		return string(data), nil

	case "tidal_test":
		command = "test"
		if n, ok := args["name"].(string); ok && n != "" {
			tasks = cfg.TestTasks(n)
		} else {
			tasks = cfg.TestTasks()
		}

	case "tidal_lint":
		command = "lint"
		if n, ok := args["name"].(string); ok && n != "" {
			tasks = cfg.LintTasks(n)
		} else {
			tasks = cfg.LintTasks()
		}

	case "tidal_review":
		command = "review"
		tasks = cfg.ReviewTasks()

	case "tidal_observe":
		command = "observe"
		kind := "issues"
		if k, ok := args["kind"].(string); ok && k != "" {
			kind = k
		}
		tasks = cfg.ObserveTasks(kind)

	case "tidal_grade":
		command = "grade"
		tasks = cfg.GradeTasks()

	case "tidal_topology":
		if cfg.Topology != nil {
			out := map[string]interface{}{
				"services": cfg.Topology.Services,
				"paths":    cfg.Paths,
				"external": cfg.External,
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			return string(data), nil
		}
		return "{}", nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	if len(tasks) == 0 {
		return fmt.Sprintf("{\"error\": \"%s not configured\"}", command), nil
	}

	// run tasks and capture results
	var results []runner.TaskResult
	for _, t := range tasks {
		results = append(results, runner.RunSingle(t))
	}
	data, _ := json.MarshalIndent(results, "", "  ")
	return string(data), nil
}
