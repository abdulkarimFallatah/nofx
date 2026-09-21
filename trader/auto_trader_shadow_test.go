package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestShadowModeWaitIsNoOp(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{ShadowMode: true}}
	d := &kernel.Decision{Symbol: "BTCUSDT", Action: "wait"}
	if err := at.executeDecisionWithRecord(d, &store.DecisionAction{}); err != nil {
		t.Fatalf("shadow wait returned error: %v", err)
	}
}

func TestShadowModeCloseNeverTouchesTrader(t *testing.T) {
	// trader is deliberately nil. If the close path reaches the exchange
	// interface this test panics, proving shadow mode is not observational.
	at := &AutoTrader{config: AutoTraderConfig{ShadowMode: true}}
	for _, action := range []string{"close_long", "close_short"} {
		d := &kernel.Decision{Symbol: "BTCUSDT", Action: action}
		if err := at.executeDecisionWithRecord(d, &store.DecisionAction{}); err != nil {
			t.Fatalf("shadow %s returned error: %v", action, err)
		}
	}
}

func TestShadowModeHoldNeverCancelsOrders(t *testing.T) {
	// Same nil-trader sentinel: signal-managed hold must not cancel TP orders.
	at := &AutoTrader{config: AutoTraderConfig{ShadowMode: true}}
	d := &kernel.Decision{Symbol: "BTCUSDT", Action: "hold"}
	if err := at.executeDecisionWithRecord(d, &store.DecisionAction{}); err != nil {
		t.Fatalf("shadow hold returned error: %v", err)
	}
}
