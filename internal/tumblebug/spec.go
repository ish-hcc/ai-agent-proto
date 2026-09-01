package tumblebug

import (
	"context"
	"fmt"
	"net/http"
)

// RecommendSpec asks CB-Tumblebug for node specs matching a filter.
//
// This is the entry point for the AI semiconductor axis: the accelerator fields
// of a spec are filterable metrics, so an application's accelerator requirement
// becomes a filter without any provider specific mapping.
func (c *Client) RecommendSpec(ctx context.Context, req *RecommendSpecReq) ([]SpecInfo, error) {
	var result []SpecInfo
	if err := c.Do(ctx, http.MethodPost, "/recommendSpec", nil, req, &result); err != nil {
		return nil, fmt.Errorf("failed to recommend specs: %w", err)
	}
	if result == nil {
		// No match must serialize as [], not null.
		return []SpecInfo{}, nil
	}
	return result, nil
}
