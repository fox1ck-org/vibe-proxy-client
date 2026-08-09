package vibeproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// PoolList is the response from listing pools.
//
// The server returns a BARE ARRAY from GET /api/v1/pools, while its other
// list endpoints (e.g. /api/v1/proxies) return {"items":[…],"totalCount":N}.
// Decoding only the object shape made ListPools fail every call with
// "json: cannot unmarshal array into Go value of type vibeproxy.PoolList",
// which surfaced far from here — as a failed proxy lease in any consumer that
// addresses a pool by NAME rather than by UUID.
//
// UnmarshalJSON accepts both so the client keeps working whichever shape the
// server settles on.
type PoolList struct {
	Items []Pool `json:"items"`
}

// UnmarshalJSON accepts either a bare array of pools or an object with an
// "items" key.
func (p *PoolList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(trimmed, &p.Items)
	}
	// Alias breaks the recursion into this method.
	type poolListObject struct {
		Items []Pool `json:"items"`
	}
	var obj poolListObject
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return err
	}
	p.Items = obj.Items
	return nil
}

// ListPools retrieves all proxy pools.
func (c *Client) ListPools(ctx context.Context) ([]Pool, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/pools", nil)
	if err != nil {
		return nil, fmt.Errorf("list pools: %w", err)
	}
	result, err := decodeResponse[PoolList](resp)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetPool retrieves a pool by ID.
func (c *Client) GetPool(ctx context.Context, poolID uuid.UUID) (*Pool, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/pools/"+poolID.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("get pool: %w", err)
	}
	return decodeResponse[Pool](resp)
}
