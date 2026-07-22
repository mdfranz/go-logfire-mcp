# Go Package Documentation

This document describes the Go package structure and API contracts for `go-logfire-mcp`.

## Package Overview

```
go-logfire-mcp/
├── internal/logfire/       # Core shared business logic & API client
├── cmd/logfire-cli/        # Command-line interface binary
└── cmd/logfire-mcp/        # Model Context Protocol stdio server binary
```

---

## `internal/logfire`

The `internal/logfire` package provides the core configuration loading, HTTP REST client, typed input validation, error handling, and schema embedding used by both binaries.

### Key Types & Functions

#### `Config` (`config.go`)
- Loads configuration from environment variables (`LOGFIRE_API_TOKEN`, `LOGFIRE_READ_TOKEN`, `LOGFIRE_API_KEY`, `LOGFIRE_REGION`, `LOGFIRE_BASE_URL`).
- Auto-detects region (`us` vs `eu`) from token prefix pattern `pylf_v[0-9]+_(?P<region>[a-z]+)_` if `LOGFIRE_REGION` is not explicitly set.
- Enforces strict mutual exclusion between `LOGFIRE_REGION` and `LOGFIRE_BASE_URL`.
- Restricts unencrypted `http://` base URLs to loopback hosts (`localhost`, `127.0.0.1`, `::1`).

#### `Client` (`client.go`)
- `NewClient(cfg *Config) *Client`: Creates a client with default HTTP timeouts.
- `Query(ctx context.Context, input QueryInput, format string) (string, error)`: Executes queries against `POST /v2/query`.
- Implements exponential backoff with random jitter for transient errors (`429`, `500-599`).
- Parses and respects `Retry-After` headers (both integer seconds and HTTP dates).
- Enforces configurable response body limits (16 MiB default for client, 64 KiB cap for error bodies).

#### `QueryInput` (`schemas.go`)
- Fields: `SQL` (`sql`), `MinTimestamp` (`min_timestamp`), `MaxTimestamp` (`max_timestamp`), `Limit` (`limit`), `IncludeSchema` (`include_schema`).
- `Validate() error`: Validates non-empty SQL strings (max 64 KiB), required RFC3339 `min_timestamp`, optional `max_timestamp > min_timestamp`, and row limit range (1–10,000).

#### `APIError` (`errors.go`)
- Struct containing `StatusCode`, `Message`, and `ResponseBody`.
- `Retryable() bool`: Returns `true` for status `429` and `500-599`.

#### `SchemaMetadata` (`schema.go`)
- Returns the embedded `schema.md` documentation string packaged via `//go:embed schema.md`.

---

## `cmd/logfire-cli`

The `cmd/logfire-cli` package implements the command-line interface.

- `run(args []string, stdout, stderr io.Writer) int`: Testable entry point executing the `query` subcommand.
- Returns exit code `0` on success, `1` on API or configuration errors, and `2` on usage errors.
- Writes raw query results straight to `stdout` without re-serializing.
- Usage and `--help` flags exit successfully without requiring an API token.

---

## `cmd/logfire-mcp`

The `cmd/logfire-mcp` package implements the Model Context Protocol stdio server using `github.com/modelcontextprotocol/go-sdk`.

- `run() int`: Server entry point managing signal contexts, PID locking (`LOGFIRE_MCP_LOCKFILE`), structured logging (`LOGFIRE_MCP_LOGFILE`), and `mcp.StdioTransport`.
- `registerTools(s *mcp.Server, client *logfire.Client, maxResultBytes int64)`:
  - Registers `query_run` tool with strict unknown parameter rejection via `json.NewDecoder.DisallowUnknownFields()`.
  - Registers `get_schema_metadata` tool for offline schema lookup.
  - Registers static `logfire://schema` resource (`text/markdown`).
- Invariant: Nothing writes to `stdout` except the MCP stdio transport handler. All server logs go to `stderr` or a configured log file.
