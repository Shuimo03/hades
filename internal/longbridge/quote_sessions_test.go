package longbridge

import (
	"testing"

	"github.com/longportapp/openapi-go/quote"
	"github.com/shopspring/decimal"
)

func TestResolveEffectiveQuoteRegularScope(t *testing.T) {
	q := &quote.SecurityQuote{
		Symbol:    "AAPL.US",
		LastDone:  decimalPtr("190.5"),
		Timestamp: 1_710_000_000,
		PreMarketQuote: &quote.PrePostQuote{
			LastDone:  decimalPtr("191.2"),
			Timestamp: 1_710_000_100,
		},
	}

	effective := ResolveEffectiveQuote(q, QuoteSessionScopeRegular)
	if !effective.HasQuote {
		t.Fatalf("expected effective quote")
	}
	if effective.Session != QuoteSessionRegular {
		t.Fatalf("expected regular session, got %s", effective.Session)
	}
	if effective.Price != 190.5 {
		t.Fatalf("expected regular price, got %.2f", effective.Price)
	}
}

func TestResolveEffectiveQuoteExtendedScope(t *testing.T) {
	q := &quote.SecurityQuote{
		Symbol:    "AAPL.US",
		LastDone:  decimalPtr("190.5"),
		Timestamp: 1_710_000_000,
		PostMarketQuote: &quote.PrePostQuote{
			LastDone:  decimalPtr("192.8"),
			Timestamp: 1_710_000_200,
		},
	}

	effective := ResolveEffectiveQuote(q, QuoteSessionScopeExtended)
	if !effective.HasQuote {
		t.Fatalf("expected effective quote")
	}
	if effective.Session != QuoteSessionPostMarket {
		t.Fatalf("expected post-market session, got %s", effective.Session)
	}
	if effective.Price != 192.8 {
		t.Fatalf("expected post-market price, got %.2f", effective.Price)
	}
	if !effective.IsExtended {
		t.Fatalf("expected extended quote flag")
	}
}

func TestParseQuoteSessionScope(t *testing.T) {
	scope, valid := ParseQuoteSessionScope("regular")
	if !valid || scope != QuoteSessionScopeRegular {
		t.Fatalf("expected regular scope, got %s valid=%v", scope, valid)
	}

	scope, valid = ParseQuoteSessionScope("unexpected")
	if valid {
		t.Fatalf("expected invalid scope")
	}
	if scope != QuoteSessionScopeExtended {
		t.Fatalf("expected fallback to extended, got %s", scope)
	}
}

func TestParseCandlestickTradeSession(t *testing.T) {
	session, valid := ParseCandlestickTradeSession("all")
	if !valid || session != quote.CandlestickTradeSessionAll {
		t.Fatalf("expected all session, got %v valid=%v", session, valid)
	}

	session, valid = ParseCandlestickTradeSession("invalid")
	if valid {
		t.Fatalf("expected invalid trade session")
	}
	if session != quote.CandlestickTradeSessionNormal {
		t.Fatalf("expected fallback to regular session, got %v", session)
	}
}

func decimalPtr(value string) *decimal.Decimal {
	v := decimal.RequireFromString(value)
	return &v
}
