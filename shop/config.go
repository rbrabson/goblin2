package shop

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/discordid"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Config represents the configuration for the shop in a guild.
type Config struct {
	ID             bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID        discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	ChannelID      string                `json:"channel_id" bson:"channel_id"`
	MessageID      string                `json:"message_id" bson:"message_id"`
	ModChannelID   string                `json:"mod_channel_id" bson:"mod_channel_id"`
	NotificationID string                `json:"notification_id" bson:"notification_id"`
	Version        int                   `json:"version" bson:"version"`
}

// GetConfig reads the configuration from the database. If the config does not exist,
// then one is created.
func GetConfig(guildID discordid.SnowflakeID) *Config {
	key := configCacheKey{
		guildID: guildID,
	}

	if config, ok := configCache.Get(key); ok {
		return copyConfig(&config)
	}

	config, _ := readConfig(key.guildID)
	if config != nil {
		configCache.Set(key, *config)
		return copyConfig(config)
	}

	config = newConfig(guildID)
	configCache.Set(key, *config)
	return copyConfig(config)
}

// newConfig creates a new configuration for the given guild ID.
func newConfig(guildID discordid.SnowflakeID) *Config {
	config := &Config{
		GuildID: guildID,
	}

	slog.Info("created new shop config", "guildID", guildID)

	return config
}

// UpdateConfig updates the shop config with the given mutation, retrying on version conflicts.
func UpdateConfig(guildID discordid.SnowflakeID, mutate func(*Config) error) error {
	const maxRetries = 3

	configMu.RLock()
	defer configMu.RUnlock()

	key := configCacheKey{
		guildID: guildID,
	}

	for range maxRetries {
		config := GetConfig(guildID)

		if err := mutate(config); err != nil {
			return err
		}

		var err error
		if config.ID.IsZero() {
			err = writeConfig(config)
		} else {
			err = updateConfig(config)
		}

		if err == nil {
			configCache.Set(key, *config)
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			configCache.Delete(key)
			return err
		}

		configCache.Delete(key)

		slog.Warn("version conflict on shop config, retrying",
			slog.Any("guildID", guildID),
		)
	}

	return fmt.Errorf("failed to update shop config after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// SetChannel sets the channel to which to publish the shop items
func (c *Config) SetChannel(channelID string) {
	if err := UpdateConfig(c.GuildID, func(latest *Config) error {
		if latest.ChannelID == channelID {
			return nil
		}
		latest.ChannelID = channelID
		latest.MessageID = ""
		return nil
	}); err != nil {
		slog.Error("error setting the shop channel", "error", err)
		return
	}
	slog.Debug("set shop channel", "guildID", c.GuildID, "channel", channelID)
}

// SetModChannel sets the channel to which to publish the shop purchases and expirations.
func (c *Config) SetModChannel(channelID string) {
	if err := UpdateConfig(c.GuildID, func(latest *Config) error {
		if latest.ModChannelID == channelID {
			return nil
		}
		latest.ModChannelID = channelID
		return nil
	}); err != nil {
		slog.Error("error setting the shop mod channel", "error", err)
		return
	}
	slog.Debug("set shop mod channel", "guildID", c.GuildID, "channel", channelID)
}

// SetNotificationID sets the channel to which to notify a user (e.g., ModMail) about an action to take to complete a member's purchase.
func (c *Config) SetNotificationID(id string) {
	if err := UpdateConfig(c.GuildID, func(latest *Config) error {
		if latest.NotificationID == id {
			return nil
		}
		latest.NotificationID = id
		return nil
	}); err != nil {
		slog.Error("error setting the shop notification id", "error", err)
		return
	}
	slog.Debug("set shop notification ID", "guildID", c.GuildID, "member", id)
}

// SetMessageID saves the interaction used to publish the shop items.
func (c *Config) SetMessageID(messageID string) {
	if err := UpdateConfig(c.GuildID, func(latest *Config) error {
		if latest.MessageID == messageID {
			return nil
		}
		latest.MessageID = messageID
		return nil
	}); err != nil {
		slog.Error("error setting the shop message id", "error", err)
		return
	}
	slog.Debug("set shop message ID", "guildID", c.GuildID, "messageID", messageID)
}

// String returns a string representation of the config.
func (c *Config) String() string {
	return "Config{" +
		"ID: " + c.ID.Hex() +
		", GuildID: " + c.GuildID.String() +
		", ChannelID: " + c.ChannelID +
		", MessageID: " + c.MessageID +
		", ModChannelID: " + c.ModChannelID +
		", NotificationID: " + c.NotificationID +
		"}"
}
