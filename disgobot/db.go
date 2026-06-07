package disgobot

import (
	"goblin2/database"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	serverCollection = "goblin"
)

var (
	db *database.MongoDB
)

// readServer reads the server from the database
func readServer() *Server {
	filter := bson.M{}
	var server Server
	if err := db.FindOne(serverCollection, filter, &server); err != nil {
		slog.Debug("server not found in the database",
			slog.Any("error", err),
		)
		return nil
	}
	return &server
}

// writeServer writes the server to the database
func writeServer(server *Server) error {
	var filter bson.M
	if server.ID == bson.NilObjectID {
		filter = bson.M{}
	} else {
		filter = bson.M{"_id": server.ID}
	}
	if _, err := db.ReplaceOneUpsert(serverCollection, filter, server); err != nil {
		slog.Error("unable to save server to the database",
			slog.Any("filter", filter),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}
