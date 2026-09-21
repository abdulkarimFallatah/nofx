package kernel

import (
	"fmt"
	"math"
	"strings"
)

// RiskVerdict is the deterministic result of evaluating an AI-proposed opening trade.
type RiskVerdict string

const (
	RiskPass   RiskVerdict = "PASS"
	RiskResize RiskVerdict = "RESIZE"
	RiskReject RiskVerdict = "REJECT"
)

// RiskEngineConfig contains portfolio rules that the AI cannot override.
// Ratios are expressed as fractions of equity (0.01 = 1%).
type RiskEngineConfig struct {
	MaxRiskPerTradeRatio  float64
	MaxPortfolioRiskRatio float64
	MaxGrossExposureRatio float64
	MaxMarginUsageRatio   float64
	MaxDailyLossRatio     float64
	MaxDrawdownRatio      float64
	MinStopDistanceRatio  float64
}

// ConservativeRiskEngineConfig is intentionally defensive for V1.
// It can later be made strategy-configurable after shadow-mode validation.
func ConservativeRiskEngineConfig() RiskEngineConfig {
	return RiskEngineConfig{
		MaxRiskPerTradeRatio:  0.01,
		MaxPortfolioRiskRatio: 0.04,
		MaxGrossExposureRatio: 2.00,
		MaxMarginUsageRatio:   0.30,
		MaxDailyLossRatio:     0.03,
		MaxDrawdownRatio:      0.10,
		MinStopDistanceRatio:  0.001,
	}
}

// RiskSnapshot contains deterministic account state at decision time.
// DailyPnL is negative when the account has lost money today.
// DrawdownRatio is positive (0.05 means 5% below the tracked equity peak).
type RiskSnapshot struct {
	Equity              float64
	DailyPnL             float64
	DrawdownRatio        float64
	CurrentPortfolioRisk float64
	CurrentGrossExposure float64
	CurrentMarginUsed    float64
}

// RiskAssessment records why a proposal passed, was resized, or was rejected.
type RiskAssessment struct {
	Verdict              RiskVerdict
	Reason               string
	RequestedPositionUSD float64
	ApprovedPositionUSD  float64
	EstimatedRiskUSD     float64
	MaxTradeRiskUSD      float64
}

// EvaluateOpeningRisk independently evaluates an AI-proposed opening trade.
// entryPrice must come from trusted market/exchange data, never from model output.
func EvaluateOpeningRisk(cfg RiskEngineConfig, snapshot RiskSnapshot, d Decision, entryPrice float64) RiskAssessment {
	out := RiskAssessment{Verdict: RiskReject, RequestedPositionUSD: d.PositionSizeUSD}

	if err := validateRiskInputs(cfg, snapshot, d, entryPrice); err != nil {
		out.Reason = err.Error()
		return out
	}

	if d.Action != "open_long" && d.Action != "open_short" {
		out.Reason = "risk engine only evaluates opening trades"
		return out
	}

	maxTradeRisk := snapshot.Equity * cfg.MaxRiskPerTradeRatio
	out.MaxTradeRiskUSD = maxTradeRisk

	if snapshot.DailyPnL <= -(snapshot.Equity * cfg.MaxDailyLossRatio) {
		out.Reason = "daily loss circuit breaker reached"
		return out
	}
	if snapshot.DrawdownRatio >= cfg.MaxDrawdownRatio {
		out.Reason = "maximum drawdown circuit breaker reached"
		return out
	}

	stopDistance := math.Abs(entryPrice-d.StopLoss) / entryPrice
	if stopDistance < cfg.MinStopDistanceRatio {
		out.Reason = "protective stop is too close to entry"
		return out
	}

	// Risk is notional position value multiplied by the price distance to stop.
	// Leverage affects required margin, not the price-loss amount at the stop.
	requestedRisk := d.PositionSizeUSD * stopDistance
	portfolioRiskCapacity := snapshot.Equity*cfg.MaxPortfolioRiskRatio - snapshot.CurrentPortfolioRisk
	exposureCapacity := snapshot.Equity*cfg.MaxGrossExposureRatio - snapshot.CurrentGrossExposure
	marginCapacity := snapshot.Equity*cfg.MaxMarginUsageRatio - snapshot.CurrentMarginUsed

	if portfolioRiskCapacity <= 0 {
		out.Reason = "portfolio risk budget exhausted"
		return out
	}
	if exposureCapacity <= 0 {
		out.Reason = "gross exposure budget exhausted"
		return out
	}
	if marginCapacity <= 0 {
		out.Reason = "margin budget exhausted"
		return out
	}

	approved := d.PositionSizeUSD
	approved = math.Min(approved, maxTradeRisk/stopDistance)
	approved = math.Min(approved, portfolioRiskCapacity/stopDistance)
	approved = math.Min(approved, exposureCapacity)
	approved = math.Min(approved, marginCapacity*float64(d.Leverage))

	if approved <= 0 || math.IsNaN(approved) || math.IsInf(approved, 0) {
		out.Reason = "no safe position size available"
		return out
	}

	out.ApprovedPositionUSD = approved
	out.EstimatedRiskUSD = approved * stopDistance
	if approved+1e-9 < d.PositionSizeUSD {
		out.Verdict = RiskResize
		out.Reason = "proposal exceeds deterministic risk budget"
		return out
	}

	out.Verdict = RiskPass
	out.Reason = "proposal is within deterministic risk budget"
	out.EstimatedRiskUSD = requestedRisk
	return out
}

func validateRiskInputs(cfg RiskEngineConfig, s RiskSnapshot, d Decision, entryPrice float64) error {
	finitePositive := func(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
	finiteNonNegative := func(v float64) bool { return v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }

	if !finitePositive(s.Equity) || !finitePositive(entryPrice) {
		return fmt.Errorf("equity and trusted entry price must be finite positive values")
	}
	if !finitePositive(d.PositionSizeUSD) || d.Leverage <= 0 || !finitePositive(d.StopLoss) {
		return fmt.Errorf("opening proposal has invalid size, leverage, or stop")
	}
	if !finiteNonNegative(s.DrawdownRatio) || !finiteNonNegative(s.CurrentPortfolioRisk) ||
		!finiteNonNegative(s.CurrentGrossExposure) || !finiteNonNegative(s.CurrentMarginUsed) ||
		math.IsNaN(s.DailyPnL) || math.IsInf(s.DailyPnL, 0) {
		return fmt.Errorf("risk snapshot contains invalid values")
	}
	vals := []float64{cfg.MaxRiskPerTradeRatio, cfg.MaxPortfolioRiskRatio, cfg.MaxGrossExposureRatio,
		cfg.MaxMarginUsageRatio, cfg.MaxDailyLossRatio, cfg.MaxDrawdownRatio, cfg.MinStopDistanceRatio}
	for _, v := range vals {
		if !finitePositive(v) {
			return fmt.Errorf("risk configuration must contain finite positive limits")
		}
	}
	if strings.TrimSpace(d.Symbol) == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	if d.Action == "open_long" && d.StopLoss >= entryPrice {
		return fmt.Errorf("long protective stop must be below trusted entry price")
	}
	if d.Action == "open_short" && d.StopLoss <= entryPrice {
		return fmt.Errorf("short protective stop must be above trusted entry price")
	}
	return nil
}
