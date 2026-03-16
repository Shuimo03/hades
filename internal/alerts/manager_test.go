package alerts

import (
	"testing"

	"hades/internal/longbridge"
)

func TestQuoteScopeForAlert(t *testing.T) {
	mgr := New(nil, "", 0, longbridge.QuoteSessionScopeExtended)

	if got := mgr.QuoteScopeForAlert(&Alert{}); got != longbridge.QuoteSessionScopeExtended {
		t.Fatalf("expected manager default scope, got %s", got)
	}

	if got := mgr.QuoteScopeForAlert(&Alert{SessionScope: "regular"}); got != longbridge.QuoteSessionScopeRegular {
		t.Fatalf("expected alert regular scope, got %s", got)
	}

	if got := mgr.QuoteScopeForAlert(&Alert{SessionScope: "invalid"}); got != longbridge.QuoteSessionScopeExtended {
		t.Fatalf("expected fallback scope, got %s", got)
	}
}
