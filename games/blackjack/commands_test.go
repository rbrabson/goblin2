package blackjack

import "testing"

func TestBlackjackStatusDealerTurn(t *testing.T) {
	game := &Game{state: DealerTurn}

	if got, want := blackjackStatus(game), "Dealer's turn."; got != want {
		t.Fatalf("dealer-turn status = %q, want %q", got, want)
	}
}
