# Plan: `go-logfire-mcp` — Logfire CLI + MCP Server

## Context

The repo currently contains only a `refs/` directory with five reference documents (no Go code, no `go.mod`):
- `refs/MCP-SERVER-JULY-2026.md` — house style guide for Go MCP servers (`github.com/modelcontextprotocol/go-sdk v1.6.1`), covering bootstrap, config, logging, lockfiles, tool registration, client/retry patterns, and a phased build progression.
- `refs/SQL-MCP.md` — SQL MCP architecture reference covering Resources versus Tools, progressive schema disclosure, schema representation, and recoverable tool errors. For this project, its Resource/error guidance applies; its MySQL/SQLite catalog-introspection machinery does not.
- `refs/client-usage.md` — legacy examples for the former `GET /v1/query` API. They remain useful background for read-token auth and formats, but implementation follows the current official [Export Data API](https://pydantic.dev/docs/logfire/manage/query-api/) and the maintained client in the sibling `../logfire` checkout: `POST https://logfire-{us,eu}.pydantic.dev/v2/query`, JSON body, required `sql` and `min_timestamp`, and a project read token sent directly in the `Authorization` header (no `Bearer` prefix).
- `refs/schema.md` — the `records`/`metrics` table schema (DataFusion SQL dialect) to embed as offline MCP context.
- `refs/SKILL.md` — an existing Codex skill that frames the query workflow and supplies the desired `query_run` vocabulary and row-oriented MCP result convention. Its OAuth/project assumptions describe a different MCP implementation and must be adapted to this read-token-backed server.

Goal: build a **simple** Go CLI and MCP server for querying Logfire, following the guide's Phase 1–3 patterns (working server + test harness, production reliability, LLM ergonomics) while explicitly skipping Phase 5 (TTL caching, response compaction, default scope injection, singleflight) — this is a read-only, single-tenant query API that doesn't need that surface area.

Decisions made with the user:
- **Scope**: guide Phases 1–3 only.
- **CLI**: single `query` command exposing the core v2 endpoint fields (`--sql`, `--min-timestamp`, `--max-timestamp`, `--limit`, `--format json|csv`). No convenience subcommands.
- **Region**: inferred from the read-token prefix (`pylf_vN_<region>_`), matching the maintained Logfire client; legacy, malformed, or unknown-region tokens fall back to the US endpoint. `LOGFIRE_BASE_URL` remains the explicit test/advanced override, so no separate region variable is needed.
- **Auth**: `LOGFIRE_API_TOKEN`, required and fail-fast. Despite the neutral application-facing name, its value must be a project read token accepted by `/v2/query`, not a Logfire project API key.
- **Binaries**: two binaries (`logfire-mcp`, `logfire-cli`) sharing an `internal/logfire` core package per the guide's §2.1 shared-core pattern.
- **MCP contract**: keep the established `query_run` name, its `query` argument, and its `{columns, rows}` JSON result convention. The MCP adapter maps `query` to the REST client's internal `SQL` field, requests v2 JSON, and converts the upstream `{schema: {fields: [...]}, data: [...]}` envelope to `{columns: [...], rows: [...]}`. No `project` argument is exposed because a Logfire read token is already project-scoped.
- **Schema discovery**: expose the embedded Markdown schema as both a `get_schema_metadata` tool and a static `logfire://schema` Resource. Do not add `search_objects`; two static tables do not justify progressive catalog discovery.
- **Locking**: PID locking is supported but off by default because stdio MCP clients normally launch independent server processes with private pipes.

**Open assumption**: module path `github.com/mdfranz/go-logfire-mcp` (matches dir name / GitHub handle). Cheap to change later if wrong.

## Directory Layout

```
go-logfire-mcp/
  go.mod
  Makefile
  internal/logfire/
    config.go        # env-var loading + validation, shared by both binaries
    client.go         # POST /v2/query client: JSON body, retry/backoff, body caps, Accept header
    schemas.go         # QueryInput validation/body marshaling and AcceptHeaderFor()
    response.go        # v2 JSON decode + MCP-compatible columns/rows mapping
    schema.go          # //go:embed schema.md and exported SchemaMetadata
    schema.md          # packaged copy of refs/schema.md; equality-tested to prevent drift
    errors.go          # APIError type + Retryable()
    *_test.go           # table-driven tests per file
  cmd/logfire-mcp/
    main.go            # run() bootstrap: config, rotating logger, optional PID lockfile,
                        # signal.NotifyContext, mcp.NewServer + registerTools + s.Run
    tools.go            # typed mcp.AddTool handlers + logfire://schema Resource registration
    tools_test.go       # in-memory MCP protocol tests for tools, resource, validation, errors
  cmd/logfire-cli/
    main.go            # run(args, stdout, stderr) — flag.FlagSet for `query`, config load,
                        # Validate(), dispatch, error reporting to stderr with nonzero exit
    output.go           # writeResult/writeError — raw body passthrough to stdout, no re-serialization
  tools/
    test_mcp.py         # PEP 723 pydantic_ai subprocess harness (deterministic + optional agent tiers)
    fixture_server.py   # stdlib HTTP stub for POST /v2/query; no real token needed
```

## Core Design Points

**Config (`internal/logfire/config.go`)** — shared fields only (`LOGFIRE_API_TOKEN` required for actual queries; timeout/retry/maximum-response-byte settings with sane defaults). `LOGFIRE_API_TOKEN` must contain a Logfire project read token and is sent directly as `Authorization: <token>`; it is not a Logfire project API key and does not receive a `Bearer` prefix. In ordinary use, derive the regional base URL from tokens matching `^pylf_v[0-9]+_(?P<region>[a-z]+)_`: `us` selects `https://logfire-us.pydantic.dev`, `eu` selects `https://logfire-eu.pydantic.dev`, and a missing or unrecognized region falls back to US. `LOGFIRE_BASE_URL` is the sole explicit advanced/test override; it must be an absolute URL without userinfo/query/fragment and permits plain HTTP only for loopback hosts. The offline harness sets it to its fixture server and uses `LOGFIRE_API_TOKEN=dummy`. Project API keys are deliberately unsupported because the direct v2 query endpoint requires a read token; they authenticate the hosted MCP/platform APIs instead. MCP-only vars stay local to `cmd/logfire-mcp/main.go`: `LOGFIRE_MCP_LOGFILE` (default `"stderr"`, `"off"`, or an explicit rotating-file path), `LOGFIRE_MCP_DEBUG`, `LOGFIRE_MCP_MAX_RESULT_BYTES`, and `LOGFIRE_MCP_LOCKFILE`. The lockfile default is `"off"`; setting an explicit path opts into single-instance locking. Configuration precedence and accepted values are documented and tested. The CLI parses command/help flags before loading auth so `--help` works without a token.

**Client (`internal/logfire/client.go`)** — `Query(ctx, QueryInput, format) ([]byte, error)` JSON-marshals the input and issues `POST {baseURL}/v2/query`, setting the raw read token as `Authorization`, `Content-Type: application/json`, `Accept` per `AcceptHeaderFor(format)`, and a versioned `User-Agent` such as `go-logfire-mcp/<version>`. Although the transport method is POST, the operation is read-only and logically idempotent, so retry 429/5xx with bounded backoff+jitter while respecting both forms of `Retry-After`; never retry other 4xx responses. Preserve distinct API-error categories for 400 (query execution error) and 422 (invalid query request), while retaining status/body details for all other failures. Successful response bodies have a configurable shared cap (default 16 MiB); error bodies are capped at 64 KiB. The MCP adapter applies its own smaller result cap (default 1 MiB) to the final compatibility JSON before constructing tool content. Oversized results return an actionable error telling the caller to lower the limit, narrow the time range, or project fewer columns. Retry sleeps are context-cancellable, a fresh body reader is created for every attempt, response bodies are closed on every attempt, and tokens/query results are never logged. No singleflight or caching (per scope decision).

**Typed input (`internal/logfire/schemas.go`)** — `QueryInput` marshals to the v2 JSON body using `sql`, required `min_timestamp`, optional `max_timestamp`, optional `limit`, and the fixed compatibility fields `include_schema: true` and `explain: false`, matching the maintained Logfire client. `Validate()` checks only rules that can be enforced reliably without pretending to parse DataFusion SQL: non-empty SQL capped at 64 KiB; required RFC3339 `min_timestamp`; optional RFC3339 `max_timestamp`; `max_timestamp >= min_timestamp` when supplied (equal bounds are valid and return no rows); and optional `limit` either omitted (`0`, preserving the v2 REST default of 100) or between 1 and 10,000. Do not adopt the Python client's deprecated fallback timestamp when `min_timestamp` is omitted; require it for a forward-looking contract. Do not impose the old 14-day rule because the current direct-query documentation does not specify it. Do not reject pipes or semicolons with string matching: those characters can occur legitimately inside SQL strings/comments, and DataFusion can return more precise syntax errors. The read token and Logfire query endpoint are the read-only security boundary. The supported API also accepts `timezone` and `deployment_environment`, but leave them out of this deliberately minimal v1 CLI/MCP surface until there is a concrete use case.

**JSON compatibility mapping (`internal/logfire/response.go`)** — the CLI always writes the upstream response bytes unchanged. For MCP JSON results only, decode the upstream `{schema: {fields}, data}` response and marshal `{columns, rows}` to retain the established `query_run` contract and match Logfire's maintained Python query client. Populate `columns` from `schema.fields` and `rows` from `data`; recursively rename `data_type` to `datatype` and remove Arrow-internal `dict_id`, `dict_is_ordered`, and `metadata` keys from column descriptors. Treat a malformed successful v2 envelope as an actionable upstream-response error rather than returning ambiguous tool content.

**MCP capabilities (`cmd/logfire-mcp/tools.go`)** — use the v1.6.1 generic `mcp.AddTool` API so JSON Schema inference, `additionalProperties: false`, argument unmarshalling/schema validation, and handler-error-to-`IsError` conversion come from the SDK. Override inferred schemas where richer descriptions, RFC3339 formats, or numeric bounds are needed; call the shared `Validate()` for cross-field rules. Register:

- `query_run`: required input fields `query` and `min_timestamp`, plus optional `max_timestamp` and `limit`; map `query` to `QueryInput.SQL`, request `application/json`, and return the compatibility `{columns, rows}` JSON described above. Its description says to use DataFusion SQL, project needed columns, use `limit` as the outer response cap, and call schema discovery when unsure.
- `get_schema_metadata`: no-argument tool returning the embedded Markdown with zero network calls.
- `logfire://schema`: static `text/markdown` MCP Resource returning that same embedded content.

Both tools carry `ReadOnlyHint: true`; `query_run` is open-world because it calls Logfire, while schema discovery is closed-world. Progressive `search_objects` discovery is deliberately omitted because the complete schema has only two tables and is already compact.

**Logging and MCP bootstrap (`cmd/logfire-mcp/main.go`, `cmd/logfire-cli/logging.go`)** — `run() int` pattern, append-only `logfire-mcp.log` and `logfire-cli.log` defaults so independent processes do not interleave output. `LOGFIRE_MCP_LOGFILE` and `LOGFIRE_CLI_LOGFILE` accept `"stderr"`, `"off"`, or an explicit path; DEBUG is enabled by default and `false`, `off`, or `0` selects INFO level. The CLI logs query lifecycle metadata only (never SQL text, tokens, or result bodies); MCP logs resolved non-secret config values at startup. Keep the MCP stdout invariant: **nothing writes to stdout except the MCP stdio transport itself.** MCP uses an opt-in PID lockfile (default `"off"`; explicit path enables it), `signal.NotifyContext` for graceful shutdown, `mcp.NewServer`, and `s.Run(ctx, &mcp.StdioTransport{})`. Independent stdio server processes must work concurrently when locking is off; users who direct multiple instances to the same log path are expected to enable the shared lock or choose distinct paths. Log rotation remains a later enhancement.

**CLI (`cmd/logfire-cli/main.go`)** — `run(args, stdout, stderr io.Writer) int` (injectable for tests), single `query` command via stdlib `flag.FlagSet` (no cobra needed for one command), validates locally before any network call, writes the raw response body straight through to stdout (no re-parsing), and reports typed query-execution/query-request API errors and other failures to stderr with nonzero exit. `--min-timestamp` is required, matching v2. JSON returns the raw v2 `{schema, data}` envelope; CSV remains raw passthrough. Help/usage exits successfully without loading token configuration.

## Testing

- **Go unit tests** in `internal/logfire/`: token-to-base-URL inference (US, EU, legacy, and unknown region), `Validate()`/JSON-body table-driven cases including fixed `include_schema`/`explain` fields and equal timestamp bounds, `AcceptHeaderFor`, v2-to-compatibility response mapping, schema-embed/reference equality, response/error body limits, deterministic backoff/jitter math, both `Retry-After` forms, and an `httptest.Server`-backed `Client.Query` test covering POST method/body/raw-token auth/User-Agent/Accept headers, required timestamps, distinct 400/422 errors, retry-on-429 and 5xx, no-retry-on-other-4xx, and context cancellation.
- **CLI tests** (`cmd/logfire-cli/main_test.go`): call `run()` directly against an `httptest.Server` fixture (via `LOGFIRE_BASE_URL`), asserting exit codes, stdout passthrough, and stderr error formatting.
- **MCP protocol tests** (`cmd/logfire-mcp/tools_test.go`): use the SDK's in-memory transports to list/call tools and list/read the schema Resource. Assert inferred schema constraints, unknown-field rejection, required `min_timestamp`, correct v2 upstream JSON bodies, `{columns, rows}` compatibility output, malformed-success-response handling, validation errors, oversized-response errors, and upstream failures represented as `IsError: true` rather than protocol errors.
- **Python harness** (`tools/test_mcp.py` + `tools/fixture_server.py`, per guide §6): keep `pydantic_ai` and launch the real MCP binary over stdio. The script includes PEP 723 dependency metadata so `uv run tools/test_mcp.py` works from a clean checkout. Its deterministic tier lists capabilities and directly calls `query_run`/`get_schema_metadata` against the local fixture server with `LOGFIRE_BASE_URL` and a dummy token. The agent tier runs only through an explicit `--agent` mode/`make test-agent`, so merely having an LLM key does not make deterministic CI nondeterministic.
- **Makefile targets**: `build` (both binaries via `-ldflags` git-commit version injection), `test`, `test-e2e` (deterministic `uv run tools/test_mcp.py`), `test-agent` (explicit pydantic_ai agent tier), `fmt`, `vet`, `install`, `clean`.

## Deferred to a Later Phase

- **Arrow-go table output** (`--format table` on the CLI): request `Accept: application/vnd.apache.arrow.stream` from Logfire, decode with `github.com/apache/arrow-go/v18/arrow/ipc`, render typed columns (correct timestamp formatting, no float64 precision loss on large IDs) via `text/tabwriter`. Not part of this build — v1 CLI supports `--format json|csv` only, passthrough with no decoding.
- **Streaming NDJSON**: v2 supports `Accept: application/x-ndjson` with `schema`, `data`, `error`, and `end` messages. Defer it until there is a concrete need for streaming large results; v1 keeps bounded JSON/CSV passthrough.
- **Local DataFusion for offline/cached analysis**: re-slicing or joining already-fetched results locally without a round trip to Logfire. Only worth revisiting if repeated-query patterns emerge; Go bindings for DataFusion are CGO-wrapped Rust and heavier than Arrow-go, so this is lower priority than the table-output item above.

## Verification (End-to-End)

1. `go build ./... && go vet ./... && go test ./...`
2. CLI smoke test against the real API (requires `LOGFIRE_API_TOKEN` containing a project read token):
   `go run ./cmd/logfire-cli query --sql "SELECT start_timestamp, message FROM records LIMIT 5" --min-timestamp <RFC3339>` and again with `--format csv`; compare against the current official v2 examples rather than the legacy v1 examples in `refs/client-usage.md`.
3. CLI error paths: missing `--sql` or `--min-timestamp` (exit 2); missing token (exit 1, config error, no HTTP call); `--help` without a token (exit 0); malformed timestamp (exit 1, validation error, no HTTP call); and `max_timestamp < min_timestamp` rejection. Also verify equal min/max timestamps are accepted and return an empty result.
4. MCP server manual run: `LOGFIRE_MCP_LOGFILE=off LOGFIRE_API_TOKEN=... go run ./cmd/logfire-mcp` — locking is off by default; confirm zero stdout bytes before a client connects and clean shutdown on Ctrl+C.
5. `make test-e2e` — deterministic offline pydantic_ai subprocess harness against the fixture server; no LLM key or real Logfire token required. Run `make test-agent` separately when an LLM key is available.
6. Concurrency and optional locking: with the default lock setting, launch two independent MCP instances and confirm both operate. Then set the same explicit `LOGFIRE_MCP_LOCKFILE` path for both, confirm the second fails fast with a clear PID message, and confirm cleanup after the first exits.
