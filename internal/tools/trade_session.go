package tools

import (
	"fmt"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
)

func parseTradeSessionArg(raw interface{}) (quote.CandlestickTradeSession, error) {
	if raw == nil {
		return quote.CandlestickTradeSessionNormal, nil
	}
	value, ok := raw.(string)
	if !ok {
		return quote.CandlestickTradeSessionNormal, fmt.Errorf("trade_session must be a string")
	}
	session, valid := longbridge.ParseCandlestickTradeSession(value)
	if !valid {
		return quote.CandlestickTradeSessionNormal, fmt.Errorf("invalid trade_session: %s", value)
	}
	return session, nil
}
