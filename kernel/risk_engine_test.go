package kernel

import (
	"math"
	"strings"
	"testing"
)

func baseRiskFixture() (RiskEngineConfig, RiskSnapshot, Decision) {
	cfg := ConservativeRiskEngineConfig()
	s := RiskSnapshot{Equity: 1000}
	d := Decision{
		Symbol: "BTCUSDT", Action: "open_long", Leverage: 2,
		PositionSizeUSD: 200, StopLoss: 95, TakeProfit: 115,
		Reasoning: "synthetic risk-engine fixture",
	}
	return cfg, s, d
}

func TestRiskEnginePassesSafeProposal(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskPass {
		t.Fatalf("expected PASS, got %s: %s", got.Verdict, got.Reason)
	}
	if math.Abs(got.EstimatedRiskUSD-10) > 1e-9 {
		t.Fatalf("expected $10 risk, got %.8f", got.EstimatedRiskUSD)
	}
}

func TestRiskEngineResizesTradeRisk(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	d.PositionSizeUSD = 1000
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskResize {
		t.Fatalf("expected RESIZE, got %s: %s", got.Verdict, got.Reason)
	}
	if math.Abs(got.ApprovedPositionUSD-200) > 1e-9 {
		t.Fatalf("expected resize to $200, got %.8f", got.ApprovedPositionUSD)
	}
}

func TestRiskEngineRejectsDailyLossBreaker(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	s.DailyPnL = -31
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskReject || !strings.Contains(got.Reason, "daily loss") {
		t.Fatalf("expected daily-loss REJECT, got %+v", got)
	}
}

func TestRiskEngineRejectsDrawdownBreaker(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	s.DrawdownRatio = 0.10
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskReject || !strings.Contains(got.Reason, "drawdown") {
		t.Fatalf("expected drawdown REJECT, got %+v", got)
	}
}

func TestRiskEngineRespectsPortfolioRiskBudget(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	s.CurrentPortfolioRisk = 35
	d.PositionSizeUSD = 200
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskResize {
		t.Fatalf("expected RESIZE, got %+v", got)
	}
	if math.Abs(got.ApprovedPositionUSD-100) > 1e-9 {
		t.Fatalf("expected $100 approved from remaining $5 risk, got %.8f", got.ApprovedPositionUSD)
	}
}

func TestRiskEngineRespectsGrossExposure(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	s.CurrentGrossExposure = 1950
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskResize || math.Abs(got.ApprovedPositionUSD-50) > 1e-9 {
		t.Fatalf("expected exposure resize to $50, got %+v", got)
	}
}

func TestRiskEngineRespectsMarginBudget(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	s.CurrentMarginUsed = 290
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskResize || math.Abs(got.ApprovedPositionUSD-20) > 1e-9 {
		t.Fatalf("expected margin resize to $20, got %+v", got)
	}
}

func TestRiskEngineRejectsWrongSideStop(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	d.StopLoss = 101
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskReject || !strings.Contains(got.Reason, "below") {
		t.Fatalf("expected wrong-side stop rejection, got %+v", got)
	}
}

func TestRiskEngineRejectsInvalidNumbers(t *testing.T) {
	cfg, s, d := baseRiskFixture()
	s.Equity = math.NaN()
	got := EvaluateOpeningRisk(cfg, s, d, 100)
	if got.Verdict != RiskReject {
		t.Fatalf("expected invalid-number REJECT, got %+v", got)
	}
}
