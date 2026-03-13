package longbridge

import (
	"context"
	"encoding/json"
	"fmt"
)

type WatchlistSecurity struct {
	Symbol       string `json:"symbol"`
	Market       string `json:"market"`
	Name         string `json:"name"`
	WatchedPrice string `json:"watched_price"`
	WatchedAt    string `json:"watched_at"`
}

type WatchlistGroup struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Securities []*WatchlistSecurity `json:"securities"`
}

type watchlistGroupsResponse struct {
	Groups []*WatchlistGroup `json:"groups"`
}

func (r *watchlistGroupsResponse) UnmarshalJSON(data []byte) error {
	var direct struct {
		Groups []*WatchlistGroup `json:"groups"`
	}
	if err := json.Unmarshal(data, &direct); err == nil && len(direct.Groups) > 0 {
		r.Groups = direct.Groups
		return nil
	}

	var wrapped struct {
		Data struct {
			Groups []*WatchlistGroup `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return err
	}
	r.Groups = wrapped.Data.Groups
	return nil
}

func (c *Client) GetWatchlistGroups(ctx context.Context) ([]*WatchlistGroup, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("http client is not initialized")
	}

	var resp watchlistGroupsResponse
	if err := c.httpClient.Get(ctx, "/v1/watchlist/groups", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Groups, nil
}
