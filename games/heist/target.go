package heist

import (
	"goblin2/config"
	"goblin2/discordid"
	"log/slog"
	"path/filepath"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	defaultTargets []*Target
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
func GetTargets(guildID snowflake.ID) []*Target {
	theme := GetTheme(guildID)
	themeName := ""
	if theme != nil {
		themeName = theme.Name
	}

	targets, err := readTargets(discordid.NewSnowflakeID(guildID), themeName)
	if err == nil && len(targets) > 0 {
		return targets
	}

	targets = createNewTargets(guildID)
	for _, target := range targets {
		writeTarget(target)
	}

	return targets
}

// createNewTargets creates a list of targets for a guild with the default target values.
func createNewTargets(guildID snowflake.ID) []*Target {
	targets := make([]*Target, 0, len(defaultTargets))
	for _, target := range defaultTargets {
		targets = append(targets, &Target{
			GuildID:  discordid.NewSnowflakeID(guildID),
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
