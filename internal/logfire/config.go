package logfire

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	tokenRegionRegex = regexp.MustCompile(`^pylf_v[0-9]+_([a-z]+)_`)

	RegionEndpoints = map[string]string{
		"us": "https://logfire-us.pydantic.dev",
		"eu": "https://logfire-eu.pydantic.dev",
	}
)

type Config struct {
	APIToken         string
	Region           string
	BaseURL          string
	Timeout          time.Duration
	MaxRetries       int
	MaxResponseBytes int64
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	token := strings.TrimSpace(os.Getenv("LOGFIRE_API_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("LOGFIRE_READ_TOKEN"))
	}
	if token == "" {
		token = strings.TrimSpace(os.Getenv("LOGFIRE_API_KEY"))
	}

	regionEnv := strings.ToLower(strings.TrimSpace(os.Getenv("LOGFIRE_REGION")))
	baseURL := strings.TrimSpace(os.Getenv("LOGFIRE_BASE_URL"))

	if regionEnv != "" && baseURL != "" {
		return nil, fmt.Errorf("LOGFIRE_REGION and LOGFIRE_BASE_URL are mutually exclusive")
	}

	if regionEnv != "" {
		if _, ok := RegionEndpoints[regionEnv]; !ok {
			return nil, fmt.Errorf("invalid LOGFIRE_REGION %q: must be 'us' or 'eu'", regionEnv)
		}
	}

	// Auto-detect region from token if not explicitly provided in environment
	resolvedRegion := regionEnv
	if resolvedRegion == "" && token != "" {
		if match := tokenRegionRegex.FindStringSubmatch(token); len(match) > 1 {
			tokenRegion := match[1]
			if _, ok := RegionEndpoints[tokenRegion]; ok {
				resolvedRegion = tokenRegion
			}
		}
	}

	if resolvedRegion == "" {
		resolvedRegion = "us"
	}

	resolvedBaseURL := baseURL
	if resolvedBaseURL == "" {
		resolvedBaseURL = RegionEndpoints[resolvedRegion]
	} else {
		u, err := url.Parse(resolvedBaseURL)
		if err != nil || !u.IsAbs() || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("LOGFIRE_BASE_URL must be a valid absolute URL without userinfo, query, or fragment")
		}
		if u.Scheme != "https" {
			host := u.Hostname()
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				return nil, fmt.Errorf("plain HTTP LOGFIRE_BASE_URL is only permitted for loopback hosts (localhost, 127.0.0.1, ::1)")
			}
		}
	}

	return &Config{
		APIToken:         token,
		Region:           resolvedRegion,
		BaseURL:          resolvedBaseURL,
		Timeout:          30 * time.Second,
		MaxRetries:       3,
		MaxResponseBytes: 16 * 1024 * 1024, // 16 MiB default
	}, nil
}

// ValidateForQuery ensures that a valid auth token is present for making API calls.
func (c *Config) ValidateForQuery() error {
	if c.APIToken == "" {
		return fmt.Errorf("missing Logfire read token: set LOGFIRE_API_TOKEN or LOGFIRE_READ_TOKEN")
	}
	return nil
}

// BearerToken returns the formatted Authorization header value.
func (c *Config) BearerToken() string {
	if strings.HasPrefix(c.APIToken, "Bearer ") {
		return c.APIToken
	}
	return "Bearer " + c.APIToken
}
