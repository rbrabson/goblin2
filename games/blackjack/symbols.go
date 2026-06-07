package blackjack

import (
	"fmt"
	"goblin2/internal/config"
	"maps"
	"path/filepath"
	"strings"

	bj "github.com/rbrabson/blackjack"
)

var (
	symbols Symbols
)

type Symbols map[string]map[string]string

// GetSymbols returns the cards for the blackjack game.
func GetSymbols() Symbols {
	symbols := make(Symbols)
	maps.Copy(symbols, symbols)
	return symbols
}

// GetHand returns a string representation of the hand using the provided symbols.
func (s Symbols) GetHand(hand *bj.Hand, hidden bool) string {
	cards := make([]string, 0, len(hand.Cards()))
	var sb strings.Builder
	for idx, card := range hand.Cards() {
		if hidden && idx == 0 {
			cards = append(cards, s["Cards"]["Back"])
		} else {
			card := s[card.Suit.String()][card.Rank.String()]
			cards = append(cards, card)
		}
	}
	sb.WriteString(strings.Join(cards, ""))
	sb.WriteString(fmt.Sprintf(" (value: %s)", GetHandValue(hand, hidden)))

	return sb.String()
}

// GetHandWithoutValue returns a string representation of the hand using the provided symbols.
func (s Symbols) GetHandWithoutValue(hand *bj.Hand, hidden bool) string {
	cards := make([]string, 0, len(hand.Cards()))
	var sb strings.Builder
	for idx, card := range hand.Cards() {
		if hidden && idx == 0 {
			cards = append(cards, s["Cards"]["Back"])
		} else {
			card := s[card.Suit.String()][card.Rank.String()]
			cards = append(cards, card)
		}
	}
	sb.WriteString(strings.Join(cards, ""))

	return sb.String()
}

// GetHandValue returns a string representation of the hand value using the provided symbols.
func GetHandValue(hand *bj.Hand, hidden bool) string {
	switch {
	case hand.IsBlackjack():
		return " (blackjack)"
	case hand.IsBusted():
		return fmt.Sprintf("%d, busted", handValue(hand, hidden))
	case hand.IsSurrendered():
		return fmt.Sprintf("%d, surrendered", handValue(hand, hidden))
	default:
		return fmt.Sprintf("%d", handValue(hand, hidden))
	}
}

// LoadCards loads the cards from the specified YAML file path.
func LoadCards(path string, deckType string) error {
	filePath := filepath.Join(path, "blackjack/cards.yaml")
	var symbolMap map[string]Symbols
	if err := config.LoadConfig(filePath, &symbolMap); err != nil {
		return err
	}
	var ok bool
	symbols, ok = symbolMap[deckType]
	if !ok {
		return fmt.Errorf("missing cards for deck type: %s", deckType)
	}

	return nil
}
