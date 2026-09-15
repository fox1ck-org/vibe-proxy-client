package vibeproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	ConnectionType string  `json:"connectionType,omitempty"`
	CountryCode    *string `json:"countryCode,omitempty"`
	Region         *string `json:"region,omitempty"`
	City           *string `json:"city,omitempty"`
	ASN            *int    `json:"asn,omitempty"`
	ISP            *string `json:"isp,omitempty"`
	// ExternalID is what the PROVIDER calls this proxy — the id a renewal, a
	// sync or a destroy is addressed to. Absent for a proxy somebody entered by
	// hand, which is exactly what makes that proxy un-renewable.
	ExternalID *string           `json:"externalId,omitempty"`
	ExternalIP *string           `json:"externalIp,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Endpoints  []ProxyEndpoint   `json:"endpoints,omitempty"`

	// Expired reports that the provider's term has run out. vibe-proxy derives
	// it at read time from expires_at against the DATABASE clock — the same
	// predicate lease selection uses — so it can never disagree with a lease
	// answer. Expiry is deliberately NOT a Status value: Status stays
	// enabled/disabled/banned (operator intent), and an enabled proxy can be
	// expired. On 2026-09-15, 213 proxies read status=enabled while every
	// pinned lease on them answered 409 proxy_expired.
	Expired bool `json:"expired"`

	// Leasability is the verdict POST /leases would give a lease pinned to this
	// proxy right now (vibe-proxy's ClassifyLeasable over status × expiry ×
	// effective health). disabled outranks expired, expired outranks unhealthy.
	Leasability Leasability `json:"leasability,omitempty"`
}

// NeedsOperator reports whether this proxy cannot serve a pinned consumer
// until a human acts (renew, re-enable, re-bind). An unhealthy proxy is not
// in this set: that verdict is transient.
func (p ProxyListItem) NeedsOperator() bool {
	return p.Leasability == LeasabilityExpired || p.Leasability == LeasabilityDisabled
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

// Proxy is a proxy with the lifecycle facts a chooser needs on top of the list
// shape.
//
// An expired proxy is NOT deleted when its term ends: vibe-proxy keeps it —
// unleasable, findable and renewable at the SAME exit IP — until RenewableUntil,
// and only then reaps it. Whether it is expired is the server-derived
// ProxyListItem.Expired field; there is no client-side clock check, because a
// second opinion computed on another machine's clock is how the two disagree.
// HealthStatus is EFFECTIVE health — vibe-proxy reports a stale summary as
// "unknown" rather than letting a frozen verdict pass for a live one.
type Proxy struct {
	ProxyListItem
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// RenewableUntil is expires_at + the retention window: the last day the
	// vendor still sells the extension. Present only when ExpiresAt is.
	RenewableUntil *time.Time `json:"renewableUntil,omitempty"`
	HealthStatus   *string    `json:"healthStatus,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	// ResolvedFrom is set when the id asked for was a dead alias and this is
	// the row that now holds the same address — store ID instead.
	ResolvedFrom *uuid.UUID `json:"resolvedFrom,omitempty"`
}

// Provider is who it was bought from, from vibe-proxy's classification label.
func (p Proxy) Provider() string { return p.Labels["provider"] }

// DefaultEndpoint picks the endpoint a browser can actually use.
//
// HTTP is preferred over SOCKS deliberately: Chromium accepts an authenticated
// SOCKS5 proxy and then fails every navigation with ERR_NO_SUPPORTED_PROXIES.
func DefaultEndpoint(p Proxy) (protocol string, port int, ok bool) {
	var fallback *ProxyEndpoint
	for i := range p.Endpoints {
		e := &p.Endpoints[i]
		switch strings.ToLower(e.Protocol) {
		case "http", "https":
			if e.IsDefault {
				return e.Protocol, e.Port, true
			}
			if fallback == nil || !strings.EqualFold(fallback.Protocol, "http") {
				fallback = e
			}
		default:
			if fallback == nil {
				fallback = e
			}
		}
	}
	if fallback == nil {
		return "", 0, false
	}
	return fallback.Protocol, fallback.Port, true
}

// ListProxiesInput narrows a listing. Every set axis is ANDed server-side.
type ListProxiesInput struct {
	// Search matches proxy name and host.
	Search string
	// CountryCode is the ISO-3166 alpha-2 geo filter, case-insensitive.
	CountryCode string
	// Status filters by lifecycle state ("enabled" / "disabled" / "banned").
	Status string
	// Labels narrows to proxies carrying every key=value. vibe-proxy stamps
	// request_id on the proxies a fulfilled request produced, which is the ONLY
	// way to learn what a request turned into — the request itself never names
	// them, so a purchase whose response was lost is only recoverable this way.
	Labels map[string]string
	// Expired narrows to proxies whose term has (true) or has not (false) run
	// out. Nil means both.
	Expired *bool
	Limit   int
	// Sort/Order are passed through verbatim ("created_at" + "desc" puts the
	// proxy somebody just bought where they expect to find it).
	Sort  string
	Order string
}

