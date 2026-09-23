package store

import (
	"math"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newShadowOutcomeTestStore(t *testing.T) *ShadowJournalStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewShadowJournalStore(db)
	if err := s.initTables(); err != nil {
		t.Fatal(err)
	}
	if err := s.initOutcomeTables(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestShadowOutcomeLongComparesRequestedAndApproved(t *testing.T) {
	s := newShadowOutcomeTestStore(t)
	entry := &ShadowJournalEntry{
		TraderID: "t1", CycleNumber: 1, Symbol: "BTCUSDT", Action: "open_long",
		MarketPrice: 100, Equity: 1000, RequestedPositionUSD: 1000, ApprovedPositionUSD: 250,
		Leverage: 2, RiskVerdict: "RESIZE", RiskReason: "risk cap",
	}
	if err := s.Append(entry); err != nil {
		t.Fatal(err)
	}
	out, err := s.RecordOutcome(*entry, 90, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(out.RequestedPnLUSD-(-100)) > 1e-9 || math.Abs(out.ApprovedPnLUSD-(-25)) > 1e-9 {
		t.Fatalf("unexpected pnl: %+v", out)
	}
	if math.Abs(out.RiskDeltaPnLUSD-75) > 1e-9 {
		t.Fatalf("expected V1 to avoid 75 USD of loss, got %.4f", out.RiskDeltaPnLUSD)
	}
}

func TestShadowOutcomeShortDirection(t *testing.T) {
	s := newShadowOutcomeTestStore(t)
	entry := &ShadowJournalEntry{
		TraderID: "t1", CycleNumber: 2, Symbol: "ETHUSDT", Action: "open_short",
		MarketPrice: 100, Equity: 1000, RequestedPositionUSD: 500, ApprovedPositionUSD: 200,
		Leverage: 2, RiskVerdict: "RESIZE", RiskReason: "risk cap",
	}
	if err := s.Append(entry); err != nil {
		t.Fatal(err)
	}
	out, err := s.RecordOutcome(*entry, 90, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(out.RequestedPnLUSD-50) > 1e-9 || math.Abs(out.ApprovedPnLUSD-20) > 1e-9 {
		t.Fatalf("unexpected short pnl: %+v", out)
	}
	if math.Abs(out.RiskDeltaPnLUSD-(-30)) > 1e-9 {
		t.Fatalf("expected 30 USD opportunity reduction, got %.4f", out.RiskDeltaPnLUSD)
	}
}

func TestShadowOutcomeRejectsUnsupportedAction(t *testing.T) {
	s := newShadowOutcomeTestStore(t)
	entry := ShadowJournalEntry{ID: 1, TraderID: "t1", Symbol: "BTCUSDT", Action: "hold", MarketPrice: 100}
	if _, err := s.RecordOutcome(entry, 101, time.Time{}); err == nil {
		t.Fatal("expected unsupported action to fail")
	}
}
