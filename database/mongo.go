package database

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	dbTimeout = 10 * time.Second
)

// MongoDB represents a connection to a mongo database
type MongoDB struct {
	client     *mongo.Client
	clientOpts *options.ClientOptions
	dbname     string
	uri        string
}

// New creates a database to load and save documents in a MongoDB database.
func New(dbName, dbURL string) (*MongoDB, error) {
	cfg, err := LoadConfig(dbName, dbURL)
	if err != nil {
		slog.Error("unable to load database config",
			slog.Any("error", err))
		return nil, err
	}

	m := &MongoDB{
		uri:    cfg.URI,
		dbname: cfg.Database,
	}

	// Wait for MongoDB to become active before proceeding
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m.clientOpts = options.Client().ApplyURI(m.uri)
	m.client, err = mongo.Connect(m.clientOpts)
	if err != nil {
		slog.Error("unable to connect to the MongoDB database",
			slog.String("database", m.dbname),
			slog.Any("error", err),
		)
		return nil, err
	}

	// Check the connection
	err = m.client.Ping(ctx, nil)
	if err != nil {
		slog.Error("unable to ping the MongoDB database", "uri", m.uri, "database", m.dbname, "error", err)
		if disconnectErr := m.client.Disconnect(context.Background()); disconnectErr != nil {
			slog.Error("unable to disconnect MongoDB client after failed ping",
				slog.Any("error", disconnectErr),
			)
		}
		return nil, err
	}

	return m, nil
}

// Close closes the database connection.
func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if m == nil || m.client == nil {
		return nil
	}

	return m.client.Disconnect(ctx)
}

// Count returns the number of documents in the collection that match the filter.
func (m *MongoDB) Count(collectionName string, filter any) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return 0, err
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		slog.Debug("unable to count the documents",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("error", err),
		)
		return 0, err
	}

	return count, nil
}

// Distinct returns the distinct values for fieldName in the collection that match the filter.
func (m *MongoDB) Distinct(collectionName string, fieldName string, filter any) ([]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res := collection.Distinct(ctx, fieldName, filter)
	if res.Err() != nil {
		slog.Debug("unable to get distinct values",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.String("field", fieldName),
			slog.Any("filter", filter),
			slog.Any("error", res.Err()),
		)
		return nil, res.Err()
	}

	var values []any
	if err := res.Decode(&values); err != nil {
		slog.Error("unable to decode distinct values",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.String("field", fieldName),
			slog.Any("error", err),
		)
		return nil, ErrInvalidDocument
	}

	return values, nil
}

// FindOne loads a document identified by documentID from the collection into data.
func (m *MongoDB) FindOne(collectionName string, filter any, data any) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		slog.Error("unable to get the collection",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("error", err),
		)
		return err
	}

	res := collection.FindOne(ctx, filter)
	if res.Err() != nil {
		slog.Error("error finding the document", // TODO: change back to Debug
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("error", res.Err()),
		)
		return res.Err()
	}
	if err := res.Decode(data); err != nil {
		slog.Error("error decoding the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("error", err),
			slog.Any("data", data),
		)
		return ErrInvalidDocument
	}
	return nil
}

// FindMany reads all documents from the database that match the filter into data.
// data must be a pointer to a slice.
func (m *MongoDB) FindMany(collectionName string, filter any, data any, sortBy any, limit int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return err
	}

	// Limit the number of documents to return
	findOptions := options.Find()
	if sortBy != nil {
		findOptions.SetSort(sortBy)
	}
	if limit > 0 {
		findOptions.SetLimit(limit)
	}

	cur, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		slog.Debug("unable to find the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("error", err),
		)
		return err
	}
	defer func() {
		if err := cur.Close(ctx); err != nil {
			slog.Error("failed to close the mongodb cursor",
				slog.String("database", m.dbname),
				slog.String("collection", collectionName),
				slog.Any("error", err),
			)
		}
	}()
	err = cur.All(ctx, data)
	if err != nil {
		slog.Error("unable to decode the documents",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("data", data),
			slog.Any("error", err),
		)
		return ErrInvalidDocument
	}

	return nil
}

