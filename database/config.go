package database

// Config represents the configuration for the database.
type Config struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

// LoadConfig loads the configuration from the specified yaml file path.
func LoadConfig(dbName, dbURL string) (*Config, error) {
	return &Config{
		URI:      dbURL,
		Database: dbName,
	}, nil
}
