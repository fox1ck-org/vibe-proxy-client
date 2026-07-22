package vibeproxy

import (
	"time"

	"github.com/google/uuid"
)

// LeaseStatus represents the state of a proxy lease.
type LeaseStatus string

const (
	LeaseStatusActive   LeaseStatus = "active"
	LeaseStatusExpired  LeaseStatus = "expired"
	LeaseStatusReleased LeaseStatus = "released"
	LeaseStatusRevoked  LeaseStatus = "revoked"
)

// Protocol represents a proxy protocol type.
type Protocol string

const (
	ProtocolHTTP   Protocol = "http"
	ProtocolHTTPS  Protocol = "https"
	ProtocolSOCKS4 Protocol = "socks4"
	ProtocolSOCKS5 Protocol = "socks5"
)

// ConnectionType is the declared network class of a proxy — a first-class
// allocation axis (distinct from the intelligence-observed ipType). These are
// the requestable values; proxies with an undeclared ('unknown') class never
// match a ConnectionType filter.
type ConnectionType string

const (
	ConnectionMobile      ConnectionType = "mobile"
	ConnectionResidential ConnectionType = "residential"
	ConnectionDatacenter  ConnectionType = "datacenter"
	ConnectionISP         ConnectionType = "isp"
)

// Lease represents an active or historical proxy lease.
type Lease struct {
	ID           uuid.UUID         `json:"id"`
	PoolID       uuid.UUID         `json:"poolId"`
	ProxyID      uuid.UUID         `json:"proxyId"`
	EndpointID   uuid.UUID         `json:"endpointId"`
	ConsumerID   string            `json:"consumerId"`
	ConsumerMeta map[string]string `json:"consumerMeta,omitempty"`
	Status       LeaseStatus       `json:"status"`
	ExpiresAt    time.Time         `json:"expiresAt"`
	RenewedAt    *time.Time        `json:"renewedAt,omitempty"`
	RenewCount   int               `json:"renewCount"`
	ReleasedAt   *time.Time        `json:"releasedAt,omitempty"`
	StickyKey    *string           `json:"stickyKey,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
}

// AcquireLeaseInput is the request body for acquiring a lease.
//
// Filter axes (Protocol, ConnectionType, Country, City) narrow an unpinned
// lease's candidate set; ALL set axes must match. Zero matches → 409 with
// reason ReasonNoMatchingProxies — never a silent wrong-class allocation.
// A PreferredProxyID pin bypasses candidate filtering (the pin wins), except
// Protocol, which still gates endpoint selection on the pinned proxy.
// An idempotent re-acquire returns the consumer's existing active lease
// WITHOUT re-checking filters; release-then-acquire to change them.
type AcquireLeaseInput struct {
	PoolID           uuid.UUID         `json:"poolId"`
	ConsumerID       string            `json:"consumerId"`
	ConsumerMeta     map[string]string `json:"consumerMeta,omitempty"`
	PreferredProxyID *uuid.UUID        `json:"preferredProxyId,omitempty"`
	TTLSeconds       *int              `json:"ttlSeconds,omitempty"`
	Sticky           bool              `json:"sticky"`

	// Protocol is a HARD filter: the lease lands on an endpoint of exactly
	// this protocol, and only proxies exposing one are eligible. There is no
	// soft preference — ask for socks5, get socks5 or a clean rejection.
	Protocol *Protocol `json:"protocol,omitempty"`

	// ConnectionType narrows to the declared network class (mobile /
	// residential / datacenter / isp). How a consumer that needs strictly
	// mobile proxies (e.g. spy) expresses it.
	ConnectionType *ConnectionType `json:"connectionType,omitempty"`

	// Country / City narrow by geo (case-insensitive; empty means "any").
	// This is the first-class way to ask a pool for "a UA proxy" without
	// knowing which specific proxy to pin.
	Country *string `json:"country,omitempty"`
	City    *string `json:"city,omitempty"`

	// AllowUnhealthyPreferred lets a pinned PreferredProxyID be leased even
	// when its cluster-side health summary is 'failed', provided it is still
	// status='enabled'. Set it only when you hold live, out-of-band evidence
	// the proxy works (e.g. an observed exit IP); it never overrides a
	// disabled/banned or missing proxy.
	AllowUnhealthyPreferred bool `json:"allowUnhealthyPreferred,omitempty"`
}

// LeaseResponse is the response from acquiring or getting a lease.
type LeaseResponse struct {
	Lease      Lease          `json:"lease"`
	Connection ConnectionInfo `json:"connection"`
}

// ConnectionInfo contains the connection details for a leased proxy.
type ConnectionInfo struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Protocol string  `json:"protocol"`
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
	Token    *string `json:"token,omitempty"`
	ProxyURL string  `json:"proxyUrl"`
}

// RenewLeaseInput is the request body for renewing a lease.
type RenewLeaseInput struct {
	TTLSeconds *int `json:"ttlSeconds,omitempty"`
}

// Pool represents a proxy pool configuration.
type Pool struct {
	ID                   uuid.UUID         `json:"id"`
	Name                 string            `json:"name"`
	Description          *string           `json:"description,omitempty"`
	AllocationStrategy   string            `json:"allocationStrategy"`
	SelectionMode        string            `json:"selectionMode"`
	SelectorLabels       map[string]string `json:"selectorLabels,omitempty"`
	MaxLeases            *int              `json:"maxLeases,omitempty"`
	MaxLeasesPerConsumer *int              `json:"maxLeasesPerConsumer,omitempty"`
	DefaultLeaseTTLSec   int               `json:"defaultLeaseTtlSec"`
	MaxLeaseTTLSec       int               `json:"maxLeaseTtlSec"`
	StickyTTLSec         *int              `json:"stickyTtlSec,omitempty"`
	Status               string            `json:"status"`
	CreatedAt            time.Time         `json:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
	MemberCount          int               `json:"memberCount,omitempty"`
	ActiveLeases         int               `json:"activeLeases,omitempty"`
}

// APIError represents an error response from the vibe-proxy API.
type APIError struct {
	StatusCode int
	Message    string `json:"error"`
	// Reason is a machine-readable cause set by the server for errors that
	// need differentiated handling (e.g. preferred-proxy lease rejections:
	// "proxy_disabled", "proxy_unhealthy", "proxy_not_found"). Empty when the
	// server didn't classify the error.
	Reason string `json:"reason"`
}

func (e *APIError) Error() string {
	return e.Message
}

// Lease-rejection reason codes emitted by vibe-proxy. Match against
// APIError.Reason (via LeaseRejectionReason), never the error string.
const (
	// Pinned PreferredProxyID that can't be leased:
	ReasonProxyDisabled  = "proxy_disabled"
	ReasonProxyExpired   = "proxy_expired"
	ReasonProxyUnhealthy = "proxy_unhealthy"
	ReasonProxyNotFound  = "proxy_not_found"

	// ReasonNoMatchingProxies: the pool has eligible proxies but none match
	// the requested filter axes (or a pinned proxy lacks the requested
	// protocol endpoint). Distinct from a capacity 409 (pool full), which
	// carries no reason. Typical handling: relax filters or open a
	// CreateRequest for proxies of that class.
	ReasonNoMatchingProxies = "no_matching_proxies"
)
