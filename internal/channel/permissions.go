package channel

import (
	"errors"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

type permissionSnapshot struct {
	channelID  snowflake.ID
	overwrites []discord.PermissionOverwrite
}

// PermissionManager is used for muting and unmuting a channel on a server
type PermissionManager struct {
	client   *bot.Client
	event    *handler.CommandEvent
	guildID  snowflake.ID
	channel  discord.GuildChannel
	snapshot permissionSnapshot
}

// NewPermissionManager creates a PermissionManager for client and command event.
func NewPermissionManager(client *bot.Client, e *handler.CommandEvent) (*PermissionManager, error) {
	guild, ok := e.Guild()
	if !ok {
		return nil, errors.New("command must be used in a guild")
	}

	channel, err := e.Client().Rest.GetChannel(e.Channel().ID())
	if err != nil {
		return nil, err
	}

	guildChannel, ok := channel.(discord.GuildChannel)
	if !ok {
		return nil, errors.New("channel must be a guild channel")
	}

	existingOverwrites := guildChannel.PermissionOverwrites()

	snapshot := permissionSnapshot{
		channelID:  e.Channel().ID(),
		overwrites: append([]discord.PermissionOverwrite(nil), existingOverwrites...),
	}

	pm := &PermissionManager{
		client:   client,
		event:    e,
		guildID:  guild.ID,
		channel:  guildChannel,
		snapshot: snapshot,
	}

	return pm, nil
}

// MuteChannel sets the channel so that `@everyone` can't send messages or create threads in the channel.
func (pm *PermissionManager) MuteChannel() error {
	const mutePermissions = discord.PermissionSendMessages |
		discord.PermissionSendMessagesInThreads |
		discord.PermissionCreatePublicThreads |
		discord.PermissionCreatePrivateThreads

	overwrites := make([]discord.PermissionOverwrite, 0, len(pm.snapshot.overwrites)+1)

	everyoneOverwrite := discord.RolePermissionOverwrite{
		RoleID: pm.guildID, // @everyone role ID is the same as the guild ID
		Allow:  0,
		Deny:   mutePermissions,
	}

	for _, overwrite := range pm.snapshot.overwrites {
		if overwrite.Type() != discord.PermissionOverwriteTypeRole || overwrite.ID() != pm.guildID {
			overwrites = append(overwrites, overwrite)
			continue
		}

		roleOverwrite := overwrite.(discord.RolePermissionOverwrite)
		everyoneOverwrite.Allow = roleOverwrite.Allow
		everyoneOverwrite.Deny = roleOverwrite.Deny | mutePermissions
	}

	overwrites = append(overwrites, everyoneOverwrite)

	update := discord.GuildTextChannelUpdate{
		PermissionOverwrites: &overwrites,
	}

	updatedChannel, err := pm.client.Rest.UpdateChannel(pm.snapshot.channelID, update)
	if err != nil {
		return err
	}

	guildChannel, ok := updatedChannel.(discord.GuildChannel)
	if ok {
		pm.channel = guildChannel
	}

	slog.Info("muted channel",
		slog.Any("guildID", pm.guildID),
		slog.Any("channelID", pm.snapshot.channelID),
	)
	return nil
}

// UnmuteChannel resets the permissions for `@everyone` to what they were before the channel was muted.
func (pm *PermissionManager) UnmuteChannel() error {
	overwrites := append([]discord.PermissionOverwrite(nil), pm.snapshot.overwrites...)

	update := discord.GuildTextChannelUpdate{
		PermissionOverwrites: &overwrites,
	}

	updatedChannel, err := pm.client.Rest.UpdateChannel(pm.snapshot.channelID, update)
	if err != nil {
		return err
	}

	guildChannel, ok := updatedChannel.(discord.GuildChannel)
	if ok {
		pm.channel = guildChannel
	}

	slog.Info("unmuted channel",
		slog.Any("guildID", pm.guildID),
		slog.Any("channelID", pm.snapshot.channelID),
	)

	return nil
}
