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
// proxy_unhealthy / proxy_not_found / no_matching_proxies /
// no_healthy_proxies / network_lookup_unavailable), or "" if err is nil or not
// a classified *APIError. Use this instead of sniffing err.Error() substrings —
// the message is prose and has already changed once; the reason is the
// contract.
func LeaseRejectionReason(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Reason
	}
	return ""
}

// LeaseRejectionNeedsOperator reports whether a pinned lease was refused for a
// reason no retry can fix: the proxy is expired, disabled, or gone. Those need
// a human (renew, re-enable, re-bind). Everything else — unhealthy, pool full,
// no healthy proxies, a network lookup that failed, a transport error — is
// transient and false here.
//
// A true answer is never a licence to hand a sticky consumer a different
// proxy: the proxy is bound to a profile, and changing the exit IP without a
// reason is exactly what anti-fraud looks for.
func LeaseRejectionNeedsOperator(err error) bool {
	switch LeaseRejectionReason(err) {
	case ReasonProxyExpired, ReasonProxyDisabled, ReasonProxyNotFound:
		return true
	default:
		return false
	}
}

// RenewRejectionReason extracts the machine-readable reason from a failed
// RenewProxy call (ReasonRenewRefused / ReasonRenewUnsupported), or "" when the
// failure was something else — a transport error, a 502 from an unreachable
// provider, a 404. It is the same mechanism as LeaseRejectionReason, named
// separately because the two vocabularies do not overlap and a caller reaching
// for one should never be handed the other's codes.
func RenewRejectionReason(err error) string {
	switch reason := LeaseRejectionReason(err); reason {
	case ReasonRenewRefused, ReasonRenewUnsupported:
		return reason
	default:
		return ""
	}
}
