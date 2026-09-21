package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
)

// applyV1OpeningRiskGate builds a risk snapshot from exchange/account state and
// applies the deterministic kernel risk engine immediately before an open order.
// It fails closed: malformed position state or unavailable safe capacity blocks
// new exposure rather than guessing.
func (at *AutoTrader) applyV1OpeningRiskGate(decision *kernel.Decision, positions []map[string]interface{}, equity, trustedEntryPrice float64) error {
	if decision == nil {
		return fmt.Errorf("V1 risk gate: nil decision")
	}
	if equity <= 0 || math.IsNaN(equity) || math.IsInf(equity, 0) {
		return fmt.Errorf("V1 risk gate: invalid account equity")
	}

	snapshot := kernel.RiskSnapshot{
		Equity:           equity,
		DailyPnL:         at.dailyPnL,
		DrawdownRatio:    0, // populated below only when the baseline is trustworthy
		CurrentMarginUsed: 0,
	}

	if at.initialBalance > 0 {
		snapshot.DrawdownRatio = currentAccountDrawdownRatio(equity, at.initialBalance)
	}

	for _, pos := range positions {
		mark, ok := finitePositiveMapFloat(pos, "markPrice")
		if !ok {
			return fmt.Errorf("V1 risk gate: invalid markPrice in existing position")
		}
		qty, ok := finiteMapFloat(pos, "positionAmt")
		if !ok {
			return fmt.Errorf("V1 risk gate: invalid positionAmt in existing position")
		}
		qty = math.Abs(qty)
		if qty == 0 {
			continue
		}
		lev, ok := finitePositiveMapFloat(pos, "leverage")
		if !ok {
			return fmt.Errorf("V1 risk gate: invalid leverage in existing position")
		}

		notional := qty * mark
		snapshot.CurrentGrossExposure += notional
		snapshot.CurrentMarginUsed += notional / lev

		// Existing protective-stop risk is not consistently exposed by every
		// exchange adapter. Until it is, reserve the full per-trade risk budget
		// for each existing position. This intentionally overestimates risk.
		snapshot.CurrentPortfolioRisk += equity * kernel.ConservativeRiskEngineConfig().MaxRiskPerTradeRatio
	}

	assessment := kernel.EvaluateOpeningRisk(kernel.ConservativeRiskEngineConfig(), snapshot, *decision, trustedEntryPrice)
	switch assessment.Verdict {
	case kernel.RiskPass:
		logger.Infof("  🛡️ [V1 RISK] PASS %s: risk %.2f / %.2f USDT", decision.Symbol, assessment.EstimatedRiskUSD, assessment.MaxTradeRiskUSD)
		return nil
	case kernel.RiskResize:
		if assessment.ApprovedPositionUSD <= 0 {
			return fmt.Errorf("V1 risk gate rejected %s: no safe resized position", decision.Symbol)
		}
		logger.Infof("  🛡️ [V1 RISK] RESIZE %s: %.2f → %.2f USDT (risk %.2f / %.2f)",
			decision.Symbol, decision.PositionSizeUSD, assessment.ApprovedPositionUSD,
			assessment.EstimatedRiskUSD, assessment.MaxTradeRiskUSD)
		decision.PositionSizeUSD = assessment.ApprovedPositionUSD
		return nil
	default:
		return fmt.Errorf("V1 risk gate rejected %s: %s", decision.Symbol, assessment.Reason)
	}
}

func currentAccountDrawdownRatio(equity, initialBalance float64) float64 {
	if equity <= 0 || initialBalance <= 0 || equity >= initialBalance {
		return 0
	}
	return (initialBalance - equity) / initialBalance
}

func finiteMapFloat(m map[string]interface{}, key string) (float64, bool) {
	switch v := m[key].(type) {
	case float64:
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case float32:
		f := float64(v)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func finitePositiveMapFloat(m map[string]interface{}, key string) (float64, bool) {
	v, ok := finiteMapFloat(m, key)
	return v, ok && v > 0
}
