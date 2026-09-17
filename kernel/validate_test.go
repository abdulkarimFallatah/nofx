package kernel

import (
	"math"
	"strings"
	"testing"
)

// TestLeverageValidation verifies that model output cannot be silently altered
// into an executable decision. Invalid leverage must fail closed.
func TestLeverageValidation(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcEthLeverage  int
		altcoinLeverage int
		wantError       bool
	}{
		{
			name: "Altcoin leverage exceeded - reject",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        20, // Exceeds limit
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
				Reasoning:       "synthetic leverage-limit fixture",
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5, // Limit 5x
			wantError:       true,
		},
		{
			name: "BTC leverage exceeded - reject",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        20, // Exceeds limit
				PositionSizeUSD: 1000,
				StopLoss:        90000,
				TakeProfit:      110000,
				Reasoning:       "synthetic leverage-limit fixture",
			},
			accountEquity:   100,
			btcEthLeverage:  10, // Limit 10x
			altcoinLeverage: 5,
			wantError:       true,
		},
		{
			name: "Leverage within limit - no correction",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "open_short",
				Leverage:        5, // Not exceeded
				PositionSizeUSD: 500,
				StopLoss:        4000,
				TakeProfit:      3000,
				Reasoning:       "synthetic valid fixture",
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantError:       false,
		},
		{
			name: "Leverage is 0 - should error",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        0, // Invalid
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
				Reasoning:       "synthetic zero-leverage fixture",
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use default position value ratios for testing (10x for BTC/ETH, 1.5x for altcoins)
			err := validateDecision(&tt.decision, tt.accountEquity, tt.btcEthLeverage, tt.altcoinLeverage, 10.0, 1.5)

			// Check error status
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

		})
	}
}

func TestDecisionValidationRejectsUnsafeValues(t *testing.T) {
	base := Decision{
		Symbol:          "TEST",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 20,
		StopLoss:        80,
		TakeProfit:      180,
		Confidence:      80,
		Reasoning:       "synthetic validation fixture",
	}

	tests := []struct {
		name   string
		mutate func(*Decision)
		match  string
	}{
		{"NaN position size", func(d *Decision) { d.PositionSizeUSD = math.NaN() }, "must be finite"},
		{"infinite stop", func(d *Decision) { d.StopLoss = math.Inf(1) }, "must be finite"},
		{"empty symbol", func(d *Decision) { d.Symbol = " " }, "symbol cannot be empty"},
		{"missing reasoning", func(d *Decision) { d.Reasoning = "" }, "reasoning cannot be empty"},
		{"confidence above range", func(d *Decision) { d.Confidence = 101 }, "confidence"},
		{"negative risk", func(d *Decision) { d.RiskUSD = -1 }, "risk_usd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := base
			tt.mutate(&decision)
			err := validateDecision(&decision, 100, 5, 5, 1, 1)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("expected error containing %q, got %v", tt.match, err)
			}
		})
	}
}

func TestDecodeDecisionsStrict(t *testing.T) {
	valid := `[{"symbol":"TEST","action":"wait","reasoning":"synthetic fixture"}]`
	if _, err := decodeDecisionsStrict(valid); err != nil {
		t.Fatalf("valid decision should decode: %v", err)
	}

	tests := []struct {
		name  string
		input string
		match string
	}{
		{"unknown field", `[{"symbol":"TEST","action":"wait","reasoning":"fixture","leverge":5}]`, "unknown field"},
		{"empty array", `[]`, "cannot be empty"},
		{"trailing value", valid + ` {}`, "trailing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeDecisionsStrict(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("expected error containing %q, got %v", tt.match, err)
			}
		})
	}
}

func TestClaw402XyzAllowsFullTenXNotional(t *testing.T) {
	decision := Decision{
		Symbol:          "xyz:SP500",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 306.8,
		StopLoss:        95,
		TakeProfit:      120,
		Reasoning:       "synthetic asset-tier fixture",
	}

	if err := validateDecision(&decision, 30.68, 10, 10, 10.0, 10.0); err != nil {
		t.Fatalf("xyz TradeFi Claw402 full 10x notional should pass validation: %v", err)
	}
}

func TestSignalManagedDecisionAllowsZeroTakeProfitWithProtectiveStop(t *testing.T) {
	decision := Decision{
		Symbol:          "BTC",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 300,
		StopLoss:        95000,
		TakeProfit:      0,
		Reasoning:       "synthetic managed-exit fixture",
	}

	if err := validateDecisionForMode(&decision, 30, 10, 10, 10.0, 10.0, true); err != nil {
		t.Fatalf("signal-managed open with a protective stop should pass validation: %v", err)
	}
}

func TestSignalManagedDecisionStillRequiresProtectiveStop(t *testing.T) {
	decision := Decision{
		Symbol:          "BTC",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 300,
		StopLoss:        0,
		TakeProfit:      0,
		Reasoning:       "synthetic missing-stop fixture",
	}

	if err := validateDecisionForMode(&decision, 30, 10, 10, 10.0, 10.0, true); err == nil {
		t.Fatal("signal-managed open without a protective stop should fail validation")
	}
}

func TestFixedExitDecisionStillRequiresTakeProfit(t *testing.T) {
	decision := Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 300,
		StopLoss:        95000,
		TakeProfit:      0,
		Reasoning:       "synthetic missing-target fixture",
	}

	if err := validateDecision(&decision, 30, 10, 10, 10.0, 10.0); err == nil {
		t.Fatal("ordinary fixed-exit strategy should still reject zero take profit")
	}
}

// contains checks if string contains substring (helper function)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
