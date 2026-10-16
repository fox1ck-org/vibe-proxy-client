package vibeproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExitIPMatchKind says HOW an observed IP was matched to a pinned proxy. Log it:
// a network match is expected for a mobile proxy and would be a red flag for a
// static one (which never produces it).
type ExitIPMatchKind string

const (
	// MatchExact — the IP is the proxy's current external_ip.
	MatchExact ExitIPMatchKind = "exact"
	// MatchObserved — the IP is in the proxy's own rotation log within `within`.
	MatchObserved ExitIPMatchKind = "observed"
	// MatchNetwork — the proxy is a rotating mobile proxy and the IP belongs to
	// the same ASN.
	MatchNetwork ExitIPMatchKind = "network"
	// MatchNone — not consistent; MismatchReason says why.
	MatchNone ExitIPMatchKind = "none"
)

// ExitIPMismatch is why an IP was judged inconsistent with a pinned proxy.
type ExitIPMismatch string

const (
	// MismatchIPNotObserved — a proxy that does not rotate (or is not mobile)
	// was never seen at this IP. Static proxies keep exact semantics.
	MismatchIPNotObserved ExitIPMismatch = "ip_not_observed"
	// MismatchIPObservedOnOtherProxy — a proxy that does NOT rotate within its
	// network (static, or not mobile) was not seen at this IP, but a DIFFERENT
	// proxy was, inside `within` (ExitIPMatch.ObservedOn). Decided before
	// ip_not_observed. Never produced for a rotating mobile proxy: carriers use
	// CGNAT, so a sibling having sampled the IP proves nothing about this one.
	MismatchIPObservedOnOtherProxy ExitIPMismatch = "ip_observed_on_other_proxy"
	// MismatchNetwork — a rotating mobile proxy, but the IP is in another ASN.
	MismatchNetwork ExitIPMismatch = "network_mismatch"
	// MismatchProxyNetworkUnknown — a rotating mobile proxy whose own ASN has
	// not been learned yet, so there is nothing to compare against.
	MismatchProxyNetworkUnknown ExitIPMismatch = "proxy_network_unknown"
)

// IPNetwork is the network an IP belongs to, as vibe-proxy's ip-intelligence
// lookup resolved it (cached server-side).
//
// The location fields (Region, City, Timezone, Latitude, Longitude) are where
// the IP geolocates — what a website's own GeoIP lookup will say about it.
// They are set by ResolveIPNetwork; CheckExitIP only promises the network
// (ASN, ISP, country).
type IPNetwork struct {
	ASN         *int    `json:"asn,omitempty"`
	ISP         *string `json:"isp,omitempty"`
	CountryCode *string `json:"countryCode,omitempty"`
	Region      *string `json:"region,omitempty"`
	City        *string `json:"city,omitempty"`
	// Timezone is the IANA zone of the IP's location ("Europe/Warsaw").
	Timezone   *string   `json:"timezone,omitempty"`
	Latitude   *float64  `json:"latitude,omitempty"`
	Longitude  *float64  `json:"longitude,omitempty"`
	Source     string    `json:"source"`
	ResolvedAt time.Time `json:"resolvedAt"`
}

// ExitIPMatch is vibe-proxy's answer to "is this exit IP consistent with the
// proxy this profile is bound to?".
type ExitIPMatch struct {
	IP             string          `json:"ip"`
	Consistent     bool            `json:"consistent"`
	Match          ExitIPMatchKind `json:"match"`
	MismatchReason ExitIPMismatch  `json:"mismatchReason,omitempty"`
	// IPNetwork is set whenever the server resolved the IP's network.
	IPNetwork *IPNetwork `json:"ipNetwork,omitempty"`
	// ObservedOn is a different proxy whose rotation log (proxy_ip_observation,
	// inside `within` — never a stale external_ip snapshot) holds this IP. For a
	// static proxy it comes with MismatchIPObservedOnOtherProxy and the caller
	// decides whether to rebind. For a rotating mobile proxy it is INFORMATIONAL
	// and may accompany a consistent `network` answer as well as
	// `network_mismatch`: carrier CGNAT hands the same address to siblings.
	ObservedOn *Proxy `json:"observedOn,omitempty"`
	// Proxy is the pinned proxy (alias-followed: ResolvedFrom is set when the id
	// asked for was a dead one). It carries Expired/Leasability: a consistent IP
	// says nothing about whether the proxy can be leased.
	Proxy Proxy `json:"proxy"`
}

