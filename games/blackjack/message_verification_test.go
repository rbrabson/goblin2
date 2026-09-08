package blackjack

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/disgoorg/disgo/discord"
)

func TestVerifyBlackjackFinalMessage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stale      int
		editFails  bool
		fetchFails bool
		attempts   int
		wantErr    bool
	}{
		{name: "verified", attempts: 1},
		{name: "stale readback repaired", stale: 1, attempts: 2},
		{name: "persistent mismatch", stale: 2, attempts: 2, wantErr: true},
		{name: "edit failure", editFails: true, attempts: 2, wantErr: true},
		{name: "readback failure", fetchFails: true, attempts: 2, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			final := &discord.Message{Embeds: []discord.Embed{{Title: "Blackjack", Description: "Game has ended."}}}
			update := discord.MessageUpdate{Content: new(""), Embeds: new(final.Embeds), Components: new([]discord.LayoutComponent{})}
			edits, reads := 0, 0
			edit := func(ctx context.Context) (*discord.Message, error) {
				edits++
				if _, ok := ctx.Deadline(); !ok {
					t.Error("missing request deadline")
				}
				if tc.editFails {
					return nil, errors.New("edit failed")
				}
				return final, nil
			}
			fetch := func(ctx context.Context) (*discord.Message, error) {
				reads++
				if tc.fetchFails {
					return nil, errors.New("fetch failed")
				}
				if reads <= tc.stale {
					return &discord.Message{Embeds: []discord.Embed{{Description: "10s remaining"}}}, nil
				}
				return final, nil
			}
			err := verifyBlackjackFinalMessage(update, edit, fetch, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
			if edits != tc.attempts {
				t.Fatalf("edits = %d, want %d", edits, tc.attempts)
			}
			if tc.editFails && reads != 0 {
				t.Fatal("fetched after failed edit")
			}
		})
	}
}

func TestBlackjackMessageSnapshot(t *testing.T) {
	a := &discord.Message{Embeds: []discord.Embed{{Title: "Blackjack", Type: discord.EmbedTypeRich}}}
	b := &discord.Message{Embeds: []discord.Embed{{Title: "Blackjack", Fields: []discord.EmbedField{}}}, Components: []discord.LayoutComponent{}}
	if blackjackMessageSnapshot(a) != blackjackMessageSnapshot(b) {
		t.Fatal("metadata or empty slices caused mismatch")
	}
	b.Components = []discord.LayoutComponent{discord.ActionRowComponent{}}
	if blackjackMessageSnapshot(a) == blackjackMessageSnapshot(b) {
		t.Fatal("remaining buttons not detected")
	}
	if blackjackMessageSnapshot(nil) == blackjackMessageSnapshot(a) {
		t.Fatal("nil response matched")
	}
}
