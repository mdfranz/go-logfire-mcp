# go-logfire-mcp

A Go CLI tool (`logfire-cli`) and Model Context Protocol (MCP) server (`logfire-mcp`) for querying telemetry data stored in [Logfire](https://logfire.pydantic.dev) via its direct `POST /v2/query` REST API.

## Architecture

This project uses a shared-core pattern where business logic, API communication, and validation live in `internal/logfire/`, and the two binaries act as thin transport adapters:

- **`logfire-cli`** (`cmd/logfire-cli/`): Command-line tool for direct terminal queries returning JSON or CSV.
- **`logfire-mcp`** (`cmd/logfire-mcp/`): MCP stdio server providing tools and schema resources for AI assistants (`pydantic-ai`, Claude, Gemini).
- **`internal/logfire/`**: Core HTTP client, configuration, schema metadata, input validation, and error handling.

For complete package-level documentation, see [PKG.md](PKG.md).

## Prerequisites

- Go 1.22+
- A Logfire project read token or API key (`LOGFIRE_API_TOKEN`, `LOGFIRE_READ_TOKEN`, or `LOGFIRE_API_KEY`).

## Build & Install

```bash
# Build both binaries to the project root directory
make build

# Run unit tests
make test

# Install binaries to ~/.local/bin (override with INSTALL_DIR=/custom/path)
make install
```

## Configuration

Configuration is set via environment variables:

| Variable | Default | Description |
|---|---|---|
| `LOGFIRE_API_TOKEN` | Required | Logfire read token or API key. Also accepts `LOGFIRE_READ_TOKEN` or `LOGFIRE_API_KEY`. |
| `LOGFIRE_REGION` | `us` | Region: `us` or `eu`. Auto-inferred if token prefix is `pylf_v1_eu_...`. |
| `LOGFIRE_BASE_URL` | Optional | Custom base URL override (used for testing against mock servers). |
| `LOGFIRE_MCP_LOGFILE` | `logfire-mcp.log` | MCP server log target: `stderr`, `off`, or an append-only file path. |
| `LOGFIRE_MCP_DEBUG` | `true` | Emit DEBUG logs by default; set to `false`, `off`, or `0` for INFO level. |
| `LOGFIRE_CLI_LOGFILE` | `logfire-cli.log` | CLI log target: `stderr`, `off`, or an append-only file path. |
| `LOGFIRE_CLI_DEBUG` | `true` | Emit DEBUG logs by default; set to `false`, `off`, or `0` for INFO level. |
| `LOGFIRE_MAX_RETRIES` | `3` | Maximum retry attempts for transient API errors & HTTP 429 rate limits. |
| `LOGFIRE_MCP_LOCKFILE` | `off` | Lockfile path for single-instance PID locking (e.g. `/tmp/logfire-mcp.lock`). |
| `LOGFIRE_MCP_MAX_RESULT_BYTES` | `1048576` | Maximum size cap (in bytes) for MCP tool response strings (default 1 MiB). |

The CLI and MCP server use separate log files so their independent processes do not interleave output. API read tokens and query result payloads are never written to logs. When DEBUG logging is enabled, debug log entries record the SQL query text, parameters, execution latency (`duration_ms`), returned row count (`records`), and byte size (`result_bytes`). Use `stderr` when a process supervisor should collect logs instead.

## CLI Usage (`logfire-cli`)

```bash
# General help (does not require an API token)
./logfire-cli --help

# Query records in JSON format (default)
export LOGFIRE_API_TOKEN="pylf_v1_us_..."
./logfire-cli query \
  --sql "SELECT start_timestamp, service_name, message FROM records ORDER BY start_timestamp DESC LIMIT 5" \
  --min-timestamp "2026-01-01T00:00:00Z"

# Grouping query in CSV format
./logfire-cli query \
  --sql "SELECT service_name, count(*) as total FROM records GROUP BY service_name ORDER BY total DESC" \
  --min-timestamp "2026-01-01T00:00:00Z" \
  --format csv
```

### CLI Flags for `query`

- `--sql` (required): DataFusion SQL query string.
- `--min-timestamp` (required): Minimum timestamp filter in RFC3339 format (e.g. `2026-01-01T00:00:00Z`).
- `--max-timestamp` (optional): Upper bound timestamp filter in RFC3339 format.
- `--limit` (optional): Row limit integer (1–10,000).
- `--format` (optional): Output format: `json` (default) or `csv`.

## MCP Server Capabilities (`logfire-mcp`)

### Tools

- **`query_run`**: Executes DataFusion SQL queries against Logfire `records` or `metrics` tables.
  - Parameters: `query` (string, required), `min_timestamp` (string RFC3339, required), `max_timestamp` (string RFC3339, optional), `limit` (integer, optional).
  - Enforces strict unknown field rejection (`DisallowUnknownFields`) and returns structured MCP error results (`IsError: true`) on query failures.
- **`get_schema_metadata`**: Returns embedded Markdown documentation of table schemas, column types, and common DataFusion SQL query patterns with zero network overhead.

### Resources

- **`logfire://schema`**: Static `text/markdown` resource containing the complete Logfire database schema reference.

### MCP Client Configuration

An [`.mcp.json`](.mcp.json) file is included for MCP clients (e.g. Claude Code) that auto-discover project-scoped servers. Update the `command` path to point at your built `logfire-mcp` binary and set `LOGFIRE_API_TOKEN` in your environment.

## Testing

```bash
# Run unit tests across all packages
make test

# Run end-to-end Python harness test using pydantic-ai
make test-e2e
```

The test harness in [tools/test_mcp.py](tools/test_mcp.py) runs deterministic protocol tests against a mock server when offline, or live `pydantic-ai` agent tests when an LLM API key is present.

## License

MIT
