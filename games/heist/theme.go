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
	defaultTheme Theme
)

// A Theme is a set of messages that provide a "flavor" for a heist
type Theme struct {
	ID                  bson.ObjectID         `bson:"_id,omitempty"`
	GuildID             discordid.SnowflakeID `bson:"guild_id"`
	Name                string                `bson:"name"`
	EscapedMessages     []*HeistMessage       `bson:"escaped_messages"`
	ApprehendedMessages []*HeistMessage       `bson:"apprehended_messages"`
	DiedMessages        []*HeistMessage       `bson:"died_messages"`
	Jail                string                `bson:"jail"`
	OOB                 string                `bson:"oob"`
	Police              string                `bson:"police"`
	Bail                string                `bson:"bail"`
	Crew                string                `bson:"crew"`
	Sentence            string                `bson:"sentence"`
	Heist               string                `bson:"heist"`
	Vault               string                `bson:"vault"`
}

// A HeistMessage is a message for a successful heist outcome
type HeistMessage struct {
	Message     string       `bson:"message"`
	BonusAmount int          `bson:"bonus_amount,omitempty"`
	Result      MemberStatus `bson:"result"`
}

// LoadTheme loads the default heist theme from the configuration file
func LoadTheme(path string) error {
	filePath := filepath.Join(path, "heist/theme.yaml")
	if err := config.LoadConfig(filePath, &defaultTheme); err != nil {
		return err
	}

	return nil
}

// GetTheme returns the theme for a guild
func GetTheme(guildID snowflake.ID) *Theme {
	theme, err := readTheme(discordid.NewSnowflakeID(guildID))
	if err == nil && theme != nil {
		return theme
	}
	theme = createNewTheme(guildID)

	writeTheme(theme)

	return theme
}

// createNewTheme creates a new theme for a guild with the default theme values.
func createNewTheme(guildID snowflake.ID) *Theme {
	theme := &Theme{
		GuildID:             discordid.NewSnowflakeID(guildID),
		Name:                defaultTheme.Name,
		EscapedMessages:     defaultTheme.EscapedMessages,
		ApprehendedMessages: defaultTheme.ApprehendedMessages,
		DiedMessages:        defaultTheme.DiedMessages,
		Jail:                defaultTheme.Jail,
		OOB:                 defaultTheme.OOB,
		Police:              defaultTheme.Police,
		Bail:                defaultTheme.Bail,
		Crew:                defaultTheme.Crew,
		Sentence:            defaultTheme.Sentence,
		Heist:               defaultTheme.Heist,
		Vault:               defaultTheme.Vault,
	}

	slog.Info("created default heist theme",
		slog.Any("guildID", guildID),
		slog.String("theme", theme.Name),
	)

	return theme
}
