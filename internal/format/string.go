package format

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// FirstToUpper capitalizes the first letter of a string.
func FirstToUpper(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	runes[0] = []rune(cases.Upper(language.Und).String(string(runes[0])))[0]
	return string(runes)
}

// FirstToLower lowercases the first letter of a string.
func FirstToLower(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	runes[0] = []rune(cases.Lower(language.Und).String(string(runes[0])))[0]
	return string(runes)
}
