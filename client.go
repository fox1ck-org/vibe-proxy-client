package vibeproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "http://vibe-proxy.vibe-proxy.svc.cluster.local:8080"
	defaultTimeout = 10 * time.Second
	maxRetries     = 2

	// defaultPurchaseTimeout covers the buy path, which is synchronous inside
	// vibe-proxy: it calls the vendor's API, imports every proxy that came back,
	// and fulfils the request before it answers. The 10s default is a guaranteed
	// cut on a call that spends money — and a cut client-side does NOT cancel
	// the purchase, it only loses the receipt.
	defaultPurchaseTimeout = 120 * time.Second
)

// Client is an HTTP client for the vibe-proxy REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	// slowClient carries the same transport with a longer deadline, for the
	// handful of routes that do real work before answering.
	slowClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		cl.httpClient = c
		// Keep the slow path on the same transport, but never inherit the short
		// deadline — that is the whole reason it is a separate client.
		cl.slowClient = &http.Client{Transport: c.Transport, Timeout: cl.slowClient.Timeout}
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(cl *Client) { cl.httpClient.Timeout = d }
}

// WithPurchaseTimeout sets the deadline for the money-spending routes
// (EstimateOrder, PurchaseForRequest), which are synchronous server-side.
func WithPurchaseTimeout(d time.Duration) Option {
	return func(cl *Client) { cl.slowClient.Timeout = d }
}

// NewClient creates a new vibe-proxy API client.
// baseURL defaults to the in-cluster service URL if empty.
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	c := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		slowClient: &http.Client{
			Timeout: defaultPurchaseTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				// Re-create body reader for retry
				if body != nil {
					data, _ := json.Marshal(body)
					bodyReader = bytes.NewReader(data)
				}
				continue
			}
			return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, err)
		}

		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server returned %d", resp.StatusCode)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				if body != nil {
					data, _ := json.Marshal(body)
					bodyReader = bytes.NewReader(data)
				}
				continue
			}
			return nil, lastErr
		}

		return resp, nil
	}
	return nil, lastErr
}

// doOnce issues a request with NO retries, on the long-deadline client.
//
// Retrying is correct for reads and for idempotent writes; it is a second
// charge on a purchase. vibe-proxy's buy route has no idempotency key, so a
// transport error here means "the outcome is unknown" — the caller must go and
// look (GET the request, list proxies by its request_id label) rather than ask
// again.
func (c *Client) doOnce(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	return c.slowClient.Do(req)
}

func decodeResponse[T any](resp *http.Response) (*T, error) {
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	// When the server classified the error with a machine-readable `reason`,
	// preserve the full structured error (status + reason + message) so callers
	// can branch on the cause. This must run BEFORE the bare-sentinel mapping
	// below, which would otherwise collapse a reason-carrying 404/409 into an
	// opaque ErrNotFound/ErrPoolFull.
	var apiErr APIError
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Reason != "" {
		apiErr.StatusCode = resp.StatusCode
		return &apiErr
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusConflict:
		return ErrPoolFull
	}

	if apiErr.Message != "" {
		apiErr.StatusCode = resp.StatusCode
		return &apiErr
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    fmt.Sprintf("API error %d: %s", resp.StatusCode, string(body)),
	}
}
