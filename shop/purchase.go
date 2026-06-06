package shop

import (
	"errors"
	"fmt"
	"goblin2/bank"
	"goblin2/discordid"
	"goblin2/guild"
	"goblin2/internal/disctime"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	PURCHASED = "purchased"
)

// Purchase is a purchase made from the shop.
type Purchase struct {
	ID          bson.ObjectID         `json:"_id,omitempty" bson:"_id,omitempty"`
	GuildID     discordid.SnowflakeID `json:"guild_id" bson:"guild_id"`
	MemberID    discordid.SnowflakeID `json:"member_id" bson:"member_id"`
	Item        *Item                 `json:"item" bson:"item,inline"`
	Status      string                `json:"status" bson:"status"`
	PurchasedOn time.Time             `json:"purchased_on" bson:"purchased_on"`
	ExpiresOn   time.Time             `json:"expires_on" bson:"expires_on"`
	AutoRenew   bool                  `json:"auto_renew" bson:"auto_renew"`
	IsExpired   bool                  `json:"is_expired" bson:"is_expired"`
	Version     int                   `json:"version" bson:"version"`
}

// GetAllPurchases returns all the purchases made by a member in the guild.
func GetAllPurchases(guildID string, memberID string) []*Purchase {
	purchases, err := readPurchases(guildID, memberID)
	if err != nil {
		slog.Error("unable to read purchases from the database", "guildID", guildID, "memberID", memberID, "error", err)
		return nil
	}

	cachePurchases(purchases)

	for _, purchase := range purchases {
		purchase.HasExpired()
	}

	purchaseCmp := func(a, b *Purchase) int {
		// Sort expired purchases to the bottom of the purchases
		if a.IsExpired && !b.IsExpired {
			return 1
		}
		if !a.IsExpired && b.IsExpired {
			return -1
		}

		// Sort on the basic purchase information
		if a.Item.Type < b.Item.Type {
			return -1
		}
		if a.Item.Type > b.Item.Type {
			return 1
		}
		if a.Item.Name < b.Item.Name {
			return -1
		}
		if a.Item.Name > b.Item.Name {
			return 1
		}
		if a.PurchasedOn.Before(b.PurchasedOn) {
			return -1
		}
		if a.PurchasedOn.After(b.PurchasedOn) {
			return 1
		}
		return 0
	}
	slices.SortFunc(purchases, purchaseCmp)

	return purchases
}

func getPurchase(guildID discordid.SnowflakeID, memberID discordid.SnowflakeID, itemName string, itemType string) *Purchase {
	key := purchaseCacheKey{
		guildID:   guildID,
		memberID:  memberID,
		itemName:  itemName,
		itemType:  itemType,
		isExpired: false,
	}

	if purchase, ok := purchaseCache.Get(key); ok {
		return copyPurchase(&purchase)
	}

	purchase, err := readPurchase(guildID, memberID, itemName, itemType)
	if err != nil || purchase == nil {
		return nil
	}

	purchaseCache.Set(key, *purchase)
	return copyPurchase(purchase)
}

func UpdatePurchase(purchase *Purchase, mutate func(*Purchase) error) error {
	const maxRetries = 3

	purchaseMu.RLock()
	defer purchaseMu.RUnlock()

	key := purchaseKey(purchase)

	for range maxRetries {
		latest := getPurchase(purchase.GuildID, purchase.MemberID, purchase.Item.Name, purchase.Item.Type)
		if latest == nil {
			latest = copyPurchase(purchase)
		}

		oldKey := purchaseKey(latest)

		if err := mutate(latest); err != nil {
			return err
		}

		var err error
		if latest.ID.IsZero() {
			err = writePurchase(latest)
		} else {
			err = updatePurchase(latest)
		}

		if err == nil {
			purchaseCache.Delete(oldKey)
			purchaseCache.Set(purchaseKey(latest), *latest)
			return nil
		}
		if !errors.Is(err, bank.ErrVersionConflict) {
			purchaseCache.Delete(key)
			return err
		}

		purchaseCache.Delete(key)

		slog.Warn("version conflict on shop purchase, retrying",
			slog.Any("guildID", purchase.GuildID),
			slog.Any("memberID", purchase.MemberID),
			slog.String("itemName", purchase.Item.Name),
			slog.String("itemType", purchase.Item.Type),
		)
	}

	return fmt.Errorf("failed to update shop purchase after %d retries: %w", maxRetries, bank.ErrVersionConflict)
}

