package slots

import (
	"goblin2/config"
	"path/filepath"
	"slices"
	"strings"
)

var (
	defaultSymbols SymbolTable
)

// Symbol represents a slot symbol with a name and an emoji.
type Symbol struct {
	Name  string `yaml:"name" bson:"name"`
	Emoji string `yaml:"emoji" bson:"emoji"`
}

// String returns a string representation of the Symbol.
func (s *Symbol) String() string {
	sb := strings.Builder{}
	sb.WriteString("Symbol{")
	sb.WriteString("Name: " + s.Name)
	sb.WriteString(", Emoji: " + s.Emoji)
	sb.WriteString("}")

	return sb.String()
}

// SymbolTable defines a table of symbols for a specific guild.
type SymbolTable map[string]Symbol

// GetSymbolTable retrieves the symbol table for a specific guild.
func GetSymbolTable() SymbolTable {
	return createNewLookupTable()
}

// createNewLookupTable creates a copy of the default symbol lookup table.
func createNewLookupTable() SymbolTable {
	symbolTable := make(SymbolTable, len(defaultSymbols))
	for key, value := range defaultSymbols {
		symbolTable[key] = value
	}

	return symbolTable
}

// String returns a string representation of the SymbolTable.
func (st SymbolTable) String() string {
	sb := strings.Builder{}
	symbolNames := make([]string, 0, len(st))
	for name := range st {
		symbolNames = append(symbolNames, name)
	}
	slices.Sort(symbolNames)
	sb.WriteString(", Symbols: [")
	for i, name := range symbolNames {
		symbol := st[name]
		sb.WriteString(symbol.String())
		if i < len(symbolNames)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("]")
	return sb.String()
}

func LoadSymbols(path string, theme string) error {
	var symbols map[string][]Symbol
	filePath := filepath.Join(path, "slots/symbols.yaml")
	if err := config.LoadConfig(filePath, &symbols); err != nil {
		return err
	}

	themeSymbols, ok := symbols[theme]
	if !ok {
		return ErrConfigNotFound
	}

	defaultSymbols = make(SymbolTable, len(themeSymbols))
	for _, symbol := range themeSymbols {
		defaultSymbols[symbol.Name] = symbol
	}

	return nil
}
