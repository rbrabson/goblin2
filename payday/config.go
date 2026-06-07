package payday

import (
	"goblin2/internal/config"
	"path/filepath"
	"time"
)

var (
	defaultConfig Config
)

type Config struct {
	PaydayAmount               int           `yaml:"payday_amount"`
	PaydayFrequency            time.Duration `yaml:"-"`
	PaydayFrequencyNanoseconds int64         `yaml:"payday_frequency"`
	MaxStreak                  int           `yaml:"max_streak"`
	StreakPerDayBonus          int           `yaml:"streak_per_day_bonus"`
}

// LoadConfig loads the configuration from the specified YAML file path.
func LoadConfig(path string) error {
	filePath := filepath.Join(path, "payday/config.yaml")
	if err := config.LoadConfig(filePath, &defaultConfig); err != nil {
		return err
	}

	defaultConfig.PaydayFrequency = time.Duration(defaultConfig.PaydayFrequencyNanoseconds)

	return nil
}
