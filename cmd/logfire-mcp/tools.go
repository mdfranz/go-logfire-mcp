package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type QueryRunInput struct {
	Query        string `json:"query"`
	MinTimestamp string `json:"min_timestamp"`
	MaxTimestamp string `json:"max_timestamp,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

func registerTools(s *mcp.Server, client *logfire.Client, maxResultBytes int64) {
	// Register query_run tool
	queryTool := &mcp.Tool{
		Name:        "query_run",
		Description: "Execute a DataFusion SQL query against Logfire telemetry data (records and metrics tables). Requires query and min_timestamp.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "DataFusion SQL query to execute against records or metrics tables.",
				},
				"min_timestamp": map[string]any{
					"type":        "string",
					"format":      "date-time",
					"description": "Minimum RFC3339 timestamp bound for the query (e.g. 2026-01-01T00:00:00Z). Required.",
				},
				"max_timestamp": map[string]any{
					"type":        "string",
					"format":      "date-time",
					"description": "Optional maximum RFC3339 timestamp bound for the query.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     10000,
					"description": "Optional maximum number of rows to return (1-10000).",
				},
			},
			"required": []string{"query", "min_timestamp"},
		},
	}

	s.AddTool(queryTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input QueryRunInput
		if err := decodeStrictArgs(req.Params.Arguments, &input); err != nil {
			return errorResult(fmt.Errorf("invalid tool arguments: %w", err)), nil
		}

		queryInput := logfire.QueryInput{
			SQL:          input.Query,
			MinTimestamp: input.MinTimestamp,
			MaxTimestamp: input.MaxTimestamp,
			Limit:        input.Limit,
		}

		if err := queryInput.Validate(); err != nil {
			slog.Error("query validation failed", "error", err)
			return errorResult(err), nil
		}

		slog.Debug("executing query", "sql", input.Query, "min_timestamp", input.MinTimestamp, "max_timestamp", input.MaxTimestamp, "limit", input.Limit)

		startTime := time.Now()
		res, err := client.Query(ctx, queryInput, "json")
		duration := time.Since(startTime)

		if err != nil {
			slog.Error("query execution failed", "error", err, "duration_ms", duration.Milliseconds())
			return errorResult(err), nil
		}

		slog.Debug("query completed", "duration_ms", duration.Milliseconds(), "records", logfire.CountResultRows(res, "json"), "result_bytes", len(res))

		if maxResultBytes > 0 && int64(len(res)) > maxResultBytes {
			return errorResult(fmt.Errorf("result size (%d bytes) exceeds maximum MCP tool response limit of %d bytes; try setting limit, selecting fewer columns, or narrowing time range", len(res), maxResultBytes)), nil
		}

		return textResult(res), nil
	})

	// Register get_schema_metadata tool
	schemaTool := &mcp.Tool{
		Name:        "get_schema_metadata",
		Description: "Get complete schema documentation for Logfire telemetry tables (records and metrics) and DataFusion SQL usage examples.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	s.AddTool(schemaTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return textResult(logfire.SchemaMetadata()), nil
	})

	// Register logfire://schema Resource
	s.AddResource(&mcp.Resource{
		URI:         "logfire://schema",
		Name:        "Logfire Database Schema",
		Description: "Markdown documentation of records and metrics table schemas and DataFusion SQL filter patterns.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "logfire://schema",
					MIMEType: "text/markdown",
					Text:     logfire.SchemaMetadata(),
				},
			},
		}, nil
	})
}

func decodeStrictArgs[T any](raw json.RawMessage, target *T) error {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("unexpected trailing data in tool arguments")
	}
	return nil
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Error: %v", err),
			},
		},
	}
}
