package heist

import (
	"goblin2/config"
	"goblin2/discordid"
	"goblin2/internal/cache"
	"log/slog"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	heistTargetsCacheTTL             = 30 * time.Minute
	heistTargetsCacheCleanupInterval = 5 * time.Minute
)

type targetsCacheKey struct {
	guildID discordid.SnowflakeID
	theme   string
}

var (
	defaultTargets []*Target
	targetsCache   = cache.New[targetsCacheKey, []*Target](heistTargetsCacheTTL, heistTargetsCacheCleanupInterval)
)

// Target is a target of a heist.
type Target struct {
	ID       bson.ObjectID         `bson:"_id,omitempty"`
	GuildID  discordid.SnowflakeID `bson:"guild_id"`
	Theme    string                `bson:"theme"`
	Name     string                `bson:"target_id"`
	CrewSize int                   `bson:"crew"`
	Success  float64               `bson:"success"`
	Vault    int                   `bson:"vault"`
	VaultMax int                   `bson:"vault_max"`
	IsAtMax  bool                  `bson:"is_at_max"`
}

// LoadTargets loads the default heist targets from the specified YAML file path.
func LoadTargets(path string) error {
	filePath := filepath.Join(path, "heist/targets.yaml")
	if err := config.LoadConfig(filePath, &defaultTargets); err != nil {
		return err
	}

	return nil
}

// GetTargets returns the list of targets for the server.
func GetTargets(guildID discordid.SnowflakeID) []*Target {
	theme := GetTheme(guildID)
	themeName := ""
	if theme != nil {
		themeName = theme.Name
	}

	key := targetsCacheKey{
		guildID: guildID,
		theme:   themeName,
	}

	if targets, ok := targetsCache.Get(key); ok {
		return copyTargets(targets)
	}

	targets, err := readTargets(key.guildID, themeName)
	if err == nil && len(targets) > 0 {
		targetsCache.Set(key, copyTargets(targets))
		return copyTargets(targets)
	}

	targets = createNewTargets(guildID)
	for _, target := range targets {
		writeTarget(target)
	}

	targetsCache.Set(key, copyTargets(targets))

	return copyTargets(targets)
}

// createNewTargets creates a list of targets for a guild with the default target values.
func createNewTargets(guildID discordid.SnowflakeID) []*Target {
	targets := make([]*Target, 0, len(defaultTargets))
	for _, target := range defaultTargets {
		targets = append(targets, &Target{
			GuildID:  guildID,
			Theme:    target.Theme,
			Name:     target.Name,
			CrewSize: target.CrewSize,
			Success:  target.Success,
			Vault:    target.Vault,
			VaultMax: target.VaultMax,
			IsAtMax:  target.IsAtMax,
		})
	}
	slog.Warn("created default heist targets",
		slog.Any("guildID", guildID),
	)

	return targets
}

func copyTargets(targets []*Target) []*Target {
	if targets == nil {
		return nil
	}

	copied := make([]*Target, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			copied = append(copied, nil)
			continue
		}

		copied = append(copied, new(*target))
	}

	return copied
}

func CloseTargetsCache() {
	targetsCache.Destroy()
}
