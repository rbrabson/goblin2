package blackjack

import (
	"testing"

	bj "github.com/rbrabson/blackjack"
	"github.com/rbrabson/cards"
)

func TestSymbolsHideDealerHoleCard(t *testing.T) {
	symbols := Symbols{
		Cards:    Cards{Back: "[back]"},
		Hearts:   Suit{Ten: "[10h]"},
		Diamonds: Suit{Queen: "[qd]"},
	}
	hand := bj.NewDealerHand()
	hand.DealCard(cards.Card{Rank: cards.Ten, Suit: cards.Hearts})
	hand.DealCard(cards.Card{Rank: cards.Queen, Suit: cards.Diamonds})

	if got, want := symbols.GetHand(hand, true), "[10h][back] (10)"; got != want {
		t.Fatalf("hidden dealer hand = %q, want %q", got, want)
	}
	if got, want := symbols.GetHand(hand, false), "[10h][qd] (20)"; got != want {
		t.Fatalf("revealed dealer hand = %q, want %q", got, want)
	}
}
