package disgobot

import (
	"goblin2/internal/discordid"
	"log/slog"
	"slices"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var serverMutex sync.Mutex

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

// CanManageOwners checks if the given member can manage the server owners.
func (s *Server) CanManageOwners(memberID discordid.SnowflakeID) bool {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	return len(s.Owners) == 0 || slices.Contains(s.Owners, memberID)
}
