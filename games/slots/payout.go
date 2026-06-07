package slots

import (
	"goblin2/internal/config"
	"path/filepath"
	"slices"

	rslots "github.com/rbrabson/slots"
)

var (
	defaultPayoutTable rslots.PayoutTable
)

// GetPayoutTable retrieves the payout table for a specific guild.
func GetPayoutTable() rslots.PayoutTable {
	pt := newPayoutTable()
	slices.SortFunc(pt, func(a, b rslots.PayoutAmount) int {
		if a.Bet != b.Bet {
			return a.Bet - b.Bet
		}
		comparison := b.Payout - a.Payout
		if comparison < 0 {
			return -1
		}
		if comparison > 0 {
			return 1
		}
		return 0

	})
	return pt
}

// newPayoutTable creates a copy of the default payout table.
func newPayoutTable() rslots.PayoutTable {
	payoutTable := make(rslots.PayoutTable, len(defaultPayoutTable))
	copy(payoutTable, defaultPayoutTable)

	return payoutTable
}

// LoadPayoutTable loads the payout table from a file.
func LoadPayoutTable(path string) error {
	filePath := filepath.Join(path, "slots/payout.yaml")
	if err := config.LoadConfig(filePath, &defaultPayoutTable); err != nil {
		return err
	}

	return nil
}
