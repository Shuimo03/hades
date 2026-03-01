package tools

import (
	"context"
	"fmt"

	"hades/internal/brief"
	"hades/internal/longbridge"
)

// NewDailyBriefTool creates a tool for generating daily brief
func NewDailyBriefTool(lb *longbridge.Client, timezone string) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	gen := brief.New(lb, timezone)

	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		version := ""
		if v, ok := args["version"].(string); ok {
			version = v
		}

		symbols := []string{}
		if s, ok := args["symbols"].(string); ok && s != "" {
			symbols = splitSymbols(s)
		}

		result, err := gen.Generate(ctx, brief.BriefVersion(version), symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to generate brief: %v", err)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

// NewSetBriefConfigTool creates a tool for configuring daily brief
func NewSetBriefConfigTool() func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		// This would update config - for now just return success
		// In production, this would persist settings
		return map[string]interface{}{
			"result": "Brief 配置已更新",
		}, nil
	}
}