// CheckExitIP asks whether an IP a browser reports as its public exit is
// consistent with the proxy the caller has pinned.
//
//   - exact    — the pinned proxy's current external_ip.
//   - observed — the pinned proxy's own proxy_ip_observation within `within`.
//   - network  — the pinned proxy rotates (rotating | sticky_rotating) AND is
//     connection_type mobile AND the IP's ASN equals the proxy's ASN.
//   - none     — otherwise; MismatchReason explains.
//
// Order for a proxy that rotates within its network (rotating|sticky_rotating
// AND mobile): exact → observed → network compare. An IP sampled on another
// proxy does not decide the answer there (carrier CGNAT shares addresses); it
// is reported in ObservedOn for the caller to log. For every other proxy the
// order is exact → observed → ip_observed_on_other_proxy → ip_not_observed:
// static proxies never network-match. "Another proxy" evidence is read only
// from proxy_ip_observation inside `within`.
//
// This exists because a rotating mobile proxy's exit changes constantly —
// Z-Proxy/UA/1 showed 130 distinct IPs in 24h, each for under half an hour — so
// most of what a browser reports behind it was never sampled, and an exact
// lookup called a working profile "no registered proxy" on every tick.
//
// Errors: 400 for a bad ip/within (no reason); 404 *APIError with reason
// proxy_not_found (not swallowed — a pinned proxy that does not exist needs a
// human); 503 *APIError with reason network_lookup_unavailable — transient,
// retry later and do not record a mismatch.
func (c *Client) CheckExitIP(ctx context.Context, proxyID uuid.UUID, ip string, within time.Duration) (*ExitIPMatch, error) {
	ip = strings.TrimSpace(ip)
	if proxyID == uuid.Nil {
		return nil, fmt.Errorf("proxy id is required: %w", ErrInvalidInput)
	}
	if ip == "" {
		return nil, fmt.Errorf("ip is required: %w", ErrInvalidInput)
	}
	if within <= 0 {
		return nil, fmt.Errorf("within must be positive: %w", ErrInvalidInput)
	}

	q := url.Values{}
	q.Set("ip", ip)
	q.Set("within", within.String())
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/proxies/"+proxyID.String()+"/exit-ip-match?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("check exit ip: %w", err)
	}
	return decodeResponse[ExitIPMatch](resp)
}

// ResolveIPNetwork says where an arbitrary IP is and which network it is in:
// ASN, ISP, country, city, IANA timezone and coordinates, resolved by
// vibe-proxy's ip-intelligence providers and cached server-side by IP.
//
// It is how an anti-detect consumer makes the browser agree with its exit: the
// timezone a site derives from the exit IP must be the one the browser
// reports. Ask for the lease's ExternalIP — the address the world sees — not
// for the address you dial.
//
// A lookup no provider could answer is an *APIError with StatusCode 503
// (after the client's own retries): transient, and never to be read as "this
// IP has no location".
func (c *Client) ResolveIPNetwork(ctx context.Context, ip string) (*IPNetwork, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return nil, fmt.Errorf("ip is required: %w", ErrInvalidInput)
	}
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/ip-networks/"+url.PathEscape(ip), nil)
	if err != nil {
		return nil, fmt.Errorf("resolve ip network: %w", err)
	}
	return decodeResponse[IPNetwork](resp)
}