func (in ListProxiesInput) query() url.Values {
	q := url.Values{}
	if in.Search != "" {
		q.Set("search", in.Search)
	}
	if in.CountryCode != "" {
		q.Set("country_code", strings.ToUpper(in.CountryCode))
	}
	if in.Status != "" {
		q.Set("status", in.Status)
	}
	if len(in.Labels) > 0 {
		pairs := make([]string, 0, len(in.Labels))
		for k, v := range in.Labels {
			pairs = append(pairs, k+"="+v)
		}
		sort.Strings(pairs)
		q.Set("labels", strings.Join(pairs, ","))
	}
	if in.Expired != nil {
		q.Set("expired", strconv.FormatBool(*in.Expired))
	}
	if in.Limit > 0 {
		q.Set("limit", strconv.Itoa(in.Limit))
	}
	if in.Sort != "" {
		q.Set("sort", in.Sort)
	}
	if in.Order != "" {
		q.Set("order", in.Order)
	}
	return q
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
	// ConnectionType declares the network class when the caller knows it.
	// Omitted → 'unknown', later self-healed from intelligence server-side.
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

// ListProxies retrieves proxies matching every set filter.
//
// vibe-proxy has no per-tenant view of proxies, so this is the whole
// organisation's stock. That is deliberate rather than overlooked: a proxy an
// operator bought is exactly what a buyer should be able to attach.
func (c *Client) ListProxies(ctx context.Context, in ListProxiesInput) ([]Proxy, error) {
	path := "/api/v1/proxies"
	if q := in.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("list proxies: %w", err)
	}
	result, err := decodeResponse[struct {
		Items []Proxy `json:"items"`
	}](resp)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetProxy resolves one proxy by id. A missing proxy is ErrNotFound.
func (c *Client) GetProxy(ctx context.Context, id uuid.UUID) (*Proxy, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/proxies/"+id.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("get proxy: %w", err)
	}
	return decodeResponse[Proxy](resp)
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
// This is exact/observed resolution ONLY — never network matching. Several
// rotating mobile proxies can share one carrier ASN, so an unpinned lookup
// that matched by network would have to pick one of them arbitrarily. A caller
// that already knows which proxy the profile is bound to must ask CheckExitIP
// instead; that is the call that tolerates a rotating mobile exit.
//
// Returns (nil, nil) if no proxy matched within the window. The returned proxy
// carries Expired/Leasability and is returned regardless of them.
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

// RenewProxyInput is the term a renewal buys. A zero PeriodDays takes the
// server's default (30 days, the same term a purchase defaults to).
type RenewProxyInput struct {
	PeriodDays int `json:"periodDays,omitempty"`
}

// RenewProxy extends an existing proxy's term with the provider that sold it,
// and returns the proxy with its new expiry.
//
// Renewal is the cheap half of "the proxy died": it keeps the SAME exit IP,
// while buying a replacement hands the consumer a new address — which, for an
// account somebody is working, is exactly the change an anti-fraud check looks
// for. Try this first; treat a replacement as the fallback.
//
// Two refusals are worth separating, both via RenewRejectionReason:
//
//   - ReasonRenewRefused (409) — the vendor said no. Retrying spends the same
//     refusal; buy a replacement instead.
//   - ReasonRenewUnsupported (400) — the proxy was entered by hand, or its
//     provider has no renew API. There is nobody to ask.
//
// A provider that is merely unreachable comes back as a plain 502 with no
// reason, and retrying that one is right.
func (c *Client) RenewProxy(ctx context.Context, id uuid.UUID, periodDays int) (*Proxy, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/proxies/"+id.String()+"/renew",
		RenewProxyInput{PeriodDays: periodDays})
	if err != nil {
		return nil, fmt.Errorf("renew proxy: %w", err)
	}
	return decodeResponse[Proxy](resp)
}

// ProxyHealthCheck is one live probe of a proxy — the result of asking for a
// check right now, rather than the stored summary.
//
// Conclusive is the field that matters when reporting to a person: a check can
// fail because no probe target answered, which says nothing about the proxy.
// Rendering that as "the proxy is down" is how a healthy fleet gets declared
// dead.
type ProxyHealthCheck struct {
	ID             uuid.UUID `json:"id"`
	ProxyID        uuid.UUID `json:"proxyId"`
	EndpointID     uuid.UUID `json:"endpointId"`
	CheckType      string    `json:"checkType"`
	Success        bool      `json:"success"`
	Conclusive     bool      `json:"conclusive"`
	LatencyMs      *int      `json:"latencyMs,omitempty"`
	ExternalIP     *string   `json:"externalIp,omitempty"`
	AnonymityLevel *string   `json:"anonymityLevel,omitempty"`
	ErrorMsg       *string   `json:"errorMsg,omitempty"`
	CheckedAt      time.Time `json:"checkedAt"`
}

// CheckProxyHealth probes a proxy on demand and returns what the probe saw.
//
// Use it when a person asked "is it alive?" — the stored verdict on
// Proxy.HealthStatus is a background sweep's, and can be minutes old. There is
// deliberately no client method for the raw stored summary: it is NOT staleness
// adjusted, while Proxy.HealthStatus is, and offering both would let a frozen
// verdict pass for a live one.
func (c *Client) CheckProxyHealth(ctx context.Context, id uuid.UUID) (*ProxyHealthCheck, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/proxies/"+id.String()+"/health/check", nil)
	if err != nil {
		return nil, fmt.Errorf("check proxy health: %w", err)
	}
	return decodeResponse[ProxyHealthCheck](resp)
}
