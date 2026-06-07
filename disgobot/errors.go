package disgobot

import "errors"

var (
	ErrTokenRequired    = errors.New("bot token is required")
	ErrClientNotCreated = errors.New("bot could not start")
	ErrMongoDBRequired  = errors.New("a MongoDB instance is required to start the bot")
	ErrAlreadyAdmin     = errors.New("you are already an admin")
	ErrAlreadyOwner     = errors.New("you are already an owner")
	ErrNotAdmin         = errors.New("you are not an admin")
	ErrNotOwner         = errors.New("you are not an owner")
)
