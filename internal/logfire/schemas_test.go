package logfire_test

import (
	"strings"
	"testing"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
)

func TestQueryInputValidate(t *testing.T) {
	valid := &logfire.QueryInput{
		SQL:          "SELECT * FROM records",
		MinTimestamp: "2026-01-01T00:00:00Z",
		MaxTimestamp: "2026-01-02T00:00:00Z",
		Limit:        100,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	t.Run("Empty SQL", func(t *testing.T) {
		q := *valid
		q.SQL = "   "
		if err := q.Validate(); err == nil {
			t.Errorf("expected error for empty SQL")
		}
	})

	t.Run("Oversized SQL", func(t *testing.T) {
		q := *valid
		q.SQL = strings.Repeat("A", 65*1024)
		if err := q.Validate(); err == nil {
			t.Errorf("expected error for oversized SQL")
		}
	})

	t.Run("Missing min_timestamp", func(t *testing.T) {
		q := *valid
		q.MinTimestamp = ""
		if err := q.Validate(); err == nil {
			t.Errorf("expected error for missing min_timestamp")
		}
	})

	t.Run("Invalid RFC3339 timestamp", func(t *testing.T) {
		q := *valid
		q.MinTimestamp = "2026-01-01"
		if err := q.Validate(); err == nil {
			t.Errorf("expected error for invalid RFC3339 timestamp")
		}
	})

	t.Run("max_timestamp before min_timestamp", func(t *testing.T) {
		q := *valid
		q.MinTimestamp = "2026-01-02T00:00:00Z"
		q.MaxTimestamp = "2026-01-01T00:00:00Z"
		if err := q.Validate(); err == nil {
			t.Errorf("expected error when max_timestamp is before min_timestamp")
		}
	})

	t.Run("Limit out of bounds", func(t *testing.T) {
		q := *valid
		q.Limit = 20000
		if err := q.Validate(); err == nil {
			t.Errorf("expected error for limit > 10000")
		}
	})
}

func TestAcceptHeaderFor(t *testing.T) {
	h, err := logfire.AcceptHeaderFor("json")
	if err != nil || h != "application/json" {
		t.Errorf("expected application/json, got %s, err %v", h, err)
	}

	h, err = logfire.AcceptHeaderFor("csv")
	if err != nil || h != "text/csv" {
		t.Errorf("expected text/csv, got %s, err %v", h, err)
	}

	_, err = logfire.AcceptHeaderFor("xml")
	if err == nil {
		t.Errorf("expected error for unsupported format xml")
	}
}
