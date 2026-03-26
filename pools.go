package vibeproxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// PoolList is the response from listing pools.
type PoolList struct {
	Items []Pool `json:"items"`
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
