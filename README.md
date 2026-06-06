# Goblin

A Discord bot built with Go that integrates with MongoDB, providing various game and utility features for Discord servers.

## 🐳 Quick Start with Docker

The easiest way to run Goblin is using Docker:

```bash
# Quick setup
./setup.sh

# Edit your bot configuration
nano .env

# Edit your MongoDB configuration
namo .env-mongo

# Start the bot
docker-compose up -d
```

`setup.sh` requires the configuration files are in the same directory as the `docker-compose.yaml` file
under a directory named `yaml`. If this isn't true for your environment, then you can manually edit 
the `Dockerfile`.

📖 **See [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) for complete Docker setup instructions**, including:

- Building, tagging, and pushing Docker images
- Multiple deployment options (pre-built image, local build, with/without .env)
- Production deployment best practices
- Troubleshooting guide

## Changelog

See the [changelog](CHANGELOG.md) for details.

## Running the Goblin Bot

### Configuring the Goblin Bot

The Goblin bot relies on a set of environment variables to configure it.

#### Goblin Bot

1. Set the path to the configuration files directory.

    ```bash
    # Discord Bot Configuration
    GOBLIN_CONFIG_PATH="<path_to_config>"
    ```

2. Create the required directories:

    ```bash
    mkdir -p "$GOBLIN_CONFIG_PATH"
    mkdir -p "$GOBLIN_CONFIG_PATH/bank"
    mkdir -p "$GOBLIN_CONFIG_PATH/bot"
    mkdir -p "$GOBLIN_CONFIG_PATH/heist"
    mkdir -p "$GOBLIN_CONFIG_PATH/log"
    mkdir -p "$GOBLIN_CONFIG_PATH/payday"
    ```
3. Edit the bank configuration file `$GOBLIN_CONFIG_PATH/bank/config.yaml`:

    ```yaml
    # Default bank to be used if none is specified
    default_theme: "clash"
    
    # Themes
    themes:
      clash:
        bank_name: "Treasury"
        currency: "Coins"
        default_balance: 20000
    ```

4. Edit the bot configuration file `$GOBLIN_CONFIG_PATH/bot/config.yaml`:

    ```yaml
    # add guild ids the commands should sync to, leave empty to sync globally
    dev_guilds: []
    # the bot token
    token: <token>>
    ```

5. Edit the MongoDB configuration file `$GOBLIN_CONFIG_PATH/database/config.yaml`:

    ```yaml
    database: <db_name>
    uri: <mongodb_uri>
    ```

6. Edit the heist configuration files:

    `$GOBLIN_CONFIG_PATH/heist/config.yaml`
    ```yaml
    bail_base: 250
    boost_enabled: false
    boost_multiplier: 0
    crew_output: None
    death_timer: 45000000000
    heist_cost: 1000
    police_alert: 60000000000
    sentence_base: 45000000000
    vault_recover_percentage: 0.04
    wait_time: 60000000000
    ```

    `$GOBLIN_CONFIG_PATH/heist/targets.yaml`
    ```yaml
    - target_id: Goblin Forest
      crew: 2
      success: 29.3
      vault_max: 16000
    - target_id: Goblin Outpost
      crew: 3
      success: 20.65
      vault_max: 24000
      ...
    ```

    `$GOBLIN_CONFIG_PATH/heist/theme.yaml`
    ```yaml
    escaped_messages:
      - message: >-
          %s brought a few healers to keep themself alive
          <:Healer:1346563687137280081>, +25 <:Gold:1346583263598219297>
        bonus_amount: 25
        result: Escaped
        ...
    
    apprehended_messages:
      - message: '%s stepped onto a spring trap.'
        result: Apprehended
      - message: >-
          %s forgot to bring heals to their GoHo attack <:Golem:1346563552684671017>
          <:Hog_Rider:1346563786408071239>.
        result: Apprehended
        ...
    
    died_messages:
      - message: >-
          %s forgot funnel and was singled out by an archer tower. (death)
          <:Toombstone:1346597374054633554>
        result: Dead
        ...
    ```

7. Edit the log configuration file `$GOBLIN_CONFIG_PATH/log/config.yaml`:

    ```yaml
    # valid levels are "debug", "info", "warn", "error"
    level: info
    # valid formats are "text" and "json"
    format: text
    # whether to add the log source to the log message
    add_source: true
    ```

8. Edit the payday configuration file `$GOBLIN_CONFIG_PATH/payday/config.yaml`

    ```yaml
    # Default payday configuration
    payday_amount: 5000
    payday_frequency: 82800000000000 # 23 hours in nanoseconds
    max_streak: 7
    streak_per_day_bonus: 500
    ```

