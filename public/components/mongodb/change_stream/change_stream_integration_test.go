//go:build integration
// +build integration

package change_stream_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	mdbconfig "github.com/wombatwisdom/wombat/public/components/mongodb"
	"github.com/wombatwisdom/wombat/public/components/mongodb/change_stream"
)

type MongoDBContainer struct {
	Container testcontainers.Container
	URI       string
}

func setupMongoDBContainer(t *testing.T, ctx context.Context) *MongoDBContainer {
	t.Helper()

	// Start MongoDB with replica set enabled for change streams
	mongodbContainer, err := mongodb.Run(ctx,
		"mongo:7.0",
		mongodb.WithUsername("root"),
		mongodb.WithPassword("password"),
		mongodb.WithReplicaSet("rs0"),
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

	// For Podman compatibility: Replace internal IP with localhost
	// when TESTCONTAINERS_HOST_OVERRIDE is set
	if hostOverride := os.Getenv("TESTCONTAINERS_HOST_OVERRIDE"); hostOverride != "" {
		// Parse the URL to replace the host
		u, err := url.Parse(connectionString)
		require.NoError(t, err, "Failed to parse connection string")
		
		// Get the mapped port
		mappedPort, err := mongodbContainer.MappedPort(ctx, "27017/tcp")
		require.NoError(t, err, "Failed to get mapped port")
		
		// Replace host with localhost and mapped port
		u.Host = fmt.Sprintf("%s:%s", hostOverride, mappedPort.Port())
		
		// Add directConnection to bypass replica set discovery
		q := u.Query()
		q.Set("directConnection", "true")
		u.RawQuery = q.Encode()
		
		connectionString = u.String()
		
		t.Logf("Modified connection string for Podman: %s", connectionString)
	}

	// Wait for MongoDB to be ready
	opts := options.Client().ApplyURI(connectionString)
	client, err := mongo.Connect(ctx, opts)
	require.NoError(t, err, "Failed to connect to MongoDB")
	defer client.Disconnect(ctx)

	// Ping to ensure connection is established
	err = client.Ping(ctx, nil)
	require.NoError(t, err, "Failed to ping MongoDB")

	return &MongoDBContainer{
		Container: mongodbContainer,
		URI:       connectionString,
	}
}

func TestChangeStreamIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mongoContainer := setupMongoDBContainer(t, ctx)

	t.Run("ChangeStreamReader", func(t *testing.T) {
		t.Run("should read insert operations from collection", func(t *testing.T) {
			// Setup
			database := "test_db"
			collection := "test_collection"

			opts := change_stream.ChangeStreamReaderOptions{
				Config: mdbconfig.Config{
					Uri: mongoContainer.URI,
				},
				Database:   database,
				Collection: collection,
			}

			reader := change_stream.NewChangeStreamReader(opts)

			// Connect the reader
			err := reader.Connect(ctx)
			require.NoError(t, err, "Failed to connect reader")
			defer reader.Close(ctx)

			// Insert test data in a separate goroutine
			go func() {
				// Wait a bit to ensure change stream is ready
				time.Sleep(500 * time.Millisecond)

				// Connect separately to insert data
				client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoContainer.URI))
				require.NoError(t, err)
				defer client.Disconnect(ctx)

				coll := client.Database(database).Collection(collection)

				// Insert documents
				for i := 0; i < 3; i++ {
					doc := bson.M{
						"name":  fmt.Sprintf("test-%d", i),
						"value": i,
						"time":  time.Now(),
					}
					_, err = coll.InsertOne(ctx, doc)
					require.NoError(t, err)
					time.Sleep(100 * time.Millisecond)
				}
			}()

			// Read change events
			messages := make([]*service.Message, 0, 3)
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
			defer readCancel()

			for len(messages) < 3 {
				msg, err := reader.Read(readCtx)
				if err != nil {
					if err == service.ErrEndOfInput || err == context.DeadlineExceeded {
						break
					}
					require.NoError(t, err)
				}
				messages = append(messages, msg)
			}

			// Verify we got messages
			assert.Len(t, messages, 3, "Expected to read 3 change events")

			// Verify message content
			for i, msg := range messages {
				// Check metadata
				id, exists := msg.MetaGet(change_stream.IdHeader)
				assert.True(t, exists, "Message should have change stream ID")
				assert.NotEmpty(t, id)

				// Parse message content
				msgBytes, err := msg.AsBytes()
				require.NoError(t, err)

				var changeEvent bson.M
				err = bson.UnmarshalExtJSON(msgBytes, false, &changeEvent)
				require.NoError(t, err)

				// Debug: Log the entire change event
				t.Logf("Change event %d: %+v", i, changeEvent)

				// Verify change event structure
				assert.Equal(t, "insert", changeEvent["operationType"])

				fullDoc, ok := changeEvent["fullDocument"].(bson.M)
				if !ok {
					// Try to see what type it actually is
					t.Logf("fullDocument type: %T, value: %+v", changeEvent["fullDocument"], changeEvent["fullDocument"])
				}
				require.True(t, ok, "fullDocument should be present")

				assert.Equal(t, fmt.Sprintf("test-%d", i), fullDoc["name"])
				assert.Equal(t, int32(i), fullDoc["value"]) // BSON numbers are int32 by default
			}
		})

		t.Run("should read from database level", func(t *testing.T) {
			database := "test_db_level"

			opts := change_stream.ChangeStreamReaderOptions{
				Config: mdbconfig.Config{
					Uri: mongoContainer.URI,
				},
				Database:   database,
				Collection: "", // Empty collection means database-level watch
			}

			reader := change_stream.NewChangeStreamReader(opts)

			err := reader.Connect(ctx)
			require.NoError(t, err, "Failed to connect reader")
			defer reader.Close(ctx)

			// Insert data into multiple collections
			go func() {
				time.Sleep(500 * time.Millisecond)

				client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoContainer.URI))
				require.NoError(t, err)
				defer client.Disconnect(ctx)

				db := client.Database(database)

				// Insert into different collections
				collections := []string{"coll1", "coll2", "coll3"}
				for _, collName := range collections {
					doc := bson.M{"collection": collName, "data": "test"}
					_, err = db.Collection(collName).InsertOne(ctx, doc)
					require.NoError(t, err)
					time.Sleep(100 * time.Millisecond)
				}
			}()

			// Read events from all collections
			messages := make([]*service.Message, 0, 3)
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
			defer readCancel()

			for len(messages) < 3 {
				msg, err := reader.Read(readCtx)
				if err != nil {
					if err == service.ErrEndOfInput || err == context.DeadlineExceeded {
						break
					}
					require.NoError(t, err)
				}
				messages = append(messages, msg)
			}

			assert.Len(t, messages, 3, "Expected to read 3 change events from different collections")

			// Verify we got events from different collections
			collectionsSeen := make(map[string]bool)
			for _, msg := range messages {
				msgBytes, err := msg.AsBytes()
				require.NoError(t, err)

				var changeEvent bson.M
				err = bson.UnmarshalExtJSON(msgBytes, false, &changeEvent)
				require.NoError(t, err)

				ns, ok := changeEvent["ns"].(bson.M)
				require.True(t, ok)

				collName, ok := ns["coll"].(string)
				require.True(t, ok)
				collectionsSeen[collName] = true
			}

			assert.Len(t, collectionsSeen, 3, "Should have seen events from 3 different collections")
		})

		t.Run("should handle update and delete operations", func(t *testing.T) {
			database := "test_ops_db"
			collection := "test_ops_collection"

			opts := change_stream.ChangeStreamReaderOptions{
				Config: mdbconfig.Config{
					Uri: mongoContainer.URI,
				},
				Database:   database,
				Collection: collection,
			}

			reader := change_stream.NewChangeStreamReader(opts)

			err := reader.Connect(ctx)
			require.NoError(t, err, "Failed to connect reader")
			defer reader.Close(ctx)

			// Perform various operations
			go func() {
				time.Sleep(500 * time.Millisecond)

				client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoContainer.URI))
				require.NoError(t, err)
				defer client.Disconnect(ctx)

				coll := client.Database(database).Collection(collection)

				// Insert
				result, err := coll.InsertOne(ctx, bson.M{"_id": "test-1", "value": "initial"})
				require.NoError(t, err)
				time.Sleep(100 * time.Millisecond)

				// Update
				_, err = coll.UpdateByID(ctx, result.InsertedID, bson.M{"$set": bson.M{"value": "updated"}})
				require.NoError(t, err)
				time.Sleep(100 * time.Millisecond)

				// Delete
				_, err = coll.DeleteOne(ctx, bson.M{"_id": result.InsertedID})
				require.NoError(t, err)
			}()

			// Read all operation types
			operations := make([]string, 0, 3)
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
			defer readCancel()

			for len(operations) < 3 {
				msg, err := reader.Read(readCtx)
				if err != nil {
					if err == service.ErrEndOfInput || err == context.DeadlineExceeded {
						break
					}
					require.NoError(t, err)
				}

				msgBytes, err := msg.AsBytes()
				require.NoError(t, err)

				var changeEvent bson.M
				err = bson.UnmarshalExtJSON(msgBytes, false, &changeEvent)
				require.NoError(t, err)

				opType, ok := changeEvent["operationType"].(string)
				require.True(t, ok)
				operations = append(operations, opType)
			}

			assert.Equal(t, []string{"insert", "update", "delete"}, operations)
		})

		t.Run("should handle connection errors gracefully", func(t *testing.T) {
			opts := change_stream.ChangeStreamReaderOptions{
				Config: mdbconfig.Config{
					Uri: "mongodb://invalid-host:27017/?connectTimeoutMS=1000&serverSelectionTimeoutMS=1000",
				},
				Database:   "test_db",
				Collection: "test_collection",
			}

			reader := change_stream.NewChangeStreamReader(opts)

			err := reader.Connect(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create change stream")
		})

		t.Run("should properly close resources", func(t *testing.T) {
			opts := change_stream.ChangeStreamReaderOptions{
				Config: mdbconfig.Config{
					Uri: mongoContainer.URI,
				},
				Database:   "close_test_db",
				Collection: "close_test_collection",
			}

			reader := change_stream.NewChangeStreamReader(opts)

			// Connect
			err := reader.Connect(ctx)
			require.NoError(t, err, "Failed to connect reader")

			// Close
			err = reader.Close(ctx)
			assert.NoError(t, err, "Failed to close reader")

			// Attempting to read after close should fail
			_, err = reader.Read(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not connected")
		})
	})

	t.Run("ChangeStreamInput", func(t *testing.T) {
		t.Run("should work end-to-end with service framework", func(t *testing.T) {
			database := "service_test_db"
			collection := "service_test_collection"

			// Create config spec
			spec := service.NewConfigSpec().
				Fields(mdbconfig.Fields...).
				Field(service.NewStringField("database").Default("")).
				Field(service.NewStringField("collection").Default(""))

			// Create configuration
			confStr := fmt.Sprintf(`
uri: %s
database: %s
collection: %s
`, mongoContainer.URI, database, collection)

			env := service.NewEnvironment()
			parsedConf, err := spec.ParseYAML(confStr, env)
			require.NoError(t, err)

			// Create input
			input, err := change_stream.NewChangeStreamInput(parsedConf, service.MockResources())
			require.NoError(t, err)

			// Connect
			err = input.Connect(ctx)
			require.NoError(t, err)
			defer input.Close(ctx)

			// Insert data
			go func() {
				time.Sleep(500 * time.Millisecond)

				client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoContainer.URI))
				require.NoError(t, err)
				defer client.Disconnect(ctx)

				coll := client.Database(database).Collection(collection)
				_, err = coll.InsertOne(ctx, bson.M{"service": "test", "timestamp": time.Now()})
				require.NoError(t, err)
			}()

			// Read through input interface
			readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
			defer readCancel()

			msg, ackFn, err := input.Read(readCtx)
			require.NoError(t, err)
			require.NotNil(t, msg)
			require.NotNil(t, ackFn)

			// Verify message
			msgBytes, err := msg.AsBytes()
			require.NoError(t, err)

			var changeEvent bson.M
			err = bson.UnmarshalExtJSON(msgBytes, false, &changeEvent)
			require.NoError(t, err)

			assert.Equal(t, "insert", changeEvent["operationType"])

			// Test ack function
			err = ackFn(ctx, nil)
			assert.NoError(t, err)
		})
	})
}

// Helper to ensure the function is exported for use in integration tests
func NewChangeStreamInput(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	return change_stream.NewChangeStreamInput(conf, mgr)
}
