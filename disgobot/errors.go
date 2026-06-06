package disgobot

import "errors"

var (
	ErrTokenRequired    = errors.New("bot token is required")
	ErrClientNotCreated = errors.New("bot could not start")
	ErrMongoDBRequired  = errors.New("a MongoDB instance is required to start the bot")
)
