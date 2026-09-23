package trader

import (
	"fmt"
	"nofx/market"
	"time"
)

const shadowObservationLimit = 100

// observeShadowOutcomes marks existing shadow openings to fresh trusted market
// prices. It is observational only: it never sends an order or mutates exchange
// state. A failure is returned so missing research data cannot look successful.
func (at *AutoTrader) observeShadowOutcomes() error {
	if at == nil || !at.config.ShadowMode {
		return nil
	}
	if at.store == nil {
		return fmt.Errorf("shadow outcome observation requires persistent store")
	}

	entries, err := at.store.ShadowJournal().ActiveOpenings(at.id, shadowObservationLimit)
	if err != nil {
		return fmt.Errorf("load shadow openings: %w", err)
	}

	now := time.Now().UTC()
	for _, entry := range entries {
		data, err := market.GetWithExchange(entry.Symbol, at.exchange)
		if err != nil {
			return fmt.Errorf("observe %s for shadow journal %d: %w", entry.Symbol, entry.ID, err)
		}
		if _, err := at.store.ShadowJournal().RecordOutcome(entry, data.CurrentPrice, now); err != nil {
			return fmt.Errorf("record outcome for shadow journal %d: %w", entry.ID, err)
		}
	}
	return nil
}
