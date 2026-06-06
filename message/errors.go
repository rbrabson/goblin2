package message

import "errors"

var (
	ErrClientMissing = errors.New("DiscordConfig.Client is required to create a message")
)