// PurchaseItem creates a new Purchase with the given guild ID, member ID, and a purchasable
// shop item.
func PurchaseItem(guildID, memberID discordid.SnowflakeID, item *Item, status string, renew bool) (*Purchase, error) {
	p := message.NewPrinter(language.AmericanEnglish)

	member, _ := readMember(discordid.SnowflakeID(guildID), discordid.SnowflakeID(memberID))
	if member != nil && member.HasRestriction(showBan) {
		return nil, errors.New(p.Sprintf("you are banned from using the shop"))
	}

	bankAccount := bank.GetAccount(guildID, memberID)
	err := bankAccount.WithdrawFromCurrent(item.Price)
	if err != nil {
		slog.Debug("unable to withdraw cash from the bank account", "guildID", guildID, "memberID", memberID, "itemName", item.Name, "itemPrice", item.Price, "error", err)
		return nil, errors.New(p.Sprintf("insufficient funds to buy the %s `%s` for %d", item.Type, item.Name, item.Price))
	}

	purchase := &Purchase{
		GuildID:     guildID,
		MemberID:    memberID,
		Item:        item,
		Status:      status,
		PurchasedOn: time.Now(),
	}
	if item.AutoRenewable {
		purchase.AutoRenew = renew
	}
	if item.Duration != "" {
		duration, _ := disctime.ParseDuration(item.Duration)
		purchase.ExpiresOn = disctime.RoundToNextDay(time.Now().Add(duration))
	}
	err = writePurchase(purchase)
	if err != nil {
		slog.Error("unable to write purchase to the database", "guildID", guildID, "memberID", memberID, "itemName", item.Name, "itemType", item.Type, "error", err)
		// Refund the member
		if err := bankAccount.Deposit(item.Price); err != nil {
			slog.Error("unable to deposit cash from the bank account", "guildID", guildID, "memberID", memberID, "itemName", item.Name, "itemType", item.Type, "error", err)
		}
		return nil, fmt.Errorf("unable to write purchase to the database: %w", err)
	}
	slog.Info("creating new purchase", "guildID", guildID, "memberID", memberID, "itemName", item.Name, "itemType", item.Type)
	config := GetConfig(guildID)
	if config.ModChannelID != "" {
		guildMember, err := guild.GetMemberByID(guildID, memberID)
		if err != nil {
			slog.Error("unable to get guild member", "guildID", guildID, "memberID", memberID, "error", err)
			return nil, fmt.Errorf("unable to get guild member: %w", err)
		}

		if err := sendShopMessage(config.ModChannelID, p.Sprintf("`%s` (id=%s) purchased %s `%s` for %d", guildMember.Name, memberID, item.Type, item.Name, item.Price)); err != nil {
			slog.Error("unable to send message to mod channel", "guildID", guildID, "memberID", memberID, "itemName", item.Name, "itemType", item.Type, "error", err)
		}
	}

	return purchase, nil
}

