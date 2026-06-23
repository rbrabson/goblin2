package leaderboard

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/internal/cache"
	"goblin2/internal/discordid"
	"goblin2/internal/disctime"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	leaderboardCacheTTL             = 30 * time.Minute
	leaderboardCacheCleanupInterval = 5 * time.Minute
)

type leaderboardCacheKey struct {
	guildID discordid.SnowflakeID
}

var (
	leaderboardCache = cache.New[leaderboardCacheKey, Leaderboard](leaderboardCacheTTL, leaderboardCacheCleanupInterval)
	leaderboardMu    sync.RWMutex
)

// A Leaderboard is used to send a monthly leaderboard to the Discord server for each guild.
type Leaderboard struct {
	ID         bson.ObjectID         `bson:"_id,omitempty"`
	GuildID    discordid.SnowflakeID `bson:"guild_id"`
	ChannelID  string                `bson:"channel_id"`
	LastSeason time.Time             `bson:"last_season"`
	Version    int                   `bson:"version"`
}

// getLeaderboard returns the leaderboard for the given guild.
func getLeaderboard(guildID discordid.SnowflakeID) *Leaderboard {
	key := leaderboardCacheKey{
		guildID: guildID,
	}

	if lb, ok := leaderboardCache.Get(key); ok {
		return copyLeaderboard(&lb)
	}

	lb := readLeaderboard(key.guildID)
	if lb != nil {
		leaderboardCache.Set(key, *lb)
		return copyLeaderboard(lb)
	}

	lb = createLeaderboard(guildID)
	leaderboardCache.Set(key, *lb)
	return copyLeaderboard(lb)
}

// createLeaderboard creates a new leaderboard for the given guildID and sets the last season to the current month.
func createLeaderboard(guildID discordid.SnowflakeID) *Leaderboard {
	lb := &Leaderboard{
		GuildID:    guildID,
		LastSeason: disctime.CurrentMonth(time.Now()),
	}
	slog.Info("created new leaderboard",
		slog.Any("guild_id", guildID),
	)

	err := writeLeaderboard(lb)
	if err != nil {
		slog.Error("unable to write leaderboard to database",
			slog.Any("guildID", guildID),
			slog.Any("error", err),
		)
	}

	return lb
}

// copyLeaderboard returns a copy of the given leaderboard. This prevents callers from mutating the cached leaderboard directly.
func copyLeaderboard(lb *Leaderboard) *Leaderboard {
	if lb == nil {
		return nil
	}

	return new(*lb)
}

// CloseLeaderboardCache stops the leaderboard cache cleanup goroutine and clears all cached leaderboard entries.
func CloseLeaderboardCache() {
	leaderboardCache.Destroy()
}

// getLeaderboards returns all the leaderboards for all guilds known to the bot.
func getLeaderboards() []*Leaderboard {
	var leaderboards []*Leaderboard
	filter := bson.M{
		"guild_id": bson.M{
			"$exists": true,
			"$ne":     "",
		},
	}
	err := db.FindMany(leaderboardCollection, filter, &leaderboards, bson.D{}, 0)
	if err != nil {
		slog.Error("unable to get leaderboards", "error", err)
		return nil
	}

	for _, lb := range leaderboards {
		key := leaderboardCacheKey{
			guildID: lb.GuildID,
		}
		leaderboardCache.Set(key, *lb)
	}

	slog.Debug("leaderboards", "count", len(leaderboards))
	return leaderboards
}

