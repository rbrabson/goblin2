package disgobot

import (
	"goblin2/internal/discordid"
	"log/slog"
	"slices"
	"sync"

	"github.com/disgoorg/snowflake/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var serverMutex sync.Mutex

// seedOwnerFromGuild seeds the bootstrap owner from a guild's owner the first time the bot loads or
// joins a guild and no owners have been configured yet. It is a no-op once any owner exists, so
// later guilds and restarts leave the configured owners untouched.
func seedOwnerFromGuild(guildID, ownerID snowflake.ID) {
	server := GetServer()
	if server.HasOwners() {
		return
	}

	seeded, err := server.SeedOwner(discordid.NewSnowflakeID(ownerID))
	if err != nil {
		slog.Error("failed to seed bootstrap owner from guild owner",
			slog.Any("guildID", guildID),
			slog.Any("ownerID", ownerID),
			slog.Any("error", err),
		)
		return
	}
	if seeded {
		slog.Info("seeded bootstrap owner from guild owner",
			slog.Any("guildID", guildID),
			slog.Any("ownerID", ownerID),
		)
	}
}

// Server represents the owners and admins of the bot in the database.
type Server struct {
	ID     bson.ObjectID           `bson:"_id,omitempty"`
	Owners []discordid.SnowflakeID `bson:"owners"`
	Admins []discordid.SnowflakeID `bson:"admins"`
}

// GetServer retrieves the bot from the database.
func GetServer() *Server {
	server := readServer()
	if server == nil {
		server = createServer()
	}
	return server
}

// createServer creates a new server in the database and writes the newly created server to the database.
func createServer() *Server {
	server := &Server{
		Owners: []discordid.SnowflakeID{},
		Admins: []discordid.SnowflakeID{},
	}
	if err := writeServer(server); err != nil {
		slog.Error("failed to write server object",
			slog.Any("error", err),
		)
	}
	return server
}

// AddOwner adds a member as an owner of the server.
func (s *Server) AddOwner(memberID discordid.SnowflakeID) error {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if slices.Contains(s.Owners, memberID) {
		return ErrAlreadyOwner
	}
	s.Owners = append(s.Owners, memberID)
	return writeServer(s)
}

// SeedOwner designates memberID as the bootstrap owner when no owners are configured yet. It is
// idempotent: once any owner exists it is a no-op and reports seeded == false. This is used to seed
// the first owner from a Discord guild owner on the bot's first run.
func (s *Server) SeedOwner(memberID discordid.SnowflakeID) (seeded bool, err error) {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if len(s.Owners) > 0 {
		return false, nil
	}
	s.Owners = append(s.Owners, memberID)
	if err := writeServer(s); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveOwner removes a member as an owner of the server.
func (s *Server) RemoveOwner(memberID discordid.SnowflakeID) error {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if !slices.Contains(s.Owners, memberID) {
		return ErrNotOwner
	}
	s.Owners = slices.DeleteFunc(s.Owners, func(s discordid.SnowflakeID) bool {
		return s == memberID
	})
	return writeServer(s)
}

// ListOwners lists the owners of the server.
func (s *Server) ListOwners() []discordid.SnowflakeID {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return slices.Clone(s.Owners)
}

// AddAdmin adds a member as an admin of the server.
func (s *Server) AddAdmin(memberID discordid.SnowflakeID) error {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if slices.Contains(s.Admins, memberID) {
		return ErrAlreadyAdmin
	}
	s.Admins = append(s.Admins, memberID)
	return writeServer(s)
}

// RemoveAdmin removes a member as an admin of the server.
func (s *Server) RemoveAdmin(memberID discordid.SnowflakeID) error {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if !slices.Contains(s.Admins, memberID) {
		return ErrNotAdmin
	}
	s.Admins = slices.DeleteFunc(s.Admins, func(s discordid.SnowflakeID) bool {
		return s == memberID
	})
	return writeServer(s)
}

// ListAdmins lists the admins of the server.
func (s *Server) ListAdmins() []discordid.SnowflakeID {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return slices.Clone(s.Admins)
}

// HasOwners checks if the server has any owners.
func (s *Server) HasOwners() bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return len(s.Owners) > 0
}

// IsOwner checks if the given member is an owner of the server.
func (s *Server) IsOwner(memberID discordid.SnowflakeID) bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return slices.Contains(s.Owners, memberID)
}

// HasAdmins checks if the server has any admins.
func (s *Server) HasAdmins() bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return len(s.Admins) > 0
}

// IsAdmin checks if the given member is an admin of the server.
func (s *Server) IsAdmin(memberID discordid.SnowflakeID) bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return slices.Contains(s.Admins, memberID)
}

// CanManage checks if the given member can manage the server.
func (s *Server) CanManage(memberID discordid.SnowflakeID) bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return len(s.Owners) == 0 || slices.Contains(s.Owners, memberID) || slices.Contains(s.Admins, memberID)
}
