//go:build integration
// +build integration

package mongodb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	mdbconfig "github.com/wombatwisdom/wombat/public/components/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupMongoDBContainer(t *testing.T, ctx context.Context) string {
	t.Helper()

	// Start MongoDB container
	mongodbContainer, err := mongodb.Run(ctx,
		"mongo:7.0",
		mongodb.WithUsername("root"),
		mongodb.WithPassword("password"),
	)
	require.NoError(t, err, "Failed to start MongoDB container")

	t.Cleanup(func() {
		if err := mongodbContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate MongoDB container: %v", err)
		}
	})

	// Get the connection string
	connectionString, err := mongodbContainer.ConnectionString(ctx)
	require.NoError(t, err, "Failed to get connection string")

	// Wait for MongoDB to be ready
	opts := options.Client().ApplyURI(connectionString)
	client, err := mongo.Connect(ctx, opts)
	require.NoError(t, err, "Failed to connect to MongoDB")
	defer client.Disconnect(ctx)

	// Ping to ensure connection is established
	err = client.Ping(ctx, nil)
	require.NoError(t, err, "Failed to ping MongoDB")

	return connectionString
}

func TestMongoDBConfigIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mongoURI := setupMongoDBContainer(t, ctx)

	t.Run("Config.NewClient", func(t *testing.T) {
		t.Run("should connect to real MongoDB instance", func(t *testing.T) {
			config := mdbconfig.Config{
				Uri: mongoURI,
			}

			client, err := config.NewClient(ctx)
			require.NoError(t, err, "Failed to create client")
			require.NotNil(t, client)

			// Verify connection is working
			err = client.Ping(ctx, nil)
			assert.NoError(t, err, "Failed to ping MongoDB")

			// List databases to ensure connection works
			databases, err := client.ListDatabaseNames(ctx, bson.M{})
			assert.NoError(t, err, "Failed to list databases")
			assert.NotEmpty(t, databases, "Should have at least system databases")

			// Clean up
			err = client.Disconnect(ctx)
			assert.NoError(t, err, "Failed to disconnect")
		})

		t.Run("should perform CRUD operations", func(t *testing.T) {
			config := mdbconfig.Config{
				Uri: mongoURI,
			}

			client, err := config.NewClient(ctx)
			require.NoError(t, err, "Failed to create client")
			defer client.Disconnect(ctx)

			// Create a test database and collection
			db := client.Database("test_db")
			collection := db.Collection("test_collection")

			// Insert
			doc := bson.M{
				"name":  "integration test",
				"value": 42,
				"tags":  []string{"test", "integration"},
			}
			insertResult, err := collection.InsertOne(ctx, doc)
			require.NoError(t, err, "Failed to insert document")
			require.NotNil(t, insertResult.InsertedID)

			// Find
			var result bson.M
			err = collection.FindOne(ctx, bson.M{"name": "integration test"}).Decode(&result)
			require.NoError(t, err, "Failed to find document")
			assert.Equal(t, "integration test", result["name"])
			assert.Equal(t, int32(42), result["value"]) // BSON numbers are int32 by default

			// Update
			updateResult, err := collection.UpdateOne(
				ctx,
				bson.M{"_id": insertResult.InsertedID},
				bson.M{"$set": bson.M{"value": 100}},
			)
			require.NoError(t, err, "Failed to update document")
			assert.Equal(t, int64(1), updateResult.ModifiedCount)

			// Verify update
			err = collection.FindOne(ctx, bson.M{"_id": insertResult.InsertedID}).Decode(&result)
			require.NoError(t, err)
			assert.Equal(t, int32(100), result["value"])

			// Delete
			deleteResult, err := collection.DeleteOne(ctx, bson.M{"_id": insertResult.InsertedID})
			require.NoError(t, err, "Failed to delete document")
			assert.Equal(t, int64(1), deleteResult.DeletedCount)

			// Verify deletion
			err = collection.FindOne(ctx, bson.M{"_id": insertResult.InsertedID}).Decode(&result)
			assert.ErrorIs(t, err, mongo.ErrNoDocuments)
		})

		t.Run("should handle authentication", func(t *testing.T) {
			// Test with correct credentials (already included in connection string)
			config := mdbconfig.Config{
				Uri: mongoURI,
			}

			client, err := config.NewClient(ctx)
			require.NoError(t, err, "Failed to create client with auth")
			
			err = client.Ping(ctx, nil)
			assert.NoError(t, err, "Failed to ping with auth")
			
			client.Disconnect(ctx)

			// Test with invalid credentials
			invalidConfig := mdbconfig.Config{
				Uri: "mongodb://wronguser:wrongpass@localhost:27017/",
			}

			invalidClient, err := invalidConfig.NewClient(ctx)
			if err == nil {
				// Connection might succeed but operations should fail
				err = invalidClient.Ping(ctx, nil)
				assert.Error(t, err, "Expected error with invalid credentials")
				invalidClient.Disconnect(ctx)
			}
		})

		t.Run("should handle connection pooling", func(t *testing.T) {
			config := mdbconfig.Config{
				Uri: mongoURI + "?maxPoolSize=5&minPoolSize=1",
			}

			// Create multiple clients to test pooling
			clients := make([]*mongo.Client, 3)
			for i := 0; i < 3; i++ {
				client, err := config.NewClient(ctx)
				require.NoError(t, err, "Failed to create client %d", i)
				clients[i] = client

				// Perform operation to ensure connection is established
				err = client.Ping(ctx, nil)
				assert.NoError(t, err, "Failed to ping with client %d", i)
			}

			// Clean up all clients
			for i, client := range clients {
				err := client.Disconnect(ctx)
				assert.NoError(t, err, "Failed to disconnect client %d", i)
			}
		})

		t.Run("should handle timeouts correctly", func(t *testing.T) {
			// Create a context with a very short timeout
			shortCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
			defer cancel()

			config := mdbconfig.Config{
				Uri: mongoURI,
			}

			client, err := config.NewClient(ctx) // Use normal context for connection
			require.NoError(t, err)
			defer client.Disconnect(ctx)

			// Try to perform an operation with the short timeout context
			collection := client.Database("test_db").Collection("test_collection")
			
			// This should timeout
			_, err = collection.InsertOne(shortCtx, bson.M{"test": "timeout"})
			assert.Error(t, err, "Expected timeout error")
			assert.Contains(t, err.Error(), "context deadline exceeded")
		})

		t.Run("should work with different MongoDB features", func(t *testing.T) {
			config := mdbconfig.Config{
				Uri: mongoURI,
			}

			client, err := config.NewClient(ctx)
			require.NoError(t, err)
			defer client.Disconnect(ctx)

			db := client.Database("feature_test_db")
			collection := db.Collection("feature_test_collection")

			// Test bulk operations
			docs := []interface{}{
				bson.M{"bulk": 1, "type": "test"},
				bson.M{"bulk": 2, "type": "test"},
				bson.M{"bulk": 3, "type": "test"},
			}
			bulkResult, err := collection.InsertMany(ctx, docs)
			require.NoError(t, err, "Failed to perform bulk insert")
			assert.Len(t, bulkResult.InsertedIDs, 3)

			// Test aggregation
			pipeline := []bson.M{
				{"$match": bson.M{"type": "test"}},
				{"$group": bson.M{
					"_id":   "$type",
					"count": bson.M{"$sum": 1},
					"total": bson.M{"$sum": "$bulk"},
				}},
			}

			cursor, err := collection.Aggregate(ctx, pipeline)
			require.NoError(t, err, "Failed to perform aggregation")
			defer cursor.Close(ctx)

			var results []bson.M
			err = cursor.All(ctx, &results)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, "test", results[0]["_id"])
			assert.Equal(t, int32(3), results[0]["count"])
			assert.Equal(t, int32(6), results[0]["total"]) // 1+2+3

			// Test indexes
			indexModel := mongo.IndexModel{
				Keys: bson.D{{"type", 1}, {"bulk", -1}},
			}
			indexName, err := collection.Indexes().CreateOne(ctx, indexModel)
			require.NoError(t, err, "Failed to create index")
			assert.NotEmpty(t, indexName)

			// Clean up
			_, err = collection.DeleteMany(ctx, bson.M{})
			assert.NoError(t, err)
		})
	})
}