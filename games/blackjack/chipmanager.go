package blackjack

import (
	"goblin2/bank"
	"goblin2/internal/discordid"
	"log/slog"
)

// ChipManager manages the chips for a blackjack player using a bank account.
type ChipManager struct {
	game           *Game
	memberID       discordid.SnowflakeID
	reservedCredit int
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
	if c.reservedCredit == amount {
		c.reservedCredit = 0
		return
	}
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

func (c *ChipManager) reserveCredit(amount int) error {
	// A credit reservation is represented by a successful deposit. The next
	// matching AddChips call from the dependency is then consumed as a no-op.
	if err := bank.GetAccount(c.game.guildID, c.memberID).Deposit(amount); err != nil {
		return err
	}
	c.reservedCredit = amount
	return nil
}

func (c *ChipManager) cancelCredit(amount int) error {
	if c.reservedCredit == amount {
		c.reservedCredit = 0
		return bank.GetAccount(c.game.guildID, c.memberID).Withdraw(amount)
	}
	return nil
}

// HasEnoughChips checks if the player has enough chips for the specified amount.
func (c *ChipManager) HasEnoughChips(amount int) bool {
	return c.GetChips() >= amount
}
