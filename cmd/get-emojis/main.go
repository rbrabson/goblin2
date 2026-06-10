package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const (
	prodBotTokenEnv = "PROD_BOT_TOKEN"
	devBotTokenEnv  = "DEV_BOT_TOKEN"
	prodAppIDEnv    = "PROD_APP_ID"
	devAppIDEnv     = "DEV_APP_ID"
	outputFileEnv   = "EMOJI_OUTPUT_FILE"

	defaultOutputFile = "tmp/dev-emojis.txt"
)

type syncedEmoji struct {
	Name    string
	Mention string
}

func main() {
	prodBotToken := requiredEnv(prodBotTokenEnv)
	devBotToken := requiredEnv(devBotTokenEnv)
	prodAppID := requiredSnowflakeEnv(prodAppIDEnv)
	devAppID := requiredSnowflakeEnv(devAppIDEnv)

	outputFile := strings.TrimSpace(os.Getenv(outputFileEnv))
	if outputFile == "" {
		outputFile = defaultOutputFile
	}

	prodClient, err := disgo.New(prodBotToken)
	if err != nil {
		log.Fatalf("failed to create production Discord client: %v", err)
	}

	devClient, err := disgo.New(devBotToken)
	if err != nil {
		log.Fatalf("failed to create development Discord client: %v", err)
	}

	prodEmojis, err := prodClient.Rest.GetApplicationEmojis(prodAppID)
	if err != nil {
		log.Fatalf("failed to list production application emojis: %v", err)
	}

	devEmojis, err := devClient.Rest.GetApplicationEmojis(devAppID)
	if err != nil {
		log.Fatalf("failed to list development application emojis: %v", err)
	}

	log.Printf("found %d production application emojis", len(prodEmojis))
	log.Printf("found %d development application emojis before sync", len(devEmojis))

	devEmojiByName := make(map[string]discord.Emoji, len(devEmojis))
	for _, emoji := range devEmojis {
		if emoji.Name == "" {
			log.Printf("skipping development emoji with empty name: id=%s", emoji.ID)
			continue
		}

		devEmojiByName[emoji.Name] = emoji
	}

	createdCount := 0
	skippedCount := 0
	failedCount := 0

	for _, prodEmoji := range prodEmojis {
		if prodEmoji.Name == "" {
			log.Printf("skipping production emoji with empty name: id=%s", prodEmoji.ID)
			skippedCount++
			continue
		}

		if existingEmoji, ok := devEmojiByName[prodEmoji.Name]; ok {
			log.Printf("emoji already exists in development app: %s => %s", existingEmoji.Name, existingEmoji.Mention())
			skippedCount++
			continue
		}

		icon, err := downloadEmojiIcon(prodEmoji)
		if err != nil {
			log.Printf("failed to download emoji %s (%s): %v", prodEmoji.Name, prodEmoji.ID, err)
			failedCount++
			continue
		}

		createdEmoji, err := devClient.Rest.CreateApplicationEmoji(devAppID, discord.EmojiCreate{
			Name:  prodEmoji.Name,
			Image: *icon,
		})
		if err != nil {
			log.Printf("failed to create development emoji %s (%s): %v", prodEmoji.Name, prodEmoji.ID, err)
			failedCount++
			continue
		}

		log.Printf("created development emoji: %s => %s", createdEmoji.Name, createdEmoji.Mention())

		devEmojiByName[createdEmoji.Name] = *createdEmoji
		createdCount++

		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("sync attempted: created=%d already-existed-or-skipped=%d failed=%d", createdCount, skippedCount, failedCount)

	devEmojis, err = devClient.Rest.GetApplicationEmojis(devAppID)
	if err != nil {
		log.Fatalf("failed to verify development application emojis: %v", err)
	}

	log.Printf("found %d development application emojis after sync", len(devEmojis))

	synced, missing := verifySyncedEmojis(prodEmojis, devEmojis)
	if len(missing) > 0 {
		for _, emojiName := range missing {
			log.Printf("missing development emoji after sync: %s", emojiName)
		}

		log.Fatalf("development app is missing %d of %d production emojis", len(missing), countNamedEmojis(prodEmojis))
	}

	if err := writeEmojiMentions(outputFile, synced); err != nil {
		log.Fatalf("failed to write emoji output file: %v", err)
	}

	log.Printf("verified %d production emojis in development app", len(synced))
	log.Printf("wrote %d emoji mentions to %s", len(synced), outputFile)
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("missing required environment variable %s", name)
	}

	return value
}

func requiredSnowflakeEnv(name string) snowflake.ID {
	value := requiredEnv(name)

	id, err := snowflake.Parse(value)
	if err != nil {
		log.Fatalf("invalid snowflake in %s=%q: %v", name, value, err)
	}

	return id
}

func verifySyncedEmojis(prodEmojis []discord.Emoji, devEmojis []discord.Emoji) ([]syncedEmoji, []string) {
	devEmojiByName := make(map[string]discord.Emoji, len(devEmojis))
	for _, emoji := range devEmojis {
		if emoji.Name == "" {
			continue
		}

		devEmojiByName[emoji.Name] = emoji
	}

	synced := make([]syncedEmoji, 0, len(prodEmojis))
	missing := make([]string, 0)

	for _, prodEmoji := range prodEmojis {
		if prodEmoji.Name == "" {
			continue
		}

		devEmoji, ok := devEmojiByName[prodEmoji.Name]
		if !ok {
			missing = append(missing, prodEmoji.Name)
			continue
		}

		synced = append(synced, syncedEmoji{
			Name:    devEmoji.Name,
			Mention: devEmoji.Mention(),
		})
	}

	slices.Sort(missing)

	return synced, missing
}

func countNamedEmojis(emojis []discord.Emoji) int {
	count := 0
	for _, emoji := range emojis {
		if emoji.Name != "" {
			count++
		}
	}

	return count
}

func downloadEmojiIcon(emoji discord.Emoji) (*discord.Icon, error) {
	url := emoji.URL()

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("get %s: unexpected status %s", url, resp.Status)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	icon, err := discord.ParseIcon(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}

	return icon, nil
}

func writeEmojiMentions(path string, emojis []syncedEmoji) error {
	slices.SortFunc(emojis, func(a, b syncedEmoji) int {
		return strings.Compare(a.Name, b.Name)
	})

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	var out strings.Builder
	for _, emoji := range emojis {
		out.WriteString(emoji.Name)
		out.WriteString(": ")
		out.WriteString(emoji.Mention)
		out.WriteString("\n")
	}

	return os.WriteFile(path, []byte(out.String()), 0o644)
}
