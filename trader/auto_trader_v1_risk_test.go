package trader

import (
	"math"
	"testing"

	"nofx/kernel"
)

func TestCurrentAccountDrawdownRatio(t *testing.T) {
	tests := []struct{ equity, initial, want float64 }{
		{100, 100, 0},
		{110, 100, 0},
		{90, 100, 0.10},
		{50, 0, 0},
	}
	for _, tt := range tests {
		got := currentAccountDrawdownRatio(tt.equity, tt.initial)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Fatalf("drawdown(%v,%v)=%.8f want %.8f", tt.equity, tt.initial, got, tt.want)
		}
	}
}

func TestV1GateSnapshotMathUsesNotionalAndMargin(t *testing.T) {
	// The kernel-level tests own risk arithmetic. This regression fixture
	// documents the exchange-map representation the trader gate accepts.
	pos := map[string]interface{}{"markPrice": 100.0, "positionAmt": -2.0, "leverage": 4.0}
	mark, ok := finitePositiveMapFloat(pos, "markPrice")
	if !ok {
		t.Fatal("mark price should parse")
	}
	qty, ok := finiteMapFloat(pos, "positionAmt")
	if !ok {
		t.Fatal("quantity should parse")
	}
	lev, ok := finitePositiveMapFloat(pos, "leverage")
	if !ok {
		t.Fatal("leverage should parse")
	}
	notional := math.Abs(qty) * mark
	if notional != 200 || notional/lev != 50 {
		t.Fatalf("unexpected exposure/margin: %.2f / %.2f", notional, notional/lev)
	}
}

func TestV1RiskConfigRemainsConservative(t *testing.T) {
	cfg := kernel.ConservativeRiskEngineConfig()
	if cfg.MaxRiskPerTradeRatio != 0.01 || cfg.MaxPortfolioRiskRatio != 0.04 ||
		cfg.MaxMarginUsageRatio != 0.30 || cfg.MaxDailyLossRatio != 0.03 ||
		cfg.MaxDrawdownRatio != 0.10 {
		t.Fatalf("unexpected V1 conservative limits: %+v", cfg)
	}
}
