package store

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ShadowOutcome records a point-in-time mark-to-market result for a shadow
// proposal. It never represents an exchange fill or a real order.
type ShadowOutcome struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	JournalID           int64     `gorm:"not null;index:idx_shadow_outcome_journal_time" json:"journal_id"`
	TraderID            string    `gorm:"not null;index" json:"trader_id"`
	Symbol              string    `gorm:"not null;index" json:"symbol"`
	Timestamp           time.Time `gorm:"not null;index:idx_shadow_outcome_journal_time,sort:desc" json:"timestamp"`
	ObservedPrice       float64   `gorm:"not null" json:"observed_price"`
	PriceReturnRatio    float64   `gorm:"not null" json:"price_return_ratio"`
	RequestedPnLUSD     float64   `gorm:"not null" json:"requested_pnl_usd"`
	ApprovedPnLUSD      float64   `gorm:"not null" json:"approved_pnl_usd"`
	RiskDeltaPnLUSD     float64   `gorm:"not null" json:"risk_delta_pnl_usd"`
	CreatedAt           time.Time `json:"created_at"`
}

func (ShadowOutcome) TableName() string { return "shadow_outcomes" }

func (s *ShadowJournalStore) initOutcomeTables() error {
	if err := s.db.AutoMigrate(&ShadowOutcome{}); err != nil {
		return fmt.Errorf("failed to migrate shadow outcomes: %w", err)
	}
	return nil
}

// RecordOutcome marks one journal proposal to a trusted observed market price.
// PnL intentionally excludes fees, funding and slippage; those will be modeled
// separately so raw price movement remains auditable.
func (s *ShadowJournalStore) RecordOutcome(entry ShadowJournalEntry, observedPrice float64, observedAt time.Time) (*ShadowOutcome, error) {
	if entry.ID <= 0 || entry.MarketPrice <= 0 || observedPrice <= 0 ||
		math.IsNaN(observedPrice) || math.IsInf(observedPrice, 0) {
		return nil, fmt.Errorf("invalid shadow outcome input")
	}

	direction := 0.0
	switch strings.ToLower(entry.Action) {
	case "open_long":
		direction = 1
	case "open_short":
		direction = -1
	default:
		return nil, fmt.Errorf("unsupported shadow action %q", entry.Action)
	}

	priceReturn := direction * (observedPrice-entry.MarketPrice) / entry.MarketPrice
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	} else {
		observedAt = observedAt.UTC()
	}
	outcome := &ShadowOutcome{
		JournalID: entry.ID, TraderID: entry.TraderID, Symbol: entry.Symbol, Timestamp: observedAt,
		ObservedPrice: observedPrice, PriceReturnRatio: priceReturn,
		RequestedPnLUSD: entry.RequestedPositionUSD * priceReturn,
		ApprovedPnLUSD: entry.ApprovedPositionUSD * priceReturn,
	}
	outcome.RiskDeltaPnLUSD = outcome.ApprovedPnLUSD - outcome.RequestedPnLUSD

	if err := s.db.Create(outcome).Error; err != nil {
		return nil, fmt.Errorf("failed to record shadow outcome: %w", err)
	}
	return outcome, nil
}

func (s *ShadowJournalStore) Outcomes(journalID int64, limit int) ([]ShadowOutcome, error) {
	if journalID <= 0 {
		return nil, fmt.Errorf("invalid journal id")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var outcomes []ShadowOutcome
	if err := s.db.Where("journal_id = ?", journalID).Order("timestamp DESC").Limit(limit).Find(&outcomes).Error; err != nil {
		return nil, fmt.Errorf("failed to query shadow outcomes: %w", err)
	}
	return outcomes, nil
}
