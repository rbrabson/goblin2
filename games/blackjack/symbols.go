package blackjack

import (
	"fmt"
	"goblin2/internal/config"
	"path/filepath"
	"strings"

	bj "github.com/rbrabson/blackjack"
	"github.com/rbrabson/cards"
)

var (
	symbols Symbols
)

type Symbols struct {
	Cards    Cards `yaml:"Cards"`
	Clubs    Suit  `yaml:"Clubs"`
	Diamonds Suit  `yaml:"Diamonds"`
	Hearts   Suit  `yaml:"Hearts"`
	Spades   Suit  `yaml:"Spades"`
}
type Cards struct {
	Back     string `yaml:"Back"`
	Multiple string `yaml:"Multiple"`
}
type Suit struct {
	Ace   string `yaml:"Ace"`
	Eight string `yaml:"Eight"`
	Five  string `yaml:"Five"`
	Four  string `yaml:"Four"`
	Jack  string `yaml:"Jack"`
	King  string `yaml:"King"`
	Nine  string `yaml:"Nine"`
	Queen string `yaml:"Queen"`
	Seven string `yaml:"Seven"`
	Six   string `yaml:"Six"`
	Ten   string `yaml:"Ten"`
	Three string `yaml:"Three"`
	Two   string `yaml:"Two"`
}

// GetSymbols returns the cards for the blackjack game.
func GetSymbols() *Symbols {
	return &symbols
}

// GetHand returns a string representation of the hand using the provided symbols.
func (s *Symbols) GetHand(hand *bj.Hand, hidden bool) string {
	cardsInHand := make([]string, 0, len(hand.Cards()))
	var sb strings.Builder
	for idx, card := range hand.Cards() {
		if hidden && idx == 0 {
			cardsInHand = append(cardsInHand, s.Cards.Back)
		} else {
			switch card.Suit {
			case cards.Clubs:
				switch card.Rank {
				case cards.Ace:
					cardsInHand = append(cardsInHand, s.Clubs.Ace)
				}
			case cards.Diamonds:
			case cards.Hearts:
			case cards.Spades:
			}
		}
	}
	sb.WriteString(strings.Join(cardsInHand, ""))
	sb.WriteString(fmt.Sprintf(" (value: %s)", GetHandValue(hand, hidden)))

	return sb.String()
}

func (s *Symbols) getCard(card cards.Card) string {
	switch card.Suit {
	case cards.Clubs:
		return s.getCardInSuit(s.Clubs, card.Rank)
	case cards.Diamonds:
		return s.getCardInSuit(s.Diamonds, card.Rank)
	case cards.Hearts:
		return s.getCardInSuit(s.Hearts, card.Rank)
	case cards.Spades:
		return s.getCardInSuit(s.Spades, card.Rank)
	default:
		return "<!-- unknown suit -->"
	}
}

func (s *Symbols) getCardInSuit(suit Suit, rank cards.Rank) string {
	switch rank {
	case cards.Ace:
		return suit.Ace
	case cards.Two:
		return suit.Two
	case cards.Three:
		return suit.Three
	case cards.Four:
		return suit.Four
	case cards.Five:
		return suit.Five
	case cards.Six:
		return suit.Six
	case cards.Seven:
		return suit.Seven
	case cards.Eight:
		return suit.Eight
	case cards.Nine:
		return suit.Nine
	case cards.Ten:
		return suit.Ten
	case cards.Jack:
		return suit.Jack
	case cards.Queen:
		return suit.Queen
	case cards.King:
		return suit.King
	default:
		return "<!-- unknown rank -->"
	}
}

// GetHandWithoutValue returns a string representation of the hand using the provided symbols.
func (s *Symbols) GetHandWithoutValue(hand *bj.Hand, hidden bool) string {
	cards := make([]string, 0, len(hand.Cards()))
	var sb strings.Builder
	for idx, card := range hand.Cards() {
		if hidden && idx == 0 {
			cards = append(cards, s.Cards.Back)
		} else {
			cards = append(cards, s.getCard(card))
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
func LoadCards(path string) error {
	filePath := filepath.Join(path, "blackjack/cards.yaml")
	if err := config.LoadConfig(filePath, &symbols); err != nil {
		return err
	}

	return nil
}
