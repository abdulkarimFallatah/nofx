package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ShadowJournalEntry is an immutable observation of what the AI proposed and
// what the deterministic V1 risk engine allowed. It is research data only.
type ShadowJournalEntry struct {
	ID                   int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TraderID             string    `gorm:"column:trader_id;not null;index:idx_shadow_trader_time" json:"trader_id"`
	CycleNumber          int       `gorm:"column:cycle_number;not null" json:"cycle_number"`
	Timestamp            time.Time `gorm:"not null;index:idx_shadow_trader_time,sort:desc" json:"timestamp"`
	Symbol               string    `gorm:"not null;index" json:"symbol"`
	Action               string    `gorm:"not null" json:"action"`
	MarketPrice          float64   `gorm:"not null" json:"market_price"`
	Equity               float64   `gorm:"not null" json:"equity"`
	RequestedPositionUSD float64   `gorm:"not null" json:"requested_position_usd"`
	ApprovedPositionUSD  float64   `gorm:"not null" json:"approved_position_usd"`
	Leverage             int       `gorm:"not null" json:"leverage"`
	StopLoss             float64   `json:"stop_loss"`
	TakeProfit           float64   `json:"take_profit"`
	Confidence           int       `json:"confidence"`
	RiskVerdict          string    `gorm:"not null" json:"risk_verdict"`
	RiskReason           string    `gorm:"not null" json:"risk_reason"`
	EstimatedRiskUSD     float64   `gorm:"not null" json:"estimated_risk_usd"`
	MaxTradeRiskUSD      float64   `gorm:"not null" json:"max_trade_risk_usd"`
	CreatedAt            time.Time `json:"created_at"`
}

func (ShadowJournalEntry) TableName() string { return "shadow_journal" }

type ShadowJournalStore struct{ db *gorm.DB }

func NewShadowJournalStore(db *gorm.DB) *ShadowJournalStore { return &ShadowJournalStore{db: db} }

func (s *ShadowJournalStore) initTables() error {
	if err := s.db.AutoMigrate(&ShadowJournalEntry{}); err != nil {
		return fmt.Errorf("failed to migrate shadow journal: %w", err)
	}
	return nil
}

func (s *ShadowJournalStore) Append(entry *ShadowJournalEntry) error {
	if entry == nil {
		return fmt.Errorf("shadow journal entry cannot be nil")
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	} else {
		entry.Timestamp = entry.Timestamp.UTC()
	}
	if entry.TraderID == "" || entry.Symbol == "" || entry.Action == "" || entry.RiskVerdict == "" {
		return fmt.Errorf("shadow journal entry missing required identity fields")
	}
	if err := s.db.Create(entry).Error; err != nil {
		return fmt.Errorf("failed to append shadow journal entry: %w", err)
	}
	return nil
}

func (s *ShadowJournalStore) ActiveOpenings(traderID string, limit int) ([]ShadowJournalEntry, error) {
	if traderID == "" {
		return nil, fmt.Errorf("trader id cannot be empty")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var entries []ShadowJournalEntry
	if err := s.db.Where("trader_id = ? AND action IN ? AND approved_position_usd > 0", traderID, []string{"open_long", "open_short"}).
		Order("timestamp DESC").Limit(limit).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to query active shadow openings: %w", err)
	}
	return entries, nil
}

func (s *ShadowJournalStore) Latest(traderID string, limit int) ([]ShadowJournalEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var entries []ShadowJournalEntry
	if err := s.db.Where("trader_id = ?", traderID).Order("timestamp DESC").Limit(limit).Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to query shadow journal: %w", err)
	}
	return entries, nil
}
