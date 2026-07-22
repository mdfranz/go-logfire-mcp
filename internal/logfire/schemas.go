package logfire

import (
	"fmt"
	"strings"
	"time"
)

type QueryInput struct {
	SQL           string `json:"sql"`
	MinTimestamp  string `json:"min_timestamp"`
	MaxTimestamp  string `json:"max_timestamp,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	IncludeSchema bool   `json:"include_schema"`
}

// Validate checks business validation rules on QueryInput.
func (q *QueryInput) Validate() error {
	sqlTrimmed := strings.TrimSpace(q.SQL)
	if sqlTrimmed == "" {
		return fmt.Errorf("sql query cannot be empty")
	}
	if len(q.SQL) > 64*1024 {
		return fmt.Errorf("sql query string exceeds maximum allowed size of 64 KiB")
	}

	if q.MinTimestamp == "" {
		return fmt.Errorf("min_timestamp is required (RFC3339 format, e.g. 2026-01-01T00:00:00Z)")
	}

	minTime, err := time.Parse(time.RFC3339, q.MinTimestamp)
	if err != nil {
		return fmt.Errorf("invalid min_timestamp: must be a valid RFC3339 timestamp (e.g. 2026-01-01T00:00:00Z): %w", err)
	}

	if q.MaxTimestamp != "" {
		maxTime, err := time.Parse(time.RFC3339, q.MaxTimestamp)
		if err != nil {
			return fmt.Errorf("invalid max_timestamp: must be a valid RFC3339 timestamp (e.g. 2026-01-01T00:00:00Z): %w", err)
		}
		if !maxTime.After(minTime) {
			return fmt.Errorf("max_timestamp (%s) must be strictly after min_timestamp (%s)", q.MaxTimestamp, q.MinTimestamp)
		}
	}

	if q.Limit < 0 || q.Limit > 10000 {
		return fmt.Errorf("limit must be between 1 and 10000 (got %d)", q.Limit)
	}

	return nil
}

// AcceptHeaderFor returns the HTTP Accept header value for the requested output format.
func AcceptHeaderFor(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		return "application/json", nil
	case "csv":
		return "text/csv", nil
	default:
		return "", fmt.Errorf("unsupported output format %q: expected 'json' or 'csv'", format)
	}
}
