package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCLIRunSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"schema":{},"data":[{"msg":"ok"}]}`))
	}))
	defer server.Close()

	os.Setenv("LOGFIRE_BASE_URL", server.URL)
	os.Setenv("LOGFIRE_API_TOKEN", "dummy-token")
	defer func() {
		os.Unsetenv("LOGFIRE_BASE_URL")
		os.Unsetenv("LOGFIRE_API_TOKEN")
	}()

	var stdout, stderr bytes.Buffer
	args := []string{"query", "--sql", "SELECT 1", "--min-timestamp", "2026-01-01T00:00:00Z"}

	code := run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. Stderr: %s", code, stderr.String())
	}

	expected := `{"schema":{},"data":[{"msg":"ok"}]}`
	if stdout.String() != expected {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

func TestCLIRunHelpNoTokenRequired(t *testing.T) {
	os.Unsetenv("LOGFIRE_API_TOKEN")
	os.Unsetenv("LOGFIRE_READ_TOKEN")

	var stdout, stderr bytes.Buffer
	args := []string{"--help"}

	code := run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for --help, got %d", code)
	}
	if !strings.Contains(stdout.String(), "logfire-cli — Command line tool") {
		t.Errorf("help text missing from stdout")
	}
}

func TestCLIRunMissingRequiredFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"query"}

	code := run(args, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing required flags, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--sql is required") {
		t.Errorf("expected error message for missing --sql in stderr")
	}
}
