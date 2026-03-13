package tools

import (
	"context"
	"fmt"
	"strings"

	"hades/internal/longbridge"
)

func NewWatchlistGroupsTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		groups, err := lb.GetWatchlistGroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get watchlist groups: %v", err)
		}

		items := make([]map[string]interface{}, 0, len(groups))
		for _, group := range groups {
			if group == nil {
				continue
			}

			securities := make([]map[string]interface{}, 0, len(group.Securities))
			for _, security := range group.Securities {
				if security == nil {
					continue
				}
				securities = append(securities, map[string]interface{}{
					"symbol":        security.Symbol,
					"market":        security.Market,
					"name":          security.Name,
					"watched_price": security.WatchedPrice,
					"watched_at":    security.WatchedAt,
				})
			}

			items = append(items, map[string]interface{}{
				"id":         group.ID,
				"name":       group.Name,
				"count":      len(securities),
				"securities": securities,
			})
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"analysis_of": "watchlist_groups",
				"count":       len(items),
				"items":       items,
			},
		}, nil
	}
}

func resolveSymbolsFromArgs(ctx context.Context, lb *longbridge.Client, args map[string]interface{}) ([]string, map[string]interface{}, error) {
	if symbols := splitStringArg(args["symbols"]); len(symbols) > 0 {
		symbols = dedupeStrings(symbols)
		return symbols, map[string]interface{}{
			"type":    "symbols",
			"symbols": symbols,
		}, nil
	}

	groupName, _ := args["group_name"].(string)
	groupID, _ := args["group_id"].(string)
	groupName = strings.TrimSpace(groupName)
	groupID = strings.TrimSpace(groupID)
	if groupName == "" && groupID == "" {
		return nil, nil, fmt.Errorf("missing symbols or watchlist group parameter")
	}

	groups, err := lb.GetWatchlistGroups(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get watchlist groups: %v", err)
	}

	for _, group := range groups {
		if group == nil {
			continue
		}
		if groupID != "" && group.ID != groupID {
			continue
		}
		if groupName != "" && !strings.EqualFold(group.Name, groupName) {
			continue
		}

		symbols := make([]string, 0, len(group.Securities))
		for _, security := range group.Securities {
			if security == nil || strings.TrimSpace(security.Symbol) == "" {
				continue
			}
			symbols = append(symbols, security.Symbol)
		}
		symbols = dedupeStrings(symbols)
		if len(symbols) == 0 {
			return nil, nil, fmt.Errorf("watchlist group %q is empty", group.Name)
		}

		return symbols, map[string]interface{}{
			"type":       "watchlist_group",
			"group_id":   group.ID,
			"group_name": group.Name,
			"symbols":    symbols,
		}, nil
	}

	if groupID != "" {
		return nil, nil, fmt.Errorf("watchlist group_id %q not found", groupID)
	}
	return nil, nil, fmt.Errorf("watchlist group_name %q not found", groupName)
}
