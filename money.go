package vibeproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// StockCharge is a purchase or renewal vibe-proxy booked for nobody (line
// proxy.stock in vibe-finance's books): what ProxyLine charged for a proxy
// bought for the fleet or for an account in stock.
type StockCharge struct {
	// Seq is vibe-proxy's booking order: page with the last one seen.
	Seq int64 `json:"seq"`
	// Key is the charge's money key: proxy_purchase:<proxy id> or
	// proxy_renewal:<audit event id>.
	Key     string `json:"key"`
	ProxyID string `json:"proxy_id"`
	// AccountID is the vibe-accounts account it was bought for, "" = none said.
	AccountID   string    `json:"account_id,omitempty"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// StockChargesPage is one page of ListStockCharges; Next is the seq to ask
// after (the last one served, or the one asked after when none was).
type StockChargesPage struct {
	Charges []StockCharge `json:"charges"`
	Next    int64         `json:"next"`
}

// MaxStockChargesPage is the most one page serves.
const MaxStockChargesPage = 1000

// ListStockCharges lists the proxy.stock charges booked after seq `after`
// (0 = from the first), up to limit (1..MaxStockChargesPage), in booking
// order. A charge is served once its booking has settled, so a caller paging
// by Next never skips one.
func (c *Client) ListStockCharges(ctx context.Context, after int64, limit int) (*StockChargesPage, error) {
	q := url.Values{}
	q.Set("after", strconv.FormatInt(after, 10))
	q.Set("limit", strconv.Itoa(limit))
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/money/stock-charges?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("list stock charges: %w", err)
	}
	return decodeResponse[StockChargesPage](resp)
}
