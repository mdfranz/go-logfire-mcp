package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupTestServer(t *testing.T) (*mcp.ClientSession, func()) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"schema":{},"data":[{"status":"ok"}]}`))
	}))

	cfg := &logfire.Config{
		APIToken:         "dummy-token",
		BaseURL:          httpServer.URL,
		Timeout:          5 * time.Second,
		MaxRetries:       0,
		MaxResponseBytes: 1024 * 1024,
	}
	logfireClient := logfire.NewClient(cfg)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "logfire-mcp-test",
		Version: "test",
	}, nil)

	registerTools(server, logfireClient, 1024*1024)

	ct, st := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = server.Run(ctx, st)
	}()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0",
	}, nil)

	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		cancel()
		httpServer.Close()
		t.Fatalf("failed to connect client to server: %v", err)
	}

	cleanup := func() {
		session.Close()
		cancel()
		httpServer.Close()
	}

	return session, cleanup
}

func TestMCPToolsListAndCall(t *testing.T) {
	session, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// List tools
	toolsRes, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}

	foundQueryRun := false
	foundGetSchema := false
	for _, tool := range toolsRes.Tools {
		if tool.Name == "query_run" {
			foundQueryRun = true
		}
		if tool.Name == "get_schema_metadata" {
			foundGetSchema = true
		}
	}

	if !foundQueryRun || !foundGetSchema {
		t.Fatalf("missing registered tools: query_run=%v, get_schema_metadata=%v", foundQueryRun, foundGetSchema)
	}

	// Call query_run
	args := map[string]any{
		"query":         "SELECT 1",
		"min_timestamp": "2026-01-01T00:00:00Z",
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_run",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if res.IsError {
		tc, _ := res.Content[0].(*mcp.TextContent)
		t.Fatalf("expected success, got error result: %s", tc.Text)
	}

	if len(res.Content) == 0 {
		t.Fatalf("expected tool content")
	}

	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(tc.Text, `"data":[{"status":"ok"}]`) {
		t.Errorf("unexpected content: %v", res.Content[0])
	}
}

func TestMCPUnknownFieldRejection(t *testing.T) {
	session, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	args := map[string]any{
		"query":         "SELECT 1",
		"min_timestamp": "2026-01-01T00:00:00Z",
		"unknown_field": "bad",
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_run",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if !res.IsError {
		t.Fatalf("expected IsError=true for unknown field")
	}

	tc := res.Content[0].(*mcp.TextContent)
	if !strings.Contains(tc.Text, "unknown field") && !strings.Contains(tc.Text, "unknown_field") {
		t.Errorf("expected error message to mention unknown field, got: %s", tc.Text)
	}
}

func TestMCPResourceRead(t *testing.T) {
	session, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "logfire://schema",
	})
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}

	if len(res.Contents) == 0 {
		t.Fatalf("expected resource content")
	}

	trc := res.Contents[0]
	if !strings.Contains(trc.Text, "Logfire Schema Reference") {
		t.Errorf("unexpected resource text content")
	}
}
