package logfire

import "fmt"

// APIError represents an HTTP error response from the Logfire API.
type APIError struct {
	StatusCode   int
	Message      string
	ResponseBody string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("logfire API error (status %d): %s", e.StatusCode, e.Message)
	}
	if e.ResponseBody != "" {
		return fmt.Sprintf("logfire API error (status %d): %s", e.StatusCode, e.ResponseBody)
	}
	return fmt.Sprintf("logfire API error (status %d)", e.StatusCode)
}

// Retryable returns true if the status code represents a transient error (429 or 5xx).
func (e *APIError) Retryable() bool {
	if e.StatusCode == 429 {
		return true
	}
	return e.StatusCode >= 500 && e.StatusCode <= 599
}
