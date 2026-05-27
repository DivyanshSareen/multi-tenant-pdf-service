package database

import (
	"context"
	"fmt"
	"time"

	"github.com/divyansh/multi-tenant-pdf-service/internal/models"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoClient wraps the official MongoDB driver client and exposes tenant-scoped operations.
type MongoClient struct {
	client *mongo.Client
	log    *logrus.Logger
}

// NewMongoClient connects to MongoDB and verifies the connection with a ping.
func NewMongoClient(uri string, log *logrus.Logger) (*MongoClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connecting to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("pinging mongodb: %w", err)
	}

	log.Info("connected to mongodb")
	return &MongoClient{client: client, log: log}, nil
}

// Ping checks the MongoDB connection.
func (m *MongoClient) Ping(ctx context.Context) error {
	return m.client.Ping(ctx, nil)
}

// Disconnect closes the MongoDB connection gracefully.
func (m *MongoClient) Disconnect(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

// CreateTenantDB initialises the tenant's database by creating the "documents" collection
// and an index on uploaded_at. MongoDB creates the database lazily on first write,
// so we force it by creating the collection explicitly.
func (m *MongoClient) CreateTenantDB(ctx context.Context, dbName string) error {
	db := m.client.Database(dbName)

	// CreateCollection returns an error if the collection already exists; ignore that.
	_ = db.CreateCollection(ctx, "documents")

	collection := db.Collection("documents")
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "uploaded_at", Value: -1}},
		Options: options.Index().SetName("idx_uploaded_at"),
	}
	if _, err := collection.Indexes().CreateOne(ctx, indexModel); err != nil {
		return fmt.Errorf("creating index on %s.documents: %w", dbName, err)
	}

	m.log.WithField("db", dbName).Info("initialised tenant mongodb database")
	return nil
}

// InsertDocument stores a Document in the tenant's documents collection and returns the inserted ID.
func (m *MongoClient) InsertDocument(ctx context.Context, dbName string, doc *models.Document) (string, error) {
	collection := m.client.Database(dbName).Collection("documents")

	doc.UploadedAt = time.Now().UTC()

	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("inserting document into %s: %w", dbName, err)
	}

	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return fmt.Sprintf("%v", result.InsertedID), nil
	}
	return id.Hex(), nil
}

// GetDocuments returns all documents stored in the tenant's collection, newest first.
func (m *MongoClient) GetDocuments(ctx context.Context, dbName string) ([]models.Document, error) {
	collection := m.client.Database(dbName).Collection("documents")

	opts := options.Find().SetSort(bson.D{{Key: "uploaded_at", Value: -1}})
	cursor, err := collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("querying documents in %s: %w", dbName, err)
	}
	defer cursor.Close(ctx)

	var docs []models.Document
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decoding documents from %s: %w", dbName, err)
	}
	return docs, nil
}

// DropDatabase drops the entire tenant database, permanently deleting all documents.
func (m *MongoClient) DropDatabase(ctx context.Context, dbName string) error {
	if err := m.client.Database(dbName).Drop(ctx); err != nil {
		return fmt.Errorf("dropping database %s: %w", dbName, err)
	}
	m.log.WithField("db", dbName).Info("dropped tenant mongodb database")
	return nil
}
