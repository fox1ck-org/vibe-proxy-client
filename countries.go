package vibeproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

// CountryAvailability is one row of the geo-availability summary: how much
// inventory exists in a country and how much of it a lease could take right
// now.
//
// Total without Available is the interesting case. It means proxies for that
// country exist but are disabled, out of term, or failing health checks —
// which a picker should present as "temporarily unavailable" rather than as
// "we do not cover this country". The two ask for different action.
type CountryAvailability struct {
	CountryCode string `json:"countryCode"`
	Total       int    `json:"total"`
	Available   int    `json:"available"`
}

// ListCountriesInput scopes the summary. Every field is optional, but a caller
// that intends to lease should pass the SAME axes it will lease with —
// otherwise the counts describe inventory its own AcquireLease would filter
// away, and the picker promises countries that then fail to allocate.
type ListCountriesInput struct {
	// PoolID scopes the answer to one pool's effective membership.
	PoolID *uuid.UUID
	// ConnectionType narrows to a declared network class (mobile, residential,
	// datacenter, isp).
	ConnectionType *string
	// Protocol narrows to proxies exposing that protocol's endpoint.
	Protocol *string
}

func (in ListCountriesInput) query() string {
	q := url.Values{}
	if in.PoolID != nil {
		q.Set("pool_id", in.PoolID.String())
	}
	if in.ConnectionType != nil && *in.ConnectionType != "" {
		q.Set("connection_type", *in.ConnectionType)
	}
	if in.Protocol != nil && *in.Protocol != "" {
		q.Set("protocol", *in.Protocol)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListCountries summarises proxy inventory by ISO country code, so a consumer
// can offer a country picker that knows what it can actually deliver.
//
// The server returns a bare array (as GET /api/v1/pools does), not a paginated
// envelope: the cardinality is countries-we-own, which is small and bounded.
func (c *Client) ListCountries(ctx context.Context, in ListCountriesInput) ([]CountryAvailability, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/proxies/countries"+in.query(), nil)
	if err != nil {
		return nil, fmt.Errorf("list countries: %w", err)
	}
	out, err := decodeResponse[[]CountryAvailability](resp)
	if err != nil {
		return nil, err
	}
	return *out, nil
}