// HasExpired determines if a purchase has expired. This marks the purchase as expired and undoes the effects of the purchase
// if it has expired.
func (p *Purchase) HasExpired() bool {
	if p.IsExpired {
		return true
	}

	oldIsExpired := p.IsExpired
	switch {
	case p.ExpiresOn.IsZero():
		return false
	case p.ExpiresOn.Before(time.Now().UTC()):
		switch p.Item.Type {
		case roleItemType:
			// Unassign the role to the user. If it can't be unassigned, log the error but don't mark it as expired
			// so that it can be retried later.
			guildMember, err := guild.GetMemberByID(p.GuildID, p.MemberID)
			if err != nil {
				slog.Error("failed to unassign role", "guildID", p.GuildID, "memberID", p.MemberID, "roleName", p.Item.Name, "error", err)
				return false
			}
			if err := guildMember.UnassignRole(client, p.Item.Name); err != nil {
				slog.Error("failed to unassign role", "guildID", p.GuildID, "memberID", p.MemberID, "roleName", p.Item.Name, "error", err)
				return false
			}
		default:
			slog.Warn("unknown purchase has expired", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type)
		}

		p.IsExpired = true
	default:
		return false
	}

	if p.IsExpired != oldIsExpired {
		if err := UpdatePurchase(p, func(latest *Purchase) error {
			latest.IsExpired = p.IsExpired
			return nil
		}); err != nil {
			slog.Error("unable to write purchase to the database", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type, "error", err)
		}

		g, err := client.Rest.GetGuild(p.GuildID.ID(), false)

		var msg string
		if err == nil && g.Name != "" {
			msg = fmt.Sprintf("Your purchase of %s `%s` on `%s` has expired", p.Item.Type, p.Item.Name, g.Name)
		} else {
			msg = fmt.Sprintf("Your purchase of %s `%s` has expired", p.Item.Type, p.Item.Name)
		}

		if err := sendDirectMessage(p.MemberID.ID(), msg); err != nil {
			slog.Error("unable to send direct message about expired purchase",
				slog.Any("guildID", p.GuildID),
				slog.Any("memberID", p.MemberID),
				slog.String("itemName", p.Item.Name),
				slog.String("itemType", p.Item.Type),
				slog.Any("error", err),
			)
		}

		config := GetConfig(p.GuildID)
		if config.ModChannelID != "" {
			guildMember, err := guild.GetMemberByID(p.GuildID, p.MemberID)
			memberName := p.MemberID.String()
			if err == nil && guildMember != nil {
				memberName = guildMember.Name
			}

			printer := message.NewPrinter(language.AmericanEnglish)
			if err := sendShopMessage(config.ModChannelID, printer.Sprintf("`%s` (id=%s) had their purchase of %s `%s` expire", memberName, p.MemberID, p.Item.Type, p.Item.Name)); err != nil {
				slog.Error("unable to send message to mod channel", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type, "error", err)
			}
			slog.Info("purchase has expired", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type)
		} else {
			slog.Warn("no mod channel configured to notify of expired purchase", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type)
		}
	}

	return p.IsExpired
}

// Return the purchase to the shop.
func (p *Purchase) Return() error {
	bankAccount := bank.GetAccount(p.GuildID, p.MemberID)
	err := bankAccount.DepositIntoCurrent(p.Item.Price)
	if err != nil {
		slog.Error("unable to deposit cash to the bank account", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type, "error", err)
		return fmt.Errorf("unable to deposit cash to the bank account: %w", err)
	}

	err = deletePurchase(p)
	if err != nil {
		slog.Error("unable to delete purchase from the database", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type, "error", err)
		return fmt.Errorf("unable to delete purchase from the database: %w", err)
	}

	config := GetConfig(p.GuildID)
	if config.ModChannelID != "" {
		guildMember, err := guild.GetMemberByID(p.GuildID, p.MemberID)
		if err != nil {
			slog.Error("unable to get guild member", "guildID", p.GuildID, "memberID", p.MemberID, "error", err)
			return fmt.Errorf("unable to get guild member: %w", err)
		}

		printer := message.NewPrinter(language.AmericanEnglish)
		if err := sendShopMessage(config.ModChannelID, printer.Sprintf("`%s` (id=%s) has returned the purchase of %s `%s`", guildMember.Name, p.MemberID, p.Item.Type, p.Item.Name)); err != nil {
			slog.Error("unable to send message to mod channel", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "itemType", p.Item.Type, "error", err)
		}
	}

	return nil
}

