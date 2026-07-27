package vibeproxy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AcquireLease requests a proxy lease from a pool.
// For sticky allocation, set input.Sticky = true and use a stable ConsumerID.
func (c *Client) AcquireLease(ctx context.Context, input AcquireLeaseInput) (*LeaseResponse, error) {
	if err := input.validate(); err != nil {
		return nil, fmt.Errorf("acquire lease: %w", err)
	}
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/leases", input)
	if err != nil {
		return nil, fmt.Errorf("acquire lease: %w", err)
	}
	return decodeResponse[LeaseResponse](resp)
}

// validate rejects the one request vibe-proxy is CERTAIN to answer with a 400, so
// a caller bug costs one error return instead of an unbounded stream of doomed
// HTTP round-trips.
//
// It mirrors the server's own precondition and nothing more. A zero PoolID is
// deliberately NOT checked here: the server resolves the pool and answers 404 /
// no_matching_proxies, and callers read that classification off the response —
// short-circuiting it locally would swallow a rejection reason the caller acts on.
// Anything the server might accept, or reject with a reason, stays the server's call.
func (i AcquireLeaseInput) validate() error {
	if strings.TrimSpace(i.ConsumerID) == "" {
		return fmt.Errorf("%w: consumerId is required", ErrInvalidInput)
	}
	return nil
}

// GetLease retrieves a lease by ID with connection details.
func (c *Client) GetLease(ctx context.Context, leaseID uuid.UUID) (*LeaseResponse, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/leases/"+leaseID.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("get lease: %w", err)
	}
	return decodeResponse[LeaseResponse](resp)
}

// RenewLease extends the TTL of an active lease.
func (c *Client) RenewLease(ctx context.Context, leaseID uuid.UUID, input RenewLeaseInput) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/leases/"+leaseID.String()+"/renew", input)
	if err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}
	return nil
}

// ReleaseLease releases a lease back to the pool.
func (c *Client) ReleaseLease(ctx context.Context, leaseID uuid.UUID) error {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/leases/"+leaseID.String()+"/release", nil)
	if err != nil {
		return fmt.Errorf("release lease: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}
	return nil
}
