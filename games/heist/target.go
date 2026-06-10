package heist

import (
	"goblin2/internal/cache"
	"goblin2/internal/config"
	"goblin2/internal/discordid"
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
	ID       bson.ObjectID         `yaml:"-" bson:"_id,omitempty"`
	GuildID  discordid.SnowflakeID `yaml:"-" bson:"guild_id"`
	Theme    string                `yaml:"theme" bson:"theme"`
	Name     string                `yaml:"target_id" bson:"target_id"`
	CrewSize int                   `yaml:"crew" bson:"crew"`
	Success  float64               `yaml:"success" bson:"success"`
	Vault    int                   `yaml:"vault" bson:"vault"`
	VaultMax int                   `yaml:"vault_max" bson:"vault_max"`
	IsAtMax  bool                  `yaml:"is_at_max" bson:"is_at_max"`
}

// LoadTargets loads the default heist targets from the specified YAML file path.
func LoadTargets(path string) error {
	filePath := filepath.Join(path, "heist/targets.yaml")
	if err := config.LoadConfig(filePath, &defaultTargets); err != nil {
		return err
	}
	for _, target := range defaultTargets {
		target.Theme = "clash"
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

// getAllTargets returns all targets that match the filter.
func getAllTargets(filter bson.M) []*Target {
	allTargets, _ := readAllTargets(filter)

	return allTargets
}

// copyTargets creates a deep copy of a list of targets.
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

// updateTargetCache updates the cached target list for a target's guild/theme if it is currently cached.
func updateTargetCache(target *Target) {
	if target == nil {
		return
	}

	key := targetsCacheKey{
		guildID: target.GuildID,
		theme:   target.Theme,
	}

	targets, ok := targetsCache.Get(key)
	if !ok {
		return
	}

	updated := copyTargets(targets)
	for i, cachedTarget := range updated {
		if cachedTarget == nil {
			continue
		}

		if cachedTarget.ID == target.ID || cachedTarget.Name == target.Name {
			updated[i] = new(*target)
			targetsCache.Set(key, updated)
			return
		}
	}

	updated = append(updated, new(*target))
	targetsCache.Set(key, updated)
}

// vaultUpdater updates the vault balance for any target whose vault is not at the maximum value
func vaultUpdater() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	updateVaults()

	for range ticker.C {
		updateVaults()
	}
}

// updateVaults updates the vault balance for any target whose vault is not at the maximum value
func updateVaults() {
	filter := bson.M{"is_at_max": false}

	var cfg *Config
	for _, target := range getAllTargets(filter) {
		if target == nil {
			continue
		}

		if cfg == nil || cfg.GuildID != target.GuildID {
			cfg = GetConfig(target.GuildID)
		}
		if cfg == nil {
			slog.Error("unable to update vault without heist config",
				slog.Any("guildID", target.GuildID),
				slog.String("target", target.Name),
			)
			continue
		}

		recoverPercent := cfg.BaseVaultRecovery
		if cfg.BoostEnabled {
			recoverPercent = cfg.BoostVaultRecovery
		}

		recoverAmount := int(float64(target.VaultMax) * recoverPercent)
		newVaultAmount := min(target.Vault+recoverAmount, target.VaultMax)

		slog.Info("vault updater",
			slog.Any("guildID", target.GuildID),
			slog.String("target", target.Name),
			slog.Int("old", target.Vault),
			slog.Int("new", newVaultAmount),
			slog.Int("max", target.VaultMax),
		)

		target.Vault = newVaultAmount
		target.IsAtMax = target.Vault >= target.VaultMax
		writeTarget(target)
	}
}

// CloseTargetsCache stops the target cache cleanup goroutine and clears cached targets.
func CloseTargetsCache() {
	targetsCache.Destroy()
}
