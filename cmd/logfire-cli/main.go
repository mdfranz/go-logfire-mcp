package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printGeneralHelp(stderr)
		return 2
	}

	subcommand := args[0]
	if subcommand == "-h" || subcommand == "--help" || subcommand == "help" {
		printGeneralHelp(stdout)
		return 0
	}

	if subcommand != "query" {
		fmt.Fprintf(stderr, "Unknown command %q. Available commands: query\n", subcommand)
		printGeneralHelp(stderr)
		return 2
	}

	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		sql          string
		minTimestamp string
		maxTimestamp string
		limit        int
		format       string
	)

	fs.StringVar(&sql, "sql", "", "SQL query to execute (required)")
	fs.StringVar(&minTimestamp, "min-timestamp", "", "Minimum RFC3339 timestamp filter (required)")
	fs.StringVar(&maxTimestamp, "max-timestamp", "", "Maximum RFC3339 timestamp filter (optional)")
	fs.IntVar(&limit, "limit", 0, "Maximum number of rows to return (1-10000, default 100)")
	fs.StringVar(&format, "format", "json", "Output format: json or csv (default json)")

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	logger, closeLog, err := configureLogger()
	if err != nil {
		writeError(stderr, fmt.Errorf("logging configuration error: %w", err))
		return 1
	}
	defer closeLog()

	if sql == "" {
		logger.Error("missing required query flag", "flag", "sql")
		fmt.Fprintln(stderr, "Error: --sql is required")
		fs.Usage()
		return 2
	}

	if minTimestamp == "" {
		logger.Error("missing required query flag", "flag", "min-timestamp")
		fmt.Fprintln(stderr, "Error: --min-timestamp is required")
		fs.Usage()
		return 2
	}

	cfg, err := logfire.LoadConfig()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		writeError(stderr, fmt.Errorf("configuration error: %w", err))
		return 1
	}

	if err := cfg.ValidateForQuery(); err != nil {
		logger.Error("query configuration is invalid", "error", err)
		writeError(stderr, err)
		return 1
	}

	client := logfire.NewClient(cfg)

	input := logfire.QueryInput{
		SQL:          sql,
		MinTimestamp: minTimestamp,
		MaxTimestamp: maxTimestamp,
		Limit:        limit,
	}
	logger.Debug("executing query", "sql", sql, "format", format, "sql_bytes", len(sql), "has_max_timestamp", maxTimestamp != "", "limit", limit)

	startTime := time.Now()
	result, err := client.Query(context.Background(), input, format)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("query failed", "error", err, "duration_ms", duration.Milliseconds())
		writeError(stderr, err)
		return 1
	}

	if err := writeResult(stdout, result); err != nil {
		logger.Error("failed to write query result", "error", err, "duration_ms", duration.Milliseconds())
		writeError(stderr, fmt.Errorf("failed to write stdout: %w", err))
		return 1
	}

	logger.Debug("query completed", "duration_ms", duration.Milliseconds(), "records", logfire.CountResultRows(result, format), "result_bytes", len(result))

	return 0
}

func printGeneralHelp(w io.Writer) {
	helpText := `logfire-cli — Command line tool for querying Logfire telemetry data

Usage:
  logfire-cli query --sql "<SQL>" --min-timestamp "<RFC3339>" [flags]

Flags for 'query':
  --sql string            SQL query to execute (required)
  --min-timestamp string  Minimum RFC3339 timestamp (required, e.g. 2026-01-01T00:00:00Z)
  --max-timestamp string  Maximum RFC3339 timestamp (optional)
  --limit int             Row limit (1-10000)
  --format string         Output format: json or csv (default "json")

Environment Variables:
  LOGFIRE_API_TOKEN   Logfire project read token (required for queries)
  LOGFIRE_READ_TOKEN  Alternative token env var fallback
  LOGFIRE_REGION      Logfire region ("us" or "eu", default "us")
  LOGFIRE_BASE_URL    Advanced/test override for base URL
  LOGFIRE_MAX_RETRIES Max retries for API requests (default 3)
`
	fmt.Fprint(w, helpText)
}