// UpdateLeaderboard updates the leaderboard with the given mutation, retrying on version conflicts.
func UpdateLeaderboard(guildID discordid.SnowflakeID, mutate func(*Leaderboard) error) error {
	const maxRetries = 3

	leaderboardMu.RLock()
	defer leaderboardMu.RUnlock()

	key := leaderboardCacheKey{
		guildID: guildID,
	}

	for range maxRetries {
		lb := getLeaderboard(guildID)

		if err := mutate(lb); err != nil {
			return err
		}

		var err error
		if lb.ID.IsZero() {
			err = writeLeaderboard(lb)
		} else {
			err = updateLeaderboard(lb)
		}

		if err == nil {
			leaderboardCache.Set(key, *lb)
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			leaderboardCache.Delete(key)
			return err
		}

		leaderboardCache.Delete(key)

		slog.Warn("version conflict on leaderboard, retrying",
			slog.Any("guildID", guildID),
		)
	}

	return fmt.Errorf("failed to update leaderboard after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// setChannel sets the channel ID for the leaderboard to publish the monthly leaderboard.
func (lb *Leaderboard) setChannel(channelID string) {
	if err := UpdateLeaderboard(lb.GuildID, func(latest *Leaderboard) error {
		latest.ChannelID = channelID
		return nil
	}); err != nil {
		slog.Error("error writing leaderboard", "guild", lb.GuildID, "error", err)
	}
}

// GetCurrentRanking returns the global rankings based on the current balance.
func (lb *Leaderboard) getCurrentLeaderboard() []*bank.Account {
	filter := bson.D{{Key: "guild_id", Value: lb.GuildID}}
	sort := bson.D{{Key: "current_balance", Value: -1}, {Key: "_id", Value: 1}}
	limit := int64(10)

	accounts := bank.GetAccounts(filter, sort, limit)
	slices.SortFunc(accounts, func(a, b *bank.Account) int {
		switch {
		case a.CurrentBalance > b.CurrentBalance:
			return -1
		case a.CurrentBalance < b.CurrentBalance:
			return 1
		case a.MemberID < b.MemberID:
			return -1
		default:
			return 1
		}
	})

	return accounts
}

// getMonthlyLeaderboard returns the global rankings based on the monthly balance.
func (lb *Leaderboard) getMonthlyLeaderboard() []*bank.Account {
	filter := bson.D{{Key: "guild_id", Value: lb.GuildID}}
	sort := bson.D{{Key: "monthly_balance", Value: -1}, {Key: "_id", Value: 1}}
	limit := int64(10)

	accounts := bank.GetAccounts(filter, sort, limit)
	slices.SortFunc(accounts, func(a, b *bank.Account) int {
		switch {
		case a.MonthlyBalance > b.MonthlyBalance:
			return -1
		case a.MonthlyBalance < b.MonthlyBalance:
			return 1
		case a.MemberID < b.MemberID:
			return -1
		default:
			return 1
		}
	})

	return accounts
}

// getLifetimeLeaderboard returns the global rankings based on the monthly balance.
func (lb *Leaderboard) getLifetimeLeaderboard() []*bank.Account {
	filter := bson.D{{Key: "guild_id", Value: lb.GuildID}}
	sort := bson.D{{Key: "lifetime_balance", Value: -1}, {Key: "_id", Value: 1}}
	limit := int64(10)

	accounts := bank.GetAccounts(filter, sort, limit)
	slices.SortFunc(accounts, func(a, b *bank.Account) int {
		switch {
		case a.LifetimeBalance > b.LifetimeBalance:
			return -1
		case a.LifetimeBalance < b.LifetimeBalance:
			return 1
		case a.MemberID < b.MemberID:
			return -1
		default:
			return 1
		}
	})

	return accounts
}

// sendMonthlyLeaderboard publishes the monthly leaderboard to the bank channel.
func sendMonthlyLeaderboard(client *bot.Client, lb *Leaderboard) error {
	// Get the top 10 accounts for this month
	accounts := lb.getMonthlyLeaderboard()
	leaderboardSize := min(10, len(accounts))
	accounts = accounts[:leaderboardSize]

	firstOfMonth := disctime.PreviousMonth(time.Now())
	year, month, _ := firstOfMonth.Date()
	if lb.ChannelID != "" {
		channelID, err := strconv.ParseUint(lb.ChannelID, 10, 64)
		if err != nil {
			return fmt.Errorf("%w %q: %v", errInvalidChannel, lb.ChannelID, err)
		}

		p := message.NewPrinter(language.AmericanEnglish)
		embeds := formatAccounts(client, lb.GuildID, p, fmt.Sprintf("%s %d Top 10", month, year), accounts)
		_, err = client.Rest.CreateMessage(snowflake.ID(channelID), discord.MessageCreate{
			Embeds: embeds,
		})
		if err != nil {
			return err
		}
	} else {
		slog.Warn("no leaderboard channel set for server", "guildID", lb.GuildID, "channelID", lb.ChannelID)
	}
	for _, account := range accounts {
		slog.Debug("sent monthly leaderboard", "guildID", lb.GuildID, "memberID", account.MemberID, "monthlyBalance", account.MonthlyBalance)
	}
	slog.Info("sent monthly leaderboard", "guildID", lb.GuildID, "channelID", lb.ChannelID, "leaderboardSize", leaderboardSize)
	return nil
}

// publishMonthlyLeaderboard sends the monthly leaderboard to each guild.
func sendAllMonthlyLeaderboards(client *bot.Client) {
	// Get the last season for the banks, defaulting to the current month if there are no banks.
	// This handles the off-chance that the server crashed and a new month starts before the
	// server is restarted.
	lastSeason := disctime.CurrentMonth(time.Now())
	leaderboards := getLeaderboards()
	for _, lb := range leaderboards {
		if lb.LastSeason.Before(lastSeason) {
			lastSeason = lb.LastSeason
		}
	}

	for {
		nextMonth := disctime.NextMonth(lastSeason)
		if nextMonth.After(time.Now()) {
			time.Sleep(time.Until(nextMonth))
		}

		leaderboards := getLeaderboards()
		for _, lb := range leaderboards {
			if !lb.LastSeason.Before(nextMonth) {
				continue
			}

			switch publishMonthlyLeaderboard(client, lb) {
			case publishSucceeded:
				// Only clear the guild's monthly balance after its leaderboard has been
				// published, and only advance LastSeason once the balance is cleared. If the
				// reset fails we leave LastSeason untouched so the next cycle republishes
				// (a duplicate message) rather than wrongly clearing or skipping the guild.
				if err := bank.ResetMonthlyBalances(lb.GuildID); err != nil {
					slog.Error("published leaderboard but unable to reset monthly balances; will retry next cycle",
						"guildID", lb.GuildID, "channelID", lb.ChannelID, "error", err)
					break
				}
				advanceLeaderboardSeason(lb, nextMonth)
			case publishAbandoned:
				// Permanent failure (bot removed, channel deleted, bad config): stop retrying
				// this guild and leave its monthly balance untouched.
				advanceLeaderboardSeason(lb, nextMonth)
			}
		}

		lastSeason = nextMonth
	}
}

const (
	// publishRetryBaseDelay is the initial backoff between publish attempts; it doubles
	// after each failure up to publishRetryMaxDelay.
	publishRetryBaseDelay = 1 * time.Second
	// publishRetryMaxDelay caps the backoff so recovery after a long Discord outage is
	// picked up promptly.
	publishRetryMaxDelay = 5 * time.Minute
)

// errInvalidChannel indicates the configured leaderboard channel cannot be parsed. This is a
// permanent configuration error that retrying will never fix.
var errInvalidChannel = errors.New("invalid leaderboard channel ID")

// publishResult is the disposition of an attempt to publish a guild's monthly leaderboard.
type publishResult int

const (
	publishSucceeded publishResult = iota // published; clear the balance and advance the season
	publishAbandoned                      // permanent failure; advance the season but never retry
)

// errorKind classifies why a publish attempt failed, which determines how it is retried.
type errorKind int

const (
	errPermanent   errorKind = iota // unrecoverable without intervention (kicked, channel gone, bad config)
	errDiscordDown                  // Discord/connectivity is down; wait for it to recover
)

// classifyError determines how a failure from sendMonthlyLeaderboard should be handled.
func classifyError(err error) errorKind {
	// A malformed channel configuration cannot be fixed by retrying.
	if errors.Is(err, errInvalidChannel) {
		return errPermanent
	}

	var restErr *rest.Error
	if errors.As(err, &restErr) {
		// Conditions the bot can never recover from on its own: it has been removed from
		// the guild, the channel was deleted, or it lacks permission to post there.
		if rest.IsJSONErrorCode(err,
			rest.JSONErrorCodeUnknownChannel,
			rest.JSONErrorCodeUnknownGuild,
			rest.JSONErrorCodeMissingAccess,
			rest.JSONErrorCodeLackPermissionsToPerformAction,
		) {
			return errPermanent
		}
		if restErr.Response != nil {
			switch sc := restErr.Response.StatusCode; {
			case sc == http.StatusForbidden, sc == http.StatusNotFound:
				return errPermanent
			case sc == http.StatusTooManyRequests, sc >= 500:
				return errDiscordDown
			}
		}
		// Any other error Discord returns is a request we cannot fix by retrying.
		return errPermanent
	}

	// Not a Discord REST error: a connection-level failure reaching Discord. Treat it the
	// same as Discord being down and wait for connectivity to recover.
	return errDiscordDown
}

// publishMonthlyLeaderboard publishes a single guild's leaderboard, retrying transient
// failures with exponential backoff. It blocks until the publish succeeds or the failure is
// determined to be permanent; Discord or connectivity outages are waited out indefinitely.
func publishMonthlyLeaderboard(client *bot.Client, lb *Leaderboard) publishResult {
	delay := publishRetryBaseDelay
	for attempt := 1; ; attempt++ {
		err := sendMonthlyLeaderboard(client, lb)
		if err == nil {
			return publishSucceeded
		}

		switch classifyError(err) {
		case errPermanent:
			slog.Error("giving up on monthly leaderboard; guild or channel is unreachable",
				"guildID", lb.GuildID, "channelID", lb.ChannelID, "attempt", attempt, "error", err)
			return publishAbandoned
		case errDiscordDown:
			// Discord or connectivity to it is down; keep waiting for it to recover rather
			// than abandoning the guild for the month.
			slog.Warn("Discord unavailable while sending monthly leaderboard, waiting to retry",
				"guildID", lb.GuildID, "channelID", lb.ChannelID, "attempt", attempt, "error", err)
		}

		time.Sleep(delay)
		if delay *= 2; delay > publishRetryMaxDelay {
			delay = publishRetryMaxDelay
		}
	}
}

// advanceLeaderboardSeason records that the guild's leaderboard has been handled for the
// given season so the publishing loop does not select it again until the next month.
func advanceLeaderboardSeason(lb *Leaderboard, season time.Time) {
	if err := UpdateLeaderboard(lb.GuildID, func(latest *Leaderboard) error {
		latest.LastSeason = season
		return nil
	}); err != nil {
		slog.Error("unable to write leaderboard to database",
			"guildID", lb.GuildID, "channelID", lb.ChannelID, "error", err)
	}
}

// String returns a string representation of the Leaderboard.
func (lb *Leaderboard) String() string {
	return fmt.Sprintf("Leaderboard{ID=%s, GuildID=%s, ChannelID=%s, LastSeason=%s}",
		lb.ID.Hex(),
		lb.GuildID,
		lb.ChannelID,
		lb.LastSeason,
	)
}
