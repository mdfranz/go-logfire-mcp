package logfire_test

import (
	"testing"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
)

func TestAPIError(t *testing.T) {
	err429 := &logfire.APIError{StatusCode: 429, Message: "Rate limited"}
	if !err429.Retryable() {
		t.Errorf("expected 429 to be retryable")
	}

	err500 := &logfire.APIError{StatusCode: 500, Message: "Internal Server Error"}
	if !err500.Retryable() {
		t.Errorf("expected 500 to be retryable")
	}

	err400 := &logfire.APIError{StatusCode: 400, Message: "Bad Request"}
	if err400.Retryable() {
		t.Errorf("expected 400 not to be retryable")
	}

	if err429.Error() != "logfire API error (status 429): Rate limited" {
		t.Errorf("unexpected error string: %s", err429.Error())
	}
}
