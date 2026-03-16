package longbridge

import (
	"strings"

	"github.com/longportapp/openapi-go/quote"
	"github.com/shopspring/decimal"
)

type QuoteSession string

const (
	QuoteSessionUnknown    QuoteSession = "unknown"
	QuoteSessionRegular    QuoteSession = "regular"
	QuoteSessionPreMarket  QuoteSession = "pre_market"
	QuoteSessionPostMarket QuoteSession = "post_market"
	QuoteSessionOvernight  QuoteSession = "overnight"
)

type QuoteSessionScope string

const (
	QuoteSessionScopeRegular  QuoteSessionScope = "regular"
	QuoteSessionScopeExtended QuoteSessionScope = "extended"
)

const (
	CandlestickTradeSessionRegular = "regular"
	CandlestickTradeSessionAll     = "all"
)

type EffectiveQuote struct {
	Symbol          string
	Session         QuoteSession
	Price           float64
	TimestampMillis int64
	Volume          int64
	High            float64
	Low             float64
	PrevClose       float64
	Open            float64
	HasOpen         bool
	HasQuote        bool
	IsExtended      bool
}

type quoteCandidate struct {
	session         QuoteSession
	price           float64
	timestampMillis int64
	volume          int64
	high            float64
	low             float64
	prevClose       float64
	open            float64
	hasOpen         bool
}

func ParseQuoteSessionScope(raw string) (QuoteSessionScope, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(QuoteSessionScopeExtended):
		return QuoteSessionScopeExtended, true
	case string(QuoteSessionScopeRegular):
		return QuoteSessionScopeRegular, true
	default:
		return QuoteSessionScopeExtended, false
	}
}

func ParseCandlestickTradeSession(raw string) (quote.CandlestickTradeSession, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", CandlestickTradeSessionRegular:
		return quote.CandlestickTradeSessionNormal, true
	case CandlestickTradeSessionAll, string(QuoteSessionScopeExtended):
		return quote.CandlestickTradeSessionAll, true
	default:
		return quote.CandlestickTradeSessionNormal, false
	}
}

func CandlestickTradeSessionFromScope(scope QuoteSessionScope) quote.CandlestickTradeSession {
	if scope == QuoteSessionScopeExtended {
		return quote.CandlestickTradeSessionAll
	}
	return quote.CandlestickTradeSessionNormal
}

func QuoteSessionDisplayName(session QuoteSession) string {
	switch session {
	case QuoteSessionRegular:
		return "常规盘"
	case QuoteSessionPreMarket:
		return "盘前"
	case QuoteSessionPostMarket:
		return "盘后"
	case QuoteSessionOvernight:
		return "夜盘"
	default:
		return "未知时段"
	}
}

func ResolveEffectiveQuote(q *quote.SecurityQuote, scope QuoteSessionScope) EffectiveQuote {
	if q == nil {
		return EffectiveQuote{}
	}

	candidates := make([]quoteCandidate, 0, 4)
	if candidate, ok := buildRegularQuoteCandidate(q); ok {
		candidates = append(candidates, candidate)
	}
	if scope == QuoteSessionScopeExtended {
		if candidate, ok := buildPrePostQuoteCandidate(QuoteSessionPreMarket, q.PreMarketQuote); ok {
			candidates = append(candidates, candidate)
		}
		if candidate, ok := buildPrePostQuoteCandidate(QuoteSessionPostMarket, q.PostMarketQuote); ok {
			candidates = append(candidates, candidate)
		}
		if candidate, ok := buildPrePostQuoteCandidate(QuoteSessionOvernight, q.OverNightQuote); ok {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return EffectiveQuote{Symbol: q.Symbol}
	}

	selected := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.timestampMillis > selected.timestampMillis {
			selected = candidate
		}
	}

	return EffectiveQuote{
		Symbol:          q.Symbol,
		Session:         selected.session,
		Price:           selected.price,
		TimestampMillis: selected.timestampMillis,
		Volume:          selected.volume,
		High:            selected.high,
		Low:             selected.low,
		PrevClose:       selected.prevClose,
		Open:            selected.open,
		HasOpen:         selected.hasOpen,
		HasQuote:        true,
		IsExtended:      selected.session != QuoteSessionRegular,
	}
}

func buildRegularQuoteCandidate(q *quote.SecurityQuote) (quoteCandidate, bool) {
	if q == nil || q.LastDone == nil {
		return quoteCandidate{}, false
	}
	return quoteCandidate{
		session:         QuoteSessionRegular,
		price:           decimalPtrToFloat(q.LastDone),
		timestampMillis: normalizeQuoteTimestamp(q.Timestamp),
		volume:          q.Volume,
		high:            decimalPtrToFloat(q.High),
		low:             decimalPtrToFloat(q.Low),
		prevClose:       decimalPtrToFloat(q.PrevClose),
		open:            decimalPtrToFloat(q.Open),
		hasOpen:         q.Open != nil,
	}, true
}

func buildPrePostQuoteCandidate(session QuoteSession, q *quote.PrePostQuote) (quoteCandidate, bool) {
	if q == nil || q.LastDone == nil {
		return quoteCandidate{}, false
	}
	return quoteCandidate{
		session:         session,
		price:           decimalPtrToFloat(q.LastDone),
		timestampMillis: normalizeQuoteTimestamp(q.Timestamp),
		volume:          q.Volume,
		high:            decimalPtrToFloat(q.High),
		low:             decimalPtrToFloat(q.Low),
		prevClose:       decimalPtrToFloat(q.PrevClose),
	}, true
}

func decimalPtrToFloat(value *decimal.Decimal) float64 {
	if value == nil {
		return 0
	}
	result, _ := value.Float64()
	return result
}

func normalizeQuoteTimestamp(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	if ts > 1_000_000_000_000 {
		return ts
	}
	return ts * 1000
}
