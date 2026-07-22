package vibeproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// ProxyListItem represents a proxy returned from the List / find-by-ip APIs.
type ProxyListItem struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Host         string    `json:"host"`
	Status       string    `json:"status"`
	RotationType string    `json:"rotationType"`
	// ConnectionType is the declared network class ("mobile", "residential",
	// "datacenter", "isp", or "unknown" while undeclared).
	ConnectionType string            `json:"connectionType,omitempty"`
	CountryCode    *string           `json:"countryCode,omitempty"`
	Region         *string           `json:"region,omitempty"`
	City           *string           `json:"city,omitempty"`
	ASN            *int              `json:"asn,omitempty"`
	ISP            *string           `json:"isp,omitempty"`
	ExternalIP     *string           `json:"externalIp,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Endpoints      []ProxyEndpoint   `json:"endpoints,omitempty"`
}

// ProxyEndpoint represents a proxy endpoint (protocol + port).
type ProxyEndpoint struct {
	ID        uuid.UUID `json:"id"`
	ProxyID   uuid.UUID `json:"proxyId"`
	Protocol  string    `json:"protocol"`
	Port      int       `json:"port"`
	IsDefault bool      `json:"isDefault"`
}

// ProxyListResponse is the response from listing proxies.
type ProxyListResponse struct {
	Items      []ProxyListItem `json:"items"`
	TotalCount int             `json:"totalCount"`
}

// Rotation types accepted by CreateProxyInput.RotationType.
const (
	RotationStatic         = "static"
	RotationRotating       = "rotating"
	RotationStickyRotating = "sticky_rotating"
)

// Credential auth methods accepted by CreateCredentialInput.AuthMethod.
const (
	AuthMethodUserPass    = "userpass"
	AuthMethodIPWhitelist = "ip_whitelist"
	AuthMethodToken       = "token"
	AuthMethodNone        = "none"
)

// CreateEndpointInput is a proxy endpoint (protocol + port) to seed on creation.
type CreateEndpointInput struct {
	Protocol  Protocol `json:"protocol"`
	Port      int      `json:"port"`
	IsDefault bool     `json:"isDefault"`
}

// CreateCredentialInput is a proxy credential to seed on creation.
type CreateCredentialInput struct {
	AuthMethod string     `json:"authMethod"`
	Username   string     `json:"username,omitempty"`
	Password   string     `json:"password,omitempty"`
	AllowedIPs []string   `json:"allowedIps,omitempty"`
	Token      string     `json:"token,omitempty"`
	IsPrimary  bool       `json:"isPrimary"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

// CreateProxyInput is the request body for registering a proxy, with its
// endpoints, credentials, and classification labels in a single call. Mirrors
// vibe-proxy's domain.CreateProxyInput.
type CreateProxyInput struct {
	Name         string `json:"name"`
	Host         string `json:"host"`
	RotationType string `json:"rotationType"`
	// ConnectionType declares the network class when the caller knows it
	// (e.g. vibe-accounts registering a bundled mobile proxy). Omitted →
	// 'unknown', later self-healed from intelligence server-side.
	ConnectionType *ConnectionType         `json:"connectionType,omitempty"`
	CountryCode    *string                 `json:"countryCode,omitempty"`
	Region         *string                 `json:"region,omitempty"`
	City           *string                 `json:"city,omitempty"`
	ExternalID     *string                 `json:"externalId,omitempty"`
	ExpiresAt      *time.Time              `json:"expiresAt,omitempty"`
	Endpoints      []CreateEndpointInput   `json:"endpoints"`
	Credentials    []CreateCredentialInput `json:"credentials"`
	Labels         map[string]string       `json:"labels,omitempty"`
}

// CreateProxy registers a new proxy (with its endpoints, credentials, and
// labels) via POST /api/v1/proxies and returns the created proxy. Callers that
// bundle a dedicated proxy with an account (e.g. vibe-accounts agency imports)
// use this to hand ownership of the proxy's lifecycle to vibe-proxy, keeping
// credentials out of their own store.
func (c *Client) CreateProxy(ctx context.Context, input CreateProxyInput) (*ProxyListItem, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/proxies", input)
	if err != nil {
		return nil, fmt.Errorf("create proxy: %w", err)
	}
	return decodeResponse[ProxyListItem](resp)
}

// ListProxies retrieves proxies, optionally filtered by search term.
// The search parameter matches against proxy name and host (IP address).
func (c *Client) ListProxies(ctx context.Context, search string) ([]ProxyListItem, error) {
	path := "/api/v1/proxies"
	if search != "" {
		path += "?search=" + url.QueryEscape(search)
	}

	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("list proxies: %w", err)
	}
	result, err := decodeResponse[ProxyListResponse](resp)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// FindProxyByObservedIP resolves an IP to the proxy whose exit gateway that
// IP most recently belonged to. This is the single, canonical lookup used
// by callers (e.g. vibe-fb's extension-driven account registration) to
// match a browser-observed public IP back to a managed proxy.
//
// Under the hood the server checks the proxy's current external_ip column
// first, then falls back to a time-windowed scan of proxy_ip_observation
// so that brief rotation races between sampling and lookup don't break
// the match. `within` is how far back the server is allowed to search;
// it is clamped server-side to a maximum bound (default 1h).
//
// Returns (nil, nil) if no proxy matched within the window.
func (c *Client) FindProxyByObservedIP(ctx context.Context, ip string, within time.Duration) (*ProxyListItem, error) {
	if ip == "" {
		return nil, fmt.Errorf("ip is required")
	}
	if within <= 0 {
		return nil, fmt.Errorf("within must be positive")
	}

	path := "/api/v1/proxies/find-by-ip?ip=" + url.QueryEscape(ip) + "&within=" + url.QueryEscape(within.String())
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("find proxy by observed ip: %w", err)
	}

	result, err := decodeResponse[ProxyListItem](resp)
	if err != nil {
		// 404 is the "no proxy found" signal — surface as (nil, nil) so
		// callers can distinguish it from a genuine transport error.
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}
