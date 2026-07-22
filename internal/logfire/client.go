package logfire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	cfg        *Config
	httpClient *http.Client
}

// NewClient creates a new Logfire API client using the given configuration.
func NewClient(cfg *Config) *Client {
	return NewClientWithHTTP(cfg, &http.Client{
		Timeout: cfg.Timeout,
	})
}

// NewClientWithHTTP creates a client with a custom http.Client.
func NewClientWithHTTP(cfg *Config, httpClient *http.Client) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

// Query executes a SQL query against the Logfire POST /v2/query endpoint.
func (c *Client) Query(ctx context.Context, input QueryInput, format string) (string, error) {
	if err := c.cfg.ValidateForQuery(); err != nil {
		return "", err
	}

	if err := input.Validate(); err != nil {
		return "", err
	}

	acceptHeader, err := AcceptHeaderFor(format)
	if err != nil {
		return "", err
	}

	input.IncludeSchema = true
	bodyBytes, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal query input: %w", err)
	}

	endpointURL := strings.TrimRight(c.cfg.BaseURL, "/") + "/v2/query"

	maxRetries := c.cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastErr error
	backoff := 200 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", c.cfg.BearerToken())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
			lastErr = fmt.Errorf("network error on attempt %d: %w", attempt+1, err)
		} else {
			respBody, err := c.handleResponse(resp)
			resp.Body.Close()

			if err == nil {
				return respBody, nil
			}

			lastErr = err

			apiErr, isAPIErr := err.(*APIError)
			if !isAPIErr || !apiErr.Retryable() {
				return "", err
			}

			// Determine initial or updated backoff
			if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
				if sec, parseErr := strconv.Atoi(strings.TrimSpace(retryAfterStr)); parseErr == nil && sec > 0 {
					backoff = time.Duration(sec) * time.Second
				} else if t, parseErr := http.ParseTime(retryAfterStr); parseErr == nil {
					if sleepDur := time.Until(t); sleepDur > 0 {
						backoff = sleepDur
					}
				}
			} else if apiErr.StatusCode == 429 && backoff < 1*time.Second {
				backoff = 1 * time.Second
			}
		}

		if attempt == maxRetries {
			break
		}

		// Apply jitter to backoff (0.8x to 1.2x)
		jitter := time.Duration(float64(backoff) * (0.8 + rand.Float64()*0.4))
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(jitter):
		}

		backoff *= 2
		maxBackoff := 10 * time.Second
		if lastErr != nil {
			if apiErr, ok := lastErr.(*APIError); ok && apiErr.StatusCode == 429 {
				maxBackoff = 30 * time.Second
			}
		}
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return "", fmt.Errorf("query failed after %d attempts: %w", maxRetries+1, lastErr)
}

func (c *Client) handleResponse(resp *http.Response) (string, error) {
	maxBytes := c.cfg.MaxResponseBytes
	if maxBytes <= 0 {
		maxBytes = 16 * 1024 * 1024
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		lr := &io.LimitedReader{R: resp.Body, N: maxBytes + 1}
		buf, err := io.ReadAll(lr)
		if err != nil {
			return "", fmt.Errorf("error reading response body: %w", err)
		}

		if int64(len(buf)) > maxBytes {
			return "", fmt.Errorf("response body size exceeded maximum limit of %d bytes; try reducing the limit parameter, narrowing the time range, or selecting fewer columns", maxBytes)
		}

		return string(buf), nil
	}

	// Error response body (cap at 64 KiB)
	errLr := &io.LimitedReader{R: resp.Body, N: 64 * 1024}
	errBuf, _ := io.ReadAll(errLr)

	apiErr := &APIError{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(errBuf),
	}

	// Try to extract JSON message if present
	var jsonErr struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(errBuf, &jsonErr) == nil {
		if jsonErr.Message != "" {
			apiErr.Message = jsonErr.Message
		} else if jsonErr.Detail != "" {
			apiErr.Message = jsonErr.Detail
		} else if jsonErr.Error != "" {
			apiErr.Message = jsonErr.Error
		}
	}

	return "", apiErr
}
