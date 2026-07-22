package logfire_test

import (
	"os"
	"testing"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
)

func TestConfigLoading(t *testing.T) {
	// Save existing env
	oldToken := os.Getenv("LOGFIRE_API_TOKEN")
	oldReadToken := os.Getenv("LOGFIRE_READ_TOKEN")
	oldRegion := os.Getenv("LOGFIRE_REGION")
	oldBaseURL := os.Getenv("LOGFIRE_BASE_URL")

	defer func() {
		os.Setenv("LOGFIRE_API_TOKEN", oldToken)
		os.Setenv("LOGFIRE_READ_TOKEN", oldReadToken)
		os.Setenv("LOGFIRE_REGION", oldRegion)
		os.Setenv("LOGFIRE_BASE_URL", oldBaseURL)
	}()

	t.Run("Default region US", func(t *testing.T) {
		os.Unsetenv("LOGFIRE_API_TOKEN")
		os.Unsetenv("LOGFIRE_READ_TOKEN")
		os.Unsetenv("LOGFIRE_REGION")
		os.Unsetenv("LOGFIRE_BASE_URL")

		cfg, err := logfire.LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Region != "us" {
			t.Errorf("expected default region us, got %s", cfg.Region)
		}
		if cfg.BaseURL != "https://logfire-us.pydantic.dev" {
			t.Errorf("unexpected base URL: %s", cfg.BaseURL)
		}
	})

	t.Run("Auto-detect EU token region", func(t *testing.T) {
		os.Setenv("LOGFIRE_API_TOKEN", "pylf_v1_eu_123456789")
		os.Unsetenv("LOGFIRE_READ_TOKEN")
		os.Unsetenv("LOGFIRE_REGION")
		os.Unsetenv("LOGFIRE_BASE_URL")

		cfg, err := logfire.LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Region != "eu" {
			t.Errorf("expected auto-detected region eu, got %s", cfg.Region)
		}
		if cfg.BaseURL != "https://logfire-eu.pydantic.dev" {
			t.Errorf("unexpected base URL: %s", cfg.BaseURL)
		}
	})

	t.Run("LOGFIRE_READ_TOKEN fallback", func(t *testing.T) {
		os.Unsetenv("LOGFIRE_API_TOKEN")
		os.Setenv("LOGFIRE_READ_TOKEN", "my-read-token")
		os.Unsetenv("LOGFIRE_REGION")
		os.Unsetenv("LOGFIRE_BASE_URL")

		cfg, err := logfire.LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.APIToken != "my-read-token" {
			t.Errorf("expected my-read-token, got %s", cfg.APIToken)
		}
	})

	t.Run("Mutually exclusive region and base URL", func(t *testing.T) {
		os.Setenv("LOGFIRE_REGION", "eu")
		os.Setenv("LOGFIRE_BASE_URL", "http://localhost:8080")

		_, err := logfire.LoadConfig()
		if err == nil {
			t.Errorf("expected error when setting both region and base URL")
		}
	})

	t.Run("Base URL loopback http allowed", func(t *testing.T) {
		os.Unsetenv("LOGFIRE_REGION")
		os.Setenv("LOGFIRE_BASE_URL", "http://127.0.0.1:8080")

		cfg, err := logfire.LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BaseURL != "http://127.0.0.1:8080" {
			t.Errorf("unexpected base URL: %s", cfg.BaseURL)
		}
	})

	t.Run("Base URL non-loopback http rejected", func(t *testing.T) {
		os.Unsetenv("LOGFIRE_REGION")
		os.Setenv("LOGFIRE_BASE_URL", "http://example.com:8080")

		_, err := logfire.LoadConfig()
		if err == nil {
			t.Errorf("expected error for non-loopback http base URL")
		}
	})

	t.Run("LOGFIRE_MAX_RETRIES custom setting", func(t *testing.T) {
		os.Unsetenv("LOGFIRE_REGION")
		os.Unsetenv("LOGFIRE_BASE_URL")
		os.Setenv("LOGFIRE_MAX_RETRIES", "5")
		defer os.Unsetenv("LOGFIRE_MAX_RETRIES")

		cfg, err := logfire.LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MaxRetries != 5 {
			t.Errorf("expected MaxRetries=5, got %d", cfg.MaxRetries)
		}
	})
}
