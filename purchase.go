package vibeproxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// PurchaseInput is the order placed against a vendor to satisfy a request.
// Mirrors vibe-proxy's domain.PurchaseInput.
type PurchaseInput struct {
	Vendor     string `json:"vendor"`
	ProxyType  string `json:"proxyType"`
	IPVersion  int    `json:"ipVersion"`
	Country    string `json:"country"`
	Quantity   int    `json:"quantity"`
	PeriodDays int    `json:"periodDays"`
}

// PurchaseEstimate is the pre-buy quote.
//
// It doubles as the supply probe: vibe-proxy exposes no route reporting a
// vendor's stock, and GET /api/v1/proxies/countries describes OUR OWN fleet,
// not what is buyable. A refused estimate is how "nothing available in that
// country" arrives.
type PurchaseEstimate struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Quantity int     `json:"quantity"`
}

// PurchaseOutcome is returned after a successful buy-and-fulfill.
//
// Count can be smaller than the order: vibe-proxy logs and skips a proxy it
// could not import, then fulfils the request anyway — so a fulfilled request
// with an empty ProxyIDs is a real outcome, not an impossible one. Treat it as
// "the money is spent, the inventory has not caught up yet" and re-read; the
// provider sync worker imports it by external id on its next pass.
type PurchaseOutcome struct {
	Request  *ProxyRequest `json:"request"`
	ProxyIDs []uuid.UUID   `json:"proxyIds"`
	Count    int           `json:"count"`
}

// FulfillInput links already-imported proxies to a request by hand.
type FulfillInput struct {
	ProxyIDs []uuid.UUID `json:"proxyIds"`
	Note     string      `json:"note,omitempty"`
}

// ClaimRequest takes ownership of a request as the operator. Operator-only.
// Claiming a request already in progress is not an error — vibe-proxy accepts
// a transition to the state it is already in, which is what makes a retried
// step safe.
func (c *Client) ClaimRequest(ctx context.Context, id uuid.UUID) (*ProxyRequest, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/claim", nil)
	if err != nil {
		return nil, fmt.Errorf("claim request: %w", err)
	}
	return decodeResponse[ProxyRequest](resp)
}

// MarkPurchasing moves a claimed request into the purchasing state.
func (c *Client) MarkPurchasing(ctx context.Context, id uuid.UUID) (*ProxyRequest, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/mark-purchasing", nil)
	if err != nil {
		return nil, fmt.Errorf("mark request purchasing: %w", err)
	}
	return decodeResponse[ProxyRequest](resp)
}

// CancelRequest closes a request nobody is going to fulfil.
func (c *Client) CancelRequest(ctx context.Context, id uuid.UUID) (*ProxyRequest, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/cancel", nil)
	if err != nil {
		return nil, fmt.Errorf("cancel request: %w", err)
	}
	return decodeResponse[ProxyRequest](resp)
}

// FulfillRequest links proxies to a request and closes it.
func (c *Client) FulfillRequest(ctx context.Context, id uuid.UUID, in FulfillInput) (*ProxyRequest, error) {
	resp, err := c.do(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/fulfill", in)
	if err != nil {
		return nil, fmt.Errorf("fulfill request: %w", err)
	}
	return decodeResponse[ProxyRequest](resp)
}

// EstimateOrder quotes an order without placing it. Safe to repeat.
func (c *Client) EstimateOrder(ctx context.Context, id uuid.UUID, in PurchaseInput) (*PurchaseEstimate, error) {
	resp, err := c.doOnce(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/estimate", in)
	if err != nil {
		return nil, fmt.Errorf("estimate order: %w", err)
	}
	return decodeResponse[PurchaseEstimate](resp)
}

// PurchaseForRequest buys from the vendor, imports what came back, and fulfils
// the request — all inside this one call.
//
// It is NOT retried and must not be retried blind: vibe-proxy's buy route
// carries no idempotency key, so a lost response is an unknown outcome, not a
// failed one. Recover by reading: GET the request, and list proxies filtered by
// the request_id label vibe-proxy stamps on what a fulfilled request produced.
func (c *Client) PurchaseForRequest(ctx context.Context, id uuid.UUID, in PurchaseInput) (*PurchaseOutcome, error) {
	resp, err := c.doOnce(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/purchase", in)
	if err != nil {
		return nil, fmt.Errorf("purchase for request: %w", err)
	}
	return decodeResponse[PurchaseOutcome](resp)
}

// ProxiesForRequest returns what a request actually produced.
//
// This is the recovery read after an interrupted purchase, and the only link
// there is: the request row never names its proxies, vibe-proxy records the
// relation as a label on each proxy it imported.
func (c *Client) ProxiesForRequest(ctx context.Context, id uuid.UUID) ([]Proxy, error) {
	return c.ListProxies(ctx, ListProxiesInput{
		Labels: map[string]string{"request_id": id.String()},
		Limit:  100,
	})
}