// InsertOne inserts a single document into the collection.
func (m *MongoDB) InsertOne(collectionName string, document any) (*mongo.InsertOneResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.InsertOne(ctx, document)
	if err != nil {
		slog.Debug("unable to insert the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("document", document),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// InsertMany inserts multiple documents into the collection.
func (m *MongoDB) InsertMany(collectionName string, documents []any) (*mongo.InsertManyResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.InsertMany(ctx, documents)
	if err != nil {
		slog.Debug("unable to insert documents",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Int("documentCount", len(documents)),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// UpdateOne updates a single document in the collection that matches the filter.
func (m *MongoDB) UpdateOne(collectionName string, filter any, update any) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		slog.Debug("unable to update the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("update", update),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// UpdateOneUpsert updates a single document in the collection that matches the filter
// or inserts one if no document matches.
func (m *MongoDB) UpdateOneUpsert(collectionName string, filter any, update any) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	opts := options.UpdateOne().SetUpsert(true)

	res, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		slog.Debug("unable to upsert the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("update", update),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// ReplaceOne replaces a single document in the collection that matches the filter.
func (m *MongoDB) ReplaceOne(collectionName string, filter any, replacement any) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.ReplaceOne(ctx, filter, replacement)
	if err != nil {
		slog.Debug("unable to replace the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("replacement", replacement),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// ReplaceOneUpsert replaces a single document in the collection that matches the filter
// or inserts one if no document matches.
func (m *MongoDB) ReplaceOneUpsert(collectionName string, filter any, replacement any) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	opts := options.Replace().SetUpsert(true)

	res, err := collection.ReplaceOne(ctx, filter, replacement, opts)
	if err != nil {
		slog.Debug("unable to replace or insert the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("replacement", replacement),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// UpdateMany updates all documents in the collection that match the filter.
func (m *MongoDB) UpdateMany(collectionName string, filter any, update any) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		slog.Debug("unable to update documents",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("update", update),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// DeleteOne deletes a single document in the collection that matches the filter.
func (m *MongoDB) DeleteOne(collectionName string, filter any) (*mongo.DeleteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		slog.Debug("unable to delete the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// DeleteMany deletes all documents in the collection that match the filter.
func (m *MongoDB) DeleteMany(collectionName string, filter any) (*mongo.DeleteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	res, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		slog.Debug("unable to delete documents",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("error", err),
		)
		return nil, err
	}

	return res, nil
}

// FindOneAndUpdate updates one document and decodes the returned document into data.
// data must be a pointer. By default, MongoDB returns the document before the update.
func (m *MongoDB) FindOneAndUpdate(collectionName string, filter any, update any, data any) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return err
	}

	res := collection.FindOneAndUpdate(ctx, filter, update)
	if res.Err() != nil {
		slog.Debug("unable to find and update the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("update", update),
			slog.Any("error", res.Err()),
		)
		return res.Err()
	}

	if err := res.Decode(data); err != nil {
		slog.Error("unable to decode find and update result",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("data", data),
			slog.Any("error", err),
		)
		return ErrInvalidDocument
	}

	return nil
}

// FindOneAndUpdateAfter updates one document and decodes the updated document into data.
// data must be a pointer.
func (m *MongoDB) FindOneAndUpdateAfter(collectionName string, filter any, update any, data any) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return err
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	res := collection.FindOneAndUpdate(ctx, filter, update, opts)
	if res.Err() != nil {
		slog.Debug("unable to find and update the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("update", update),
			slog.Any("error", res.Err()),
		)
		return res.Err()
	}

	if err := res.Decode(data); err != nil {
		slog.Error("unable to decode find and update result",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("data", data),
			slog.Any("error", err),
		)
		return ErrInvalidDocument
	}

	return nil
}

// FindOneAndDelete deletes one document and decodes the deleted document into data.
// data must be a pointer.
func (m *MongoDB) FindOneAndDelete(collectionName string, filter any, data any) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return err
	}

	res := collection.FindOneAndDelete(ctx, filter)
	if res.Err() != nil {
		slog.Debug("unable to find and delete the document",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("filter", filter),
			slog.Any("error", res.Err()),
		)
		return res.Err()
	}

	if err := res.Decode(data); err != nil {
		slog.Error("unable to decode find and delete result",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("data", data),
			slog.Any("error", err),
		)
		return ErrInvalidDocument
	}

	return nil
}

// Aggregate performs an aggregation operation on the specified collection using the provided pipeline.
func (m *MongoDB) Aggregate(collectionName string, pipeline mongo.Pipeline) ([]bson.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	collection, err := m.getCollection(collectionName)
	if err != nil {
		return nil, err
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		slog.Error("unable to perform aggregation on the collection",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
			slog.Any("pipeline", pipeline),
			slog.Any("error", err),
		)
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			slog.Error("failed to close the mongodb cursor",
				slog.String("database", m.dbname),
				slog.String("collection", collectionName),
				slog.Any("error", err),
			)
		}
	}()

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// getCollection returns a collection from the database that may be used for database operations.
func (m *MongoDB) getCollection(collectionName string) (*mongo.Collection, error) {
	if m.client == nil {
		var err error
		m.clientOpts = options.Client().ApplyURI(m.uri)
		m.client, err = mongo.Connect(m.clientOpts)
		if err != nil {
			slog.Error("unable to connect to the MongoDB database",
				slog.String("database", m.dbname),
				slog.Any("error", err),
			)
			return nil, err
		}
	}

	db := m.client.Database(m.dbname)
	collection := db.Collection(collectionName)

	if collection == nil {
		slog.Error("unable to access the collection",
			slog.String("database", m.dbname),
			slog.String("collection", collectionName),
		)
		return nil, ErrCollectionNotAccessible
	}

	return collection, nil
}
