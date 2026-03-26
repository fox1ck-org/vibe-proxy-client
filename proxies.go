package vibeproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

// ProxyListItem represents a proxy returned from the List API.
type ProxyListItem struct {
	ID           uuid.UUID         `json:"id"`
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	Status       string            `json:"status"`
	RotationType string            `json:"rotationType"`
	CountryCode  *string           `json:"countryCode,omitempty"`
	Region       *string           `json:"region,omitempty"`
	City         *string           `json:"city,omitempty"`
	ASN          *int              `json:"asn,omitempty"`
	ISP          *string           `json:"isp,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Endpoints    []ProxyEndpoint   `json:"endpoints,omitempty"`
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

// FindProxyByIP searches for a proxy whose host matches the given IP address.
// Returns the first match or nil if no proxy is found.
func (c *Client) FindProxyByIP(ctx context.Context, ip string) (*ProxyListItem, error) {
	proxies, err := c.ListProxies(ctx, ip)
	if err != nil {
		return nil, err
	}

	// Exact host match (search is ILIKE, so filter for exact)
	for i, p := range proxies {
		if p.Host == ip {
			return &proxies[i], nil
		}
	}

	// If no exact match, return first result (host may contain the IP as substring)
	if len(proxies) > 0 {
		return &proxies[0], nil
	}

	return nil, nil
}
