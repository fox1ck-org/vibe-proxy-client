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

	// ErrInvalidInput is returned when a request is rejected locally, before any
	// HTTP call, because the server is guaranteed to refuse it.
	//
	// This exists because a caller with an empty consumerId used to send the
	// request anyway and take a 400 every time: 740k rejected acquires against
	// one deployment in 30 hours (87% of all lease traffic), each a full HTTP
	// round-trip, none of them ever able to succeed. Failing here keeps a caller
	// bug from becoming server load, and gives the caller a typed error instead
	// of an opaque "400 Bad Request".
	ErrInvalidInput = errors.New("invalid input")
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
