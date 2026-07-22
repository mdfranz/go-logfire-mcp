package logfire_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
)

func TestClientQuerySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected accept header: %s", r.Header.Get("Accept"))
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if body["sql"] != "SELECT 1" {
			t.Errorf("unexpected sql: %v", body["sql"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"schema":{"fields":[]},"data":[{"val":1}]}`))
	}))
	defer server.Close()

	cfg := &logfire.Config{
		APIToken:         "test-token",
		BaseURL:          server.URL,
		Timeout:          5 * time.Second,
		MaxRetries:       1,
		MaxResponseBytes: 1024 * 1024,
	}
	client := logfire.NewClient(cfg)

	input := logfire.QueryInput{
		SQL:          "SELECT 1",
		MinTimestamp: "2026-01-01T00:00:00Z",
	}

	resp, err := client.Query(context.Background(), input, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"schema":{"fields":[]},"data":[{"val":1}]}`
	if resp != expected {
		t.Errorf("unexpected response: %s", resp)
	}
}

func TestClientQueryRetryOn429(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message": "rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"schema":{},"data":[]}`))
	}))
	defer server.Close()

	cfg := &logfire.Config{
		APIToken:         "test-token",
		BaseURL:          server.URL,
		Timeout:          5 * time.Second,
		MaxRetries:       2,
		MaxResponseBytes: 1024 * 1024,
	}
	client := logfire.NewClient(cfg)

	input := logfire.QueryInput{
		SQL:          "SELECT 1",
		MinTimestamp: "2026-01-01T00:00:00Z",
	}

	resp, err := client.Query(context.Background(), input, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != `{"schema":{},"data":[]}` {
		t.Errorf("unexpected response: %s", resp)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClientQueryNoRetryOn400(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message": "syntax error"}`))
	}))
	defer server.Close()

	cfg := &logfire.Config{
		APIToken:         "test-token",
		BaseURL:          server.URL,
		Timeout:          5 * time.Second,
		MaxRetries:       2,
		MaxResponseBytes: 1024 * 1024,
	}
	client := logfire.NewClient(cfg)

	input := logfire.QueryInput{
		SQL:          "INVALID SQL",
		MinTimestamp: "2026-01-01T00:00:00Z",
	}

	_, err := client.Query(context.Background(), input, "json")
	if err == nil {
		t.Fatal("expected error for 400 Bad Request")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt for 400, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClientQueryOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("12345678901234567890")) // 20 bytes
	}))
	defer server.Close()

	cfg := &logfire.Config{
		APIToken:         "test-token",
		BaseURL:          server.URL,
		Timeout:          5 * time.Second,
		MaxRetries:       0,
		MaxResponseBytes: 10, // Limit to 10 bytes
	}
	client := logfire.NewClient(cfg)

	input := logfire.QueryInput{
		SQL:          "SELECT 1",
		MinTimestamp: "2026-01-01T00:00:00Z",
	}

	_, err := client.Query(context.Background(), input, "json")
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
}
