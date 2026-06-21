package plugin

import (
	"goblin2/database"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// Status defines the status of the plugin
type Status int

const (
	Running Status = iota
	Stopping
	Stopped
)

// Plugin defines the packages registered to run on the system
type Plugin interface {
	Initialize(db *database.MongoDB, client *bot.Client)
	GetHelp() map[string]string
	GetName() string
	GetAdminHelp() map[string]string
	Stop()
	Status() Status
	GetSlashHandlers() map[string]handler.SlashCommandHandler
	GetComponentHandlers() map[string]handler.ComponentHandler
	GetSlashCommands() []discord.ApplicationCommandCreate
	GetEventListeners() []bot.EventListener
}

// String gets the string representation of the plugin status.
func (s Status) String() string {
	switch s {
	case Running:
		return "Running"
	case Stopping:
		return "Stopping"
	case Stopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}
