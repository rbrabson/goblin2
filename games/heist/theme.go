package heist

import (
	"goblin2/internal/cache"
	"goblin2/internal/config"
	"goblin2/internal/discordid"
	"goblin2/internal/gameassets"
	"log/slog"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	heistThemeCacheTTL             = 30 * time.Minute
	heistThemeCacheCleanupInterval = 5 * time.Minute
)

type themeCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	defaultTheme Theme
	themeCache   = cache.New[themeCacheKey, Theme](heistThemeCacheTTL, heistThemeCacheCleanupInterval)
)

// A Theme is a set of messages that provide a "flavor" for a heist
type Theme struct {
	ID                  bson.ObjectID         `yaml:"-" bson:"_id,omitempty"`
	GuildID             discordid.SnowflakeID `yaml:"-" bson:"guild_id"`
	Name                string                `yaml:"name" bson:"name"`
	EscapedMessages     []*Message            `yaml:"escaped_messages" bson:"escaped_messages"`
	ApprehendedMessages []*Message            `yaml:"apprehended_messages" bson:"apprehended_messages"`
	DiedMessages        []*Message            `yaml:"died_messages" bson:"died_messages"`
	Jail                string                `yaml:"jail" bson:"jail"`
	OOB                 string                `yaml:"oob" bson:"oob"`
	Police              string                `yaml:"police" bson:"police"`
	Bail                string                `yaml:"bail" bson:"bail"`
	Crew                string                `yaml:"crew" bson:"crew"`
	Sentence            string                `yaml:"sentence" bson:"sentence"`
	Heist               string                `yaml:"heist" bson:"heist"`
	Vault               string                `yaml:"vault" bson:"vault"`
}

// A Message is a message for a successful heist outcome
type Message struct {
	Message     string       `yaml:"message" bson:"message"`
	BonusAmount int          `yaml:"bonus_amount" bson:"bonus_amount,omitempty"`
	Result      MemberStatus `yaml:"result" bson:"result"`
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
func GetTheme(guildID discordid.SnowflakeID) *Theme {
	key := themeCacheKey{
		guildID: guildID,
	}

	if theme, ok := themeCache.Get(key); ok {
		return copyTheme(&theme)
	}

	if gameassets.UseYAMLGameAssets() {
		theme := createNewTheme(guildID)
		themeCache.Set(key, *theme)
		return copyTheme(theme)
	}

	theme, err := readTheme(key.guildID)
	if err == nil && theme != nil {
		themeCache.Set(key, *theme)
		return copyTheme(theme)
	}

	theme = createNewTheme(guildID)
	writeTheme(theme)

	return copyTheme(theme)
}

// createNewTheme creates a new theme for a guild with the default theme values.
func createNewTheme(guildID discordid.SnowflakeID) *Theme {
	theme := &Theme{
		GuildID:             guildID,
		Name:                defaultTheme.Name,
		EscapedMessages:     copyHeistMessages(defaultTheme.EscapedMessages),
		ApprehendedMessages: copyHeistMessages(defaultTheme.ApprehendedMessages),
		DiedMessages:        copyHeistMessages(defaultTheme.DiedMessages),
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

// copyTheme creates a deep copy of a theme.
func copyTheme(theme *Theme) *Theme {
	if theme == nil {
		return nil
	}

	copied := new(*theme)
	copied.EscapedMessages = copyHeistMessages(theme.EscapedMessages)
	copied.ApprehendedMessages = copyHeistMessages(theme.ApprehendedMessages)
	copied.DiedMessages = copyHeistMessages(theme.DiedMessages)

	return copied
}

// copyHeistMessages creates a deep copy of a slice of heist messages.
func copyHeistMessages(messages []*Message) []*Message {
	if messages == nil {
		return nil
	}

	copied := make([]*Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			copied = append(copied, nil)
			continue
		}

		copied = append(copied, new(*message))
	}

	return copied
}

// CloseThemeCache stops the theme cache cleanup goroutine and clears cached theme entries.
func CloseThemeCache() {
	themeCache.Destroy()
}
