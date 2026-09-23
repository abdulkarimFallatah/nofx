package store

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestShadowJournalAppendAndLatest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewShadowJournalStore(db)
	if err := s.initTables(); err != nil {
		t.Fatal(err)
	}

	entry := &ShadowJournalEntry{
		TraderID: "shadow-1", CycleNumber: 7, Symbol: "BTCUSDT", Action: "open_long",
		MarketPrice: 100, Equity: 1000, RequestedPositionUSD: 400, ApprovedPositionUSD: 200,
		Leverage: 2, StopLoss: 95, TakeProfit: 110, Confidence: 80,
		RiskVerdict: "RESIZE", RiskReason: "proposal exceeds deterministic risk budget",
		EstimatedRiskUSD: 10, MaxTradeRiskUSD: 10,
	}
	if err := s.Append(entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID == 0 {
		t.Fatal("expected persisted journal ID")
	}

	got, err := s.Latest("shadow-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 journal row, got %d", len(got))
	}
	if got[0].RequestedPositionUSD != 400 || got[0].ApprovedPositionUSD != 200 || got[0].RiskVerdict != "RESIZE" {
		t.Fatalf("unexpected journal row: %+v", got[0])
	}
}

func TestShadowJournalRejectsIncompleteEntry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewShadowJournalStore(db)
	if err := s.initTables(); err != nil { t.Fatal(err) }
	if err := s.Append(&ShadowJournalEntry{}); err == nil {
		t.Fatal("expected incomplete shadow entry to fail")
	}
}
