package slots

import (
	"goblin2/internal/config"
	"path/filepath"

	rslots "github.com/rbrabson/slots"
)

var (
	defaultLookupTable rslots.LookupTable
)

// GetLookupTable retrieves the lookup table for the specified guild.
func GetLookupTable() rslots.LookupTable {
	lookupTable := createLookupTable()
	return lookupTable
}

// createLookupTable creates a copy of the default lookup table.
func createLookupTable() rslots.LookupTable {
	lookupTable := make(rslots.LookupTable, len(defaultLookupTable))
	for key, value := range defaultLookupTable {
		lookupTable[key] = value
	}

	return lookupTable
}

// LoadLookupTable loads the lookup table from a YAML configuration file.
func LoadLookupTable(path string) error {
	filePath := filepath.Join(path, "slots/lookup.yaml")
	if err := config.LoadConfig(filePath, &defaultLookupTable); err != nil {
		return err
	}

	return nil
}
