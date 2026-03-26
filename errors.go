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