// Update updates the purchase with the given autoRenew value.
func (p *Purchase) Update(autoRenew bool) error {
	if p.AutoRenew == autoRenew {
		slog.Info("purchase already has the same autoRenew value", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "autoRenew", autoRenew)
		return fmt.Errorf("purchase already has the same autoRenew value")
	}

	err := UpdatePurchase(p, func(latest *Purchase) error {
		latest.AutoRenew = autoRenew
		return nil
	})
	if err != nil {
		slog.Error("unable to update purchase autorenew in the database", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "autoRenew", autoRenew, "error", err)
		return fmt.Errorf("unable to update purchase in the database: %w", err)
	}
	slog.Info("updated purchase autorenew", "guildID", p.GuildID, "memberID", p.MemberID, "itemName", p.Item.Name, "autoRenew", autoRenew)

	return nil
}

// checkForExpiredPurchases checks once a day to see if any purchases that may be expired have expired.
func checkForExpiredPurchases() {
	for {
		now := time.Now().UTC()
		filter := bson.D{
			{Key: "is_expired", Value: false},
			{Key: "$and", Value: bson.A{
				bson.D{{Key: "expires_on", Value: bson.D{{Key: "$ne", Value: time.Time{}}}}},
				bson.D{{Key: "expires_on", Value: bson.D{{Key: "$lte", Value: now}}}},
			}},
		}
		purchases, _ := readAllPurchases(filter)
		slog.Debug("checking for expired purchases", "filter", filter, "count", len(purchases))
		for _, purchase := range purchases {
			purchase.HasExpired()
		}

		// Wait until tomorrow to check again
		year, month, day := now.Date()
		tomorrow := time.Date(year, month, day+1, 0, 0, 0, 0, time.UTC)
		time.Sleep(time.Until(tomorrow))
	}
}

func sendShopMessage(channelID string, content string) error {
	if client == nil {
		return fmt.Errorf("discord client is nil")
	}

	parsedChannelID, err := strconv.ParseUint(channelID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid channel ID %q: %w", channelID, err)
	}

	_, err = client.Rest.CreateMessage(snowflake.ID(parsedChannelID), discord.MessageCreate{
		Content: content,
	})
	return err
}

func sendDirectMessage(memberID snowflake.ID, content string) error {
	if client == nil {
		return fmt.Errorf("discord client is nil")
	}

	channel, err := client.Rest.CreateDMChannel(memberID)
	if err != nil {
		return err
	}

	_, err = client.Rest.CreateMessage(channel.ID(), discord.MessageCreate{
		Content: content,
	})
	return err
}

// String returns a string representation of the purchase.
func (p *Purchase) String() string {
	sb := &strings.Builder{}

	sb.WriteString("Purchase{")
	sb.WriteString("GuildID: ")
	sb.WriteString(p.GuildID.String())
	sb.WriteString(", MemberID: ")
	sb.WriteString(p.MemberID.String())
	sb.WriteString(", Item: ")
	sb.WriteString(p.Item.String())
	sb.WriteString(", Status: ")
	sb.WriteString(p.Status)
	sb.WriteString(", PurchasedOn: ")
	sb.WriteString(p.PurchasedOn.Format(time.RFC3339))
	if !p.ExpiresOn.IsZero() {
		sb.WriteString(", ExpiresOn: ")
		sb.WriteString(p.ExpiresOn.Format(time.RFC3339))
		sb.WriteString(", AutoRenew: ")
		sb.WriteString(fmt.Sprintf("%v", p.AutoRenew))
		sb.WriteString(", IsExpired: ")
		sb.WriteString(fmt.Sprintf("%v", p.IsExpired))
	}

	return sb.String()
}