Once mongosh has started, enter the following command to create a user who can read and write to the specified
database.

```bash
use admin
db.createUser(
  {
    user: "<MONGODB_USERID>",
    pwd: "<MONGODB_PASSWORD>",
    roles: [ { role: "readWrite", db: "<MONGODB_DATABASE>" } ]
  }
)
```

### Run as a Standalone Application

When developing, you can use:

```bash
go run cmd/goblin/main.go
```

to run the Goblin bot. Once it is stable, you can use the `make` command to generate a binary that you can
execute. The binary files are located in the `bin` directory.

#### Start Container

```bash
docker run --env-file ./.env --name <container-name> goblin:1.0.0
```

### Run using `docker compose`

```bash
docker compose up --build
```

## Configuring Discord

### Setting the server owners

By default, a newly installed `Goblin` bot does not have any configured owners. This means that any user on the server may issue the most privlidged `/server owner` commands. Therefore, the first step in setting up the server is to set the server owner(s). Use the `/server owner add` command to add one or more server owners. It is recommended that the person installing the server start by adding themself as a server owner, and can then optionally add one or more additional owners.

A server owner can list the current set of server owners using the `/server owner list` command. A server owner can also remove any server owner using the`/server owner remove` command. Since any server owner can remove the person who initially setup the server, you must be extremely careful in who is added as a server owner.

Server owners have the ability to safely shut down the server using the `/server shutdown` command. The owners can then check the status of the server using the `/server status` command. Once all services on the server are in the `Stopped` state, it is safe to apply maintenance and/or reboot the server.

### Setting administrator permissions

In order to use administrative commands with the Goblin bot, users must have an administrative role assigned to them. When first installed, the Goblin bot assigns the following roles for admin users: `Admin`, `Admins`, `Administrator`, `Mod`, `Mods`, and `Moderator`. You can add additional roles or remove existing ones using the `/guild-admin role add` command, or remove an existing role using the `/guild-admin role remove` command. At any time, you can see the list of existing administrative roles using the `/guild-admin role list` command.

### Setting up Goblin permissions in the game channels

The Goblin bot requires the ability to mute and unmute the channel in which the `heist` game is run. To accomplish this, add the `Goblin` bot under the `ROLES/MEMBERS` section in the channel settings, and then add the following permissions to the bot: `View Channel`, `Manage Permissions`, and `Send Messages`. Without these settings, the `heist` bot will either be unable to mute the channel during a heist, or may be unable to send messages about the progress of a heist at all.

### Segregating administration and member commands to separate channels

There are two sets of commands used by the `Goblin` bot: administrative commands and member commands. All administrative commands start with `/<command>-admin`, while the help command is `/adminhelp`. This allows a user to easily configure the integration settings for the Goblin bot to only allow the use of administrative commands in a channel accessible only to administrators. Failing to do so won't allow general users to use administrative commands, as this is protected via the administration roles. However, the administrative commands will show up in the list of available commands to the users, which may not be desirable.

### Setting the leaderboard channel

The leaderboard is posted once a month, at the start of the month using UTC time. To have the leaderboard posted, you must first set a channel to which the leaderboard is sent. This is accomplished using the `/lb-admin channel` command. Not doing so will result in no leaderboard being sent to your Discord server.

### Setting up the shop

The `Goblin` bot is capable of providing a shop that may be used to purchase roles on the server using in-game credits. There are two types of roles that may be configured: permanent roles, and temporary roles. *Permanent* roles are, as the name implies, roles that are granted to the user and are intended to remain active indefinitely. *Temporary* roles are intended to be granted to the user for a period of time and then automatically removed when the time expires.

To setup a shop, two channels are required. The first is the channel to which to publish the shop. Each item that is added to the shop will appear in a message, with buttons that may be used to purchase the item. It is recommended that this channel be setup so that only members with admin priviledges on the server can post or delete messages. The `Goblin` bot must also be able to post and edit messages in this channel. To set the shop channel, use the `/shop-admin channel` command.

The second channel is to capture a log of all purchases made from the shop, as well as to log when a purchase expires. This channel should be visible to server admins and moderators, but no one else. To set the shop log channel, use the `/shop-admin mod-channel` command.

The shop also supports a `/shop-admin ban` command, which allows server admins and moderators to keep specific members from being able to purchase items from the shop. This is useful if you don't want alternate user accounts from being able to purchase shop items.
