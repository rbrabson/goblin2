package race

import (
	"goblin2/config"
	"goblin2/discordid"
	"goblin2/internal/cache"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	raceAvatarsCacheTTL             = 30 * time.Minute
	raceAvatarsCacheCleanupInterval = 5 * time.Minute
)

type avatarsCacheKey struct {
	guildID discordid.SnowflakeID
	theme   string
}

var (
	defaultAvatars []*Avatar
	avatarsCache   = cache.New[avatarsCacheKey, []*Avatar](raceAvatarsCacheTTL, raceAvatarsCacheCleanupInterval)
)

// Avatar represents a character that may be assigned to a member that participates in a race
type Avatar struct {
	ID            bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID       discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	Theme         string                `json:"theme" bson:"theme"`
	Emoji         string                `json:"emoji" bson:"emoji"`
	MovementSpeed string                `json:"movement_speed" bson:"movement_speed"`
}

// getRaceAvatars returns the list of avatars that may be assigned to a member during a race.
func getRaceAvatars(guildID discordid.SnowflakeID, themeName string) []*Avatar {
	key := avatarsCacheKey{
		guildID: guildID,
		theme:   themeName,
	}

	avatars, ok := avatarsCache.Get(key)
	if !ok {
		filter := bson.D{{Key: "guild_id", Value: guildID}, {Key: "theme", Value: themeName}}
		var err error
		avatars, err = readAllRacers(filter)
		if err != nil {
			avatars = createNewAvatars(guildID, themeName)
		}

		avatarsCache.Set(key, copyAvatars(avatars))
	}

	avatars = copyAvatars(avatars)
	rand.Shuffle(len(avatars), func(i, j int) {
		avatars[i], avatars[j] = avatars[j], avatars[i]
	})

	return avatars
}

// createNewAvatars reads the list of avatars for the theme and guild from the database. If the list
// does not exist, then an error is returned.
func createNewAvatars(guildID discordid.SnowflakeID, themeName string) []*Avatar {
	avatars := copyAvatars(defaultAvatars)
	for _, avatar := range avatars {
		avatar.GuildID = guildID
		avatar.Theme = themeName
		writeRacer(avatar)
	}

	slog.Debug("create new race avatars",
		slog.Any("guildID", guildID),
		slog.String("theme", themeName),
		slog.Int("count", len(defaultAvatars)),
	)

	return avatars
}

func copyAvatars(avatars []*Avatar) []*Avatar {
	if avatars == nil {
		return nil
	}

	copied := make([]*Avatar, 0, len(avatars))
	for _, avatar := range avatars {
		if avatar == nil {
			copied = append(copied, nil)
			continue
		}

		copied = append(copied, new(*avatar))
	}

	return copied
}

// calculateMovement calculates the distance a racer moves on a given turn
func (avatar *Avatar) calculateMovement(currentTurn int) int {
	cfg := GetConfig(avatar.GuildID)
	source := rand.NewPCG(rand.Uint64(), rand.Uint64())
	r := rand.New(source)
	switch avatar.MovementSpeed {
	case "veryfast":
		return r.IntN(8) * 2
	case "fast":
		return r.IntN(5) * 3
	case "slow":
		return (r.IntN(3) + 1) * 3
	case "steady":
		return 2 * 3
	case "aberrant":
		chance := r.IntN(100)
		if chance >= 70 {
			return 5 * 3
		}
		return r.IntN(3) * 3
	case "predator":
		if currentTurn%2 != 0 {
			return 0
		}
		return (r.IntN(4) + 2) * 3
	case "special", "babydragon":
		fallthrough
	default:
		switch currentTurn {
		case 1:
			return 7 * 3
		case 2:
			return 7 * 3
		default:
			movement := r.IntN(3) * 3
			if rand.IntN(100) < cfg.BabyDragonBuffPercent {
				movement += 1
			}
			return movement
		}
	}
}

// String returns a string representation of the race avatar.
func (avatar *Avatar) String() string {
	return avatar.Emoji
}

// LoadAvatars loads the avatar configuration from the specified YAML file path.
func LoadAvatars(path string) error {
	filePath := filepath.Join(path, "race/avatars.yaml")
	if err := config.LoadConfig(filePath, &defaultAvatars); err != nil {
		return err
	}

	return nil
}
