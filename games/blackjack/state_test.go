package blackjack

import "testing"

func TestBlackjackStatusSettlingPayouts(t *testing.T) {
	game := &Game{state: SettlingPayouts}

	if got, want := blackjackStatus(game), "Settling payouts..."; got != want {
		t.Fatalf("settling-payouts status = %q, want %q", got, want)
	}
}
