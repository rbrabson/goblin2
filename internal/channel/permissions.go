package channel

import (
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

// permissionSnapshot stores the @everyone overwrite before muting so it can be restored later.
type permissionSnapshot struct {
	channelID            snowflake.ID
	everyoneOverwrite    discord.RolePermissionOverwrite
	hasEveryoneOverwrite bool
}

// PermissionManager is used for muting and unmuting a channel on a server
type PermissionManager struct {
	client   *bot.Client
	guildID  snowflake.ID
	snapshot permissionSnapshot
}

// NewPermissionManager creates a PermissionManager for client and command event.
func NewPermissionManager(client *bot.Client, e *handler.CommandEvent) (*PermissionManager, error) {
	guild, ok := e.Guild()
	if !ok {
		return nil, ErrNotInGuild
	}

	channel, err := e.Client().Rest.GetChannel(e.Channel().ID())
	if err != nil {
		return nil, err
	}

	guildChannel, ok := channel.(discord.GuildChannel)
	if !ok {
		return nil, ErrChannelNotInGuild
	}

	snapshot := permissionSnapshot{
		channelID: e.Channel().ID(),
	}
	if everyoneOverwrite, ok := guildChannel.PermissionOverwrites().Role(guild.ID); ok {
		snapshot.everyoneOverwrite = everyoneOverwrite
		snapshot.hasEveryoneOverwrite = true
	}

	pm := &PermissionManager{
		client:   client,
		guildID:  guild.ID,
		snapshot: snapshot,
	}

	return pm, nil
}

// MuteChannel sets the channel so that `@everyone` can't send messages in the channel.
func (pm *PermissionManager) MuteChannel() error {
	const mutePermissions = discord.PermissionSendMessages

	everyoneOverwrite := pm.snapshot.everyoneOverwrite
	everyoneOverwrite.RoleID = pm.guildID // @everyone role ID is the same as the guild ID

	if !pm.snapshot.hasEveryoneOverwrite {
		everyoneOverwrite = discord.RolePermissionOverwrite{
			RoleID: pm.guildID,
		}
	}

	allow := everyoneOverwrite.Allow & mutePermissions
	deny := everyoneOverwrite.Deny ^ mutePermissions

	update := discord.RolePermissionOverwriteUpdate{
		Allow: &allow,
		Deny:  &deny,
	}

	if err := pm.client.Rest.UpdatePermissionOverwrite(pm.snapshot.channelID, pm.guildID, update); err != nil {
		return err
	}

	slog.Info("muted channel",
		slog.Any("guildID", pm.guildID),
		slog.Any("channelID", pm.snapshot.channelID),
	)
	return nil
}

// UnmuteChannel resets the permissions for `@everyone` to what they were before the channel was muted.
func (pm *PermissionManager) UnmuteChannel() error {
	everyoneOverwrite := pm.snapshot.everyoneOverwrite
	everyoneOverwrite.RoleID = pm.guildID

	update := discord.RolePermissionOverwriteUpdate{
		Allow: &everyoneOverwrite.Allow,
		Deny:  &everyoneOverwrite.Deny,
	}

	if err := pm.client.Rest.UpdatePermissionOverwrite(pm.snapshot.channelID, pm.guildID, update); err != nil {
		return err
	}

	slog.Info("unmuted channel",
		slog.Any("guildID", pm.guildID),
		slog.Any("channelID", pm.snapshot.channelID),
	)

	return nil
}
