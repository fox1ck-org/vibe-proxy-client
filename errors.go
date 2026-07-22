package vibeproxy

import "errors"

var (
	// ErrNotFound is returned when the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrUnauthorized is returned when the API key is missing or invalid.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrPoolFull is returned when a pool has reached its maximum lease capacity.
	ErrPoolFull = errors.New("pool is full, no proxies available")

	// ErrLeaseExpired is returned when trying to renew or release an expired lease.
	ErrLeaseExpired = errors.New("lease has expired")
)

// LeaseRejectionReason extracts the machine-readable reason from a vibe-proxy
// API error (one of the Reason* constants: proxy_disabled / proxy_expired /
// proxy_unhealthy / proxy_not_found / no_matching_proxies), or "" if err is
// nil or not a classified *APIError. Use this instead of sniffing err.Error()
// substrings.
func LeaseRejectionReason(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Reason
	}
	return ""
}
