package vibeproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// ProxyRequest is a demand-side ask routed to an operator for fulfillment.
type ProxyRequest struct {
	ID               uuid.UUID  `json:"id"`
	RequesterID      string     `json:"requesterId"`
	RequesterEmail   string     `json:"requesterEmail"`
	TargetApp        string     `json:"targetApp"`
	Countries        []string   `json:"countries"`
	Quantity         int        `json:"quantity"`
	VendorPreference *string    `json:"vendorPreference,omitempty"`
	Notes            string     `json:"notes"`
	Status           string     `json:"status"`
	ClaimedBy        *string    `json:"claimedBy,omitempty"`
	TargetPoolID     *uuid.UUID `json:"targetPoolId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// CreateRequestInput is the payload for opening a proxy request.
type CreateRequestInput struct {
	TargetApp        string   `json:"targetApp"`
	Countries        []string `json:"countries"`
	Quantity         int      `json:"quantity"`
	VendorPreference *string  `json:"vendorPreference,omitempty"`
	Notes            string   `json:"notes"`
}

// CreateRequest opens a new proxy request. Requires an authenticated identity
// (JWT or API key); the caller becomes the requester.
func (c *Client) CreateRequest(ctx context.Context, input CreateRequestInput) (*ProxyRequest, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/requests", input)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return decodeResponse[ProxyRequest](resp)
}

// GetRequest retrieves a request by ID.
func (c *Client) GetRequest(ctx context.Context, id uuid.UUID) (*ProxyRequest, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/requests/"+id.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return decodeResponse[ProxyRequest](resp)
}

// ListRequests returns requests visible to the caller. A non-empty status
// filters by lifecycle state.
func (c *Client) ListRequests(ctx context.Context, status string) ([]ProxyRequest, error) {
	path := "/api/v1/requests"
	if status != "" {
		path += "?" + url.Values{"status": {status}}.Encode()
	}
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	out, err := decodeResponse[[]ProxyRequest](resp)
	if err != nil {
		return nil, err
	}
	return *out, nil
}
