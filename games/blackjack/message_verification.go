package blackjack

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
)

type blackjackMessageCall func(context.Context) (*discord.Message, error)

// Compare only the visible fields we author. Discord can add embed metadata,
// and nil and empty component/field slices represent the same display.
func blackjackMessageSnapshot(message *discord.Message) string {
	if message == nil {
		return "null"
	}
	embeds := make([]discord.Embed, 0, len(message.Embeds))
	for _, embed := range message.Embeds {
		embeds = append(embeds, discord.Embed{
			Title: embed.Title, Description: embed.Description,
			Color: embed.Color, Fields: embed.Fields,
		})
	}
	snapshot, _ := json.Marshal(struct {
		Content       string          `json:"content"`
		Embeds        []discord.Embed `json:"embeds"`
		ComponentRows int             `json:"componentRows"`
	}{message.Content, embeds, len(message.Components)})
	return string(snapshot)
}

// The caller holds messageLock throughout verification, so a retry cannot
// overtake another edit or reuse a subsequent round's mutable game state.
func verifyBlackjackFinalMessage(update discord.MessageUpdate, edit, fetch blackjackMessageCall, logger *slog.Logger) error {
	want := blackjackMessageSnapshot(&discord.Message{Content: *update.Content, Embeds: *update.Embeds, Components: *update.Components})
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		// Bound the entire edit + readback, including rate limiter waits.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		response, err := edit(ctx)
		if err == nil {
			logger.Info("blackjack final edit response", "attempt", attempt,
				"matches", blackjackMessageSnapshot(response) == want,
				"expected", want, "actual", blackjackMessageSnapshot(response))
			stored, fetchErr := fetch(ctx)
			if fetchErr != nil {
				err = fmt.Errorf("fetch final blackjack message: %w", fetchErr)
			} else {
				actual := blackjackMessageSnapshot(stored)
				logger.Info("blackjack final message readback", "attempt", attempt,
					"matches", actual == want, "expected", want, "actual", actual)
				if actual != want {
					err = fmt.Errorf("stored blackjack message does not match completed round")
				}
			}
		}
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		logger.Warn("blackjack final message verification failed", "attempt", attempt, "error", err)
	}
	return fmt.Errorf("final blackjack message unverified after two attempts: %w", lastErr)
}
