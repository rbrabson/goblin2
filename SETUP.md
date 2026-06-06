# Configuring the goblin bot for a Discord server

This document describes how to configure the `Goblin` bot for a Discord server. It is assumed that the `Goblin` bot has already been installed and is running on the server. The instructions require the Discord server owner to be logged in to the Discord server.

## Configuring Discord

### Setting the server owners

By default, a newly installed `Goblin` bot does not have any configured owners. This means that any user on the server may issue the most privlidged `/server owner` commands. Therefore, the first step in setting up the server is to set the server owner(s). Use the `/server owner add` command to add one or more server owners. It is recommended that the person installing the server start by adding themselves as a server owner and can then optionally add one or more additional owners.

A server owner can list the current set of server owners using the `/server owner list` command. A server owner can also remove any server owner using the`/server owner remove` command. Since any server owner can remove the person who initially setup the server, you must be extremely careful in who is added as a server owner.

Server owners can safely shut down the server using the `/server shutdown` command. The owners can then check the status of the server using the `/server status` command. Once all services on the server are in the `Stopped` state, it is safe to apply maintenance and/or reboot the server.

### Setting administrator permissions

To use administrative commands with the Goblin bot, users must have an administrative role assigned to them. When first installed, the Goblin bot assigns the following roles for admin users: `Admin`, `Admins`, `Administrator`, `Mod`, `Mods`, and `Moderator`. You can add additional roles or remove existing ones using the `/guild-admin role add` command, or remove an existing role using the `/guild-admin role remove` command. At any time, you can see the list of existing administrative roles using the `/guild-admin role list` command.

It is recommended that you create a `bot-admin` role, and then assign the role to bot adminstrators. This makes it easier to manage the permissions for the bot, as well as to add or remove administrators for the bot. This guide uses the `bot-admin` role as an example, but you can use any role you like, including an existing one.

### Suggested channel setup

### Channel creation

To configure the `Goblin` bot, navigate to the `Settings` section of the Discord server, and then select the `Channels` tab. You will want to create the following channels. The goblin bot must have permissions to read and write to these channels.

1. Create a channel for bot admininistrative commands, and name it something like `#bot-admin-commands`. Only `bot-admin`s and the `Goblin` bot should be able to use this channel. This is the main channel where administration commands are sent.

2. Create a channel for shop purchases to be logged, and name it something like `#shop-purchases`. Only `bot-admin`s and the `Goblin` bot should be able to use this channel. You may want to configure it so that `bot-admin`s can't write to the channel, as only the `Goblin` both should do that.

3. Create a `Game` category. Channels under this category are the main ones the server members will interact with. It is recommended that the default perissions is that anyone with the `@everyone` role can read and write to channels under this category, as well as post reactions. Some channels in the category will require additional configuration, which is described below.

4. Create a channel for the shop under the `Game` category, and name it something like `#shop`. Configure permissions so that the `Goblin` bot can read and write to the channel. The `@everyone` role should be able to see the channel, but not write to it nor create reactions.

5. Create a channel for the leaderboard under the `Game` category, and name it something like `#leaderboard`. Configure permissions so that the `Goblin` bot can read and write to the channel. The `@everyone` role should be able to see the channel, but not write to it nor create reactions.

6. Create a channel for heists under the `Game` category, and name it something like `#heists`. The `Goblin` bot will need additional permissions to manage the this channel and change the channel permissions, so that it may mute the channel when sending the results of a heist.

7. Create a channel for races under the `Game` category, and name it something like `#races`.

8. Create a channel for blackjack under the `Game` category, and name it something like `#blackjack`.

9. Create a channel for slots under the `Game` category, and name it something like `#slots`.

### Server integration

Now that the channels have been created, the channels to which commands may be sent should be configured. This is performed in the Server's settings by navigating to the `Integrations` tab, and then selecting the `Goblin` bot. All bot commands are listed, and you can click on them to provide permission overrides.

First, deselect `@everyone` and `# All Channels` options. This way, commands can only be sent to the channels you setup, and we are going to configure all commands explicitly.

#### Administrative commands

The following commands are administrative commands, and should only be sent to the `#bot-admin-commands` channel. Only users with the `bot-admin` role should be able to send these commands.

- `/account-admin`
- `/admin-help`
- `/bank-admin`
- `/blackjack-admin`
- `/heist-admin`
- `/guild-admin`
- `/lb-admin`
- `/race-admin`
- `/shop-admin`
- `/slots-admin`
- `/stats-admin`
- `/version`

#### Member commands

The following are member commands, and should be sent to the category or channels as desdribed below. They should be configured so that `@everyone` can use the commands.

- `/bank` - all server channels. You can optionally restrict it to the `Game` category.
- `/blackjack` - to the `#blackjack` channel
- `/heist` - to the `#heist` channel
- `/help` - all server channels. You can optionally restrict it to the `Game` category.
- `/lb` - to the `#leaderboard` channel
- `/payday` - all server channels. You can optionally restrict it to the `Game` category.
- `/payday-stats` - all server channels. You can optionally restrict it to the `Game` category.
- `/race` - to the `#races` channel
- `/shop` - all server channels. You can optionally restrict it to the `Game` category.
- `/slots` - to the `#slots` channel

### Setting up Goblin permissions in the game channels

The Goblin bot requires the ability to mute and unmute the channel in which the `heist` game is run. To achieve this, add the `Goblin` bot under the `ROLES/MEMBERS` section in the channel settings, and then add the following permissions to the bot: `View Channel`, `Manage Permissions`, and `Send Messages`. Without these settings, the `Goblin` bot will either be unable to mute the channel during a heist, or may be unable to send messages about the progress of a heist at all.

### Setting the leaderboard channel

The leaderboard is posted once a month, at the start of the month using UTC time. To have the leaderboard posted, you must first set a channel to which the leaderboard is sent. This is accomplished using the `/lb-admin channel` command. Not doing so will result in no leaderboard being sent to your Discord server.

### Setting up the shop

The `Goblin` bot is capable of providing a shop that may be used to purchase roles on the server using in-game credits. There are two types of roles that may be configured: permanent roles, and temporary roles. *Permanent* roles are, as the name implies, roles that are granted to the user and are intended to remain active indefinitely. *Temporary* roles are intended to be granted to the user for a period of time and then automatically removed when the time expires.

To setup a shop, two channels are required. The first is the channel to which to publish the shop. Each item that is added to the shop will appear in a message, with buttons that may be used to purchase the item. It is recommended that this channel be setup so that only members with admin priviledges on the server can post or delete messages. The `Goblin` bot must also be able to post and edit messages in this channel. To set the shop channel, use the `/shop-admin channel` command.

The second channel is to capture a log of all purchases made from the shop, as well as to log when a purchase expires. This channel should be visible to server admins and moderators, but no one else. To set the shop log channel, use the `/shop-admin mod-channel` command.

The shop also supports a `/shop-admin ban` command, which allows server admins and moderators to keep specific members from being able to purchase items from the shop. This is useful if you don't want alternate user accounts from being able to purchase shop items.
