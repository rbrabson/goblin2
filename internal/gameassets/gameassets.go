package gameassets

import (
	"os"
	"strings"
)

const useYAMLGameAssetsEnv = "USE_YAML_GAME_ASSETS"

func UseYAMLGameAssets() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(useYAMLGameAssetsEnv)))

	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
