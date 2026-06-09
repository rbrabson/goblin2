package channel

import "errors"

var (
	ErrNotInGuild        = errors.New("command must be used in a guild")
	ErrChannelNotInGuild = errors.New("channel must be a guild channel")
)
