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
type AcquireLeaseInput struct {
	PoolID            uuid.UUID         `json:"poolId"`
	ConsumerID        string            `json:"consumerId"`
	ConsumerMeta      map[string]string `json:"consumerMeta,omitempty"`
	PreferredProtocol *Protocol         `json:"preferredProtocol,omitempty"`
	TTLSeconds        *int              `json:"ttlSeconds,omitempty"`
	Sticky            bool              `json:"sticky"`
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
}

func (e *APIError) Error() string {
	return e.Message
}
