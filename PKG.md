# External Packages & Dependencies

This document lists external libraries and packages used by `go-logfire-mcp`.

## Go Direct Dependencies

### [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) (`v1.6.1`)
- **Description**: Official Go SDK for the Model Context Protocol (MCP).
- **Usage**:
  - MCP server initialization (`mcp.NewServer`) in [cmd/logfire-mcp/main.go](cmd/logfire-mcp/main.go).
  - Stdio transport execution (`&mcp.StdioTransport{}`).
  - Typed tool registration (`AddTool`) and resource registration (`AddResource`) in [cmd/logfire-mcp/tools.go](cmd/logfire-mcp/tools.go).
  - In-memory transport testing (`mcp.NewInMemoryTransports`, `mcp.NewClient`) in [cmd/logfire-mcp/tools_test.go](cmd/logfire-mcp/tools_test.go).

## Go Indirect Dependencies

- **`golang.org/x/oauth2`**: Transitive dependency for OAuth2 token handling.
- **`golang.org/x/sys`**: Transitive dependency providing low-level OS system call primitives.

## Python Test Harness Dependencies

The end-to-end test script in [tools/test_mcp.py](tools/test_mcp.py) uses PEP 723 inline dependency metadata:

- **`pydantic-ai`**: Agentic AI framework used to run live LLM evaluation tests against the `logfire-mcp` server via `pydantic_ai.mcp.MCPToolset` and `StdioTransport`.
- **`mcp`**: Official Python Model Context Protocol SDK used for protocol-level assertions and stdio communication.
- **`httpx`**: Asynchronous HTTP client library used by `pydantic-ai`.

## Third-Party Go Dependency Policy

The core client ([internal/logfire/](internal/logfire/)) and command-line binary ([cmd/logfire-cli/](cmd/logfire-cli/)) deliberately use **zero third-party dependencies**. All HTTP requests, JSON marshaling, rate-limit retries, structured logging (`log/slog`), signal handling (`os/signal`), and CLI parsing (`flag`) rely exclusively on the Go standard library. The CLI and MCP binaries each write to their own configurable append-only log target, with DEBUG enabled by default.
