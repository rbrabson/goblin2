package blackjack

import (
	"goblin2/bank"
	"goblin2/discordid"
	"log/slog"
)

// ChipManager manages the chips for a blackjack player using a bank account.
type ChipManager struct {
	game     *Game
	memberID discordid.SnowflakeID
}

// NewChipManager returns a new ChipManager for the given guild and member.
func NewChipManager(game *Game, memberID discordid.SnowflakeID) *ChipManager {
	return &ChipManager{
		game:     game,
		memberID: memberID,
	}
}

// GetChips returns the current number of chips the player has.
func (c *ChipManager) GetChips() int {
	account := bank.GetAccount(c.game.guildID, c.memberID)
	return account.GetBalance()
}

// SetChips sets the number of chips the player has.
// This is a no-op since chips are managed via the bank account.
func (c *ChipManager) SetChips(_ int) {
	// NO-OP
}

// AddChips adds the specified number of chips to the player's account.
func (c *ChipManager) AddChips(amount int) {
	game := c.game
	amount = amount * game.config.PayoutPercent / 100
	if amount == 0 {
		slog.Warn("attempted to add zero blackjack chips to account",
			slog.Any("guildID", c.game.guildID),
			slog.Any("memberID", c.memberID),
		)
		return
	}
	account := bank.GetAccount(c.game.guildID, c.memberID)
	if err := account.Deposit(amount); err != nil {
		slog.Error("failed to add chips to account",
			slog.Any("guildID", c.game.guildID),
			slog.Any("memberID", c.memberID),
			slog.Int("amount", amount),
			slog.Any("error", err))
		return
	}
	slog.Debug("added blackjack chips to account",
		slog.Any("guildID", c.game.guildID),
		slog.Any("memberID", c.memberID),
		slog.Int("amount", amount),
	)
}

// DeductChips deducts the specified number of chips from the player's account.
func (c *ChipManager) DeductChips(amount int) error {
	account := bank.GetAccount(c.game.guildID, c.memberID)
	if err := account.Withdraw(amount); err != nil {
		slog.Error("failed to deduct chips from account",
			slog.Any("guildID", c.game.guildID),
			slog.Any("memberID", c.memberID),
			slog.Int("amount", amount),
			slog.Any("error", err))
		return err
	}
	slog.Debug("deducted blackjack chips from account",
		slog.Any("guildID", c.game.guildID),
		slog.Any("memberID", c.memberID),
		slog.Int("amount", amount),
	)
	return nil
}

// HasEnoughChips checks if the player has enough chips for the specified amount.
func (c *ChipManager) HasEnoughChips(amount int) bool {
	return c.GetChips() >= amount
}
