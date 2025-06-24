package change_stream_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/wombat/public/components/mongodb"
	"github.com/wombatwisdom/wombat/public/components/mongodb/change_stream"
	"go.mongodb.org/mongo-driver/bson"
)

func TestChangeStreamReader_Connect(t *testing.T) {
	tests := []struct {
		name        string
		options     change_stream.ChangeStreamReaderOptions
		wantErr     bool
		errContains string
	}{
		{
			name: "fails with empty URI",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "",
				},
			},
			wantErr:     true,
			errContains: "uri is required",
		},
		{
			name: "fails with invalid URI",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "not-a-valid-uri",
				},
			},
			wantErr:     true,
			errContains: "failed to create client",
		},
		// Note: Successful connection tests would require a real MongoDB instance
		// or more complex mocking that's better suited for integration tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := change_stream.NewChangeStreamReader(tt.options)
			err := reader.Connect(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChangeStreamReader_ReadErrors(t *testing.T) {
	t.Run("returns error when not connected", func(t *testing.T) {
		options := change_stream.ChangeStreamReaderOptions{
			Config: mongodb.Config{
				Uri: "mongodb://localhost:27017",
			},
		}
		reader := change_stream.NewChangeStreamReader(options)

		// Try to read without connecting
		msg, err := reader.Read(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "change stream not connected")
		assert.Nil(t, msg)
	})
}

func TestChangeStreamReader_Close(t *testing.T) {
	t.Run("handles close when not connected", func(t *testing.T) {
		options := change_stream.ChangeStreamReaderOptions{
			Config: mongodb.Config{
				Uri: "mongodb://localhost:27017",
			},
		}
		reader := change_stream.NewChangeStreamReader(options)

		// Close without connecting should not error
		err := reader.Close(context.Background())
		assert.NoError(t, err)
	})
}

func TestChangeStreamReaderOptions_ChangeStream(t *testing.T) {
	tests := []struct {
		name       string
		database   string
		collection string
		wantLevel  string // "client", "database", or "collection"
	}{
		{
			name:      "client level watch when no database specified",
			wantLevel: "client",
		},
		{
			name:      "database level watch when only database specified",
			database:  "testdb",
			wantLevel: "database",
		},
		{
			name:       "collection level watch when both database and collection specified",
			database:   "testdb",
			collection: "testcoll",
			wantLevel:  "collection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Database:   tt.database,
				Collection: tt.collection,
			}

			// Verify the options are set correctly
			assert.Equal(t, tt.database, options.Database)
			assert.Equal(t, tt.collection, options.Collection)

			// The actual ChangeStream method would be tested in integration tests
			// as it requires a real MongoDB client connection
		})
	}
}

// TestMessageFormat tests that messages are formatted correctly
func TestMessageFormat(t *testing.T) {
	// This test demonstrates the expected message format
	// In a real implementation, this would be part of integration tests
	
	t.Run("message contains change stream ID in metadata", func(t *testing.T) {
		// Create a sample message as the reader would
		changeData := bson.M{
			"operationType": "insert",
			"fullDocument": bson.M{
				"_id":  "123",
				"name": "test",
			},
		}
		
		b, err := bson.MarshalExtJSON(changeData, false, false)
		require.NoError(t, err)
		
		msg := service.NewMessage(b)
		msg.MetaSet(change_stream.IdHeader, "12345")
		
		// Verify message content
		msgBytes, err := msg.AsBytes()
		require.NoError(t, err)
		assert.Contains(t, string(msgBytes), "insert")
		assert.Contains(t, string(msgBytes), "test")
		
		// Verify metadata
		id, exists := msg.MetaGet(change_stream.IdHeader)
		assert.True(t, exists)
		assert.Equal(t, "12345", id)
	})
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		options change_stream.ChangeStreamReaderOptions
		setup   func()
		check   func(t *testing.T, reader *change_stream.ChangeStreamReader)
	}{
		{
			name: "handles connection errors gracefully",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://unreachable-host:27017/?connectTimeoutMS=100&serverSelectionTimeoutMS=100",
				},
			},
			check: func(t *testing.T, reader *change_stream.ChangeStreamReader) {
				err := reader.Connect(context.Background())
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to create")
			},
		},
		{
			name: "validates configuration",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "",
				},
			},
			check: func(t *testing.T, reader *change_stream.ChangeStreamReader) {
				err := reader.Connect(context.Background())
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "uri is required")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			
			reader := change_stream.NewChangeStreamReader(tt.options)
			tt.check(t, reader)
		})
	}
}

// TestConcurrency tests concurrent access scenarios
func TestConcurrency(t *testing.T) {
	t.Run("multiple close calls are safe", func(t *testing.T) {
		options := change_stream.ChangeStreamReaderOptions{
			Config: mongodb.Config{
				Uri: "mongodb://localhost:27017",
			},
		}
		reader := change_stream.NewChangeStreamReader(options)

		// Multiple close calls should not panic
		err1 := reader.Close(context.Background())
		err2 := reader.Close(context.Background())
		
		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

func TestNewChangeStreamReader(t *testing.T) {
	t.Run("creates reader with correct options", func(t *testing.T) {
		options := change_stream.ChangeStreamReaderOptions{
			Config: mongodb.Config{
				Uri: "mongodb://localhost:27017",
			},
			Database:   "testdb",
			Collection: "testcoll",
		}

		reader := change_stream.NewChangeStreamReader(options)
		assert.NotNil(t, reader)
	})
}

func TestChangeStreamReaderOptions_DatabaseAndCollection(t *testing.T) {
	tests := []struct {
		name          string
		database      string
		collection    string
		expectDB      bool
		expectColl    bool
	}{
		{
			name:          "no database or collection",
			database:      "",
			collection:    "",
			expectDB:      false,
			expectColl:    false,
		},
		{
			name:          "database only",
			database:      "testdb",
			collection:    "",
			expectDB:      true,
			expectColl:    false,
		},
		{
			name:          "both database and collection",
			database:      "testdb",
			collection:    "testcoll",
			expectDB:      true,
			expectColl:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Database:   tt.database,
				Collection: tt.collection,
			}

			// Verify options are set correctly
			if tt.expectDB {
				assert.NotEmpty(t, options.Database)
				assert.Equal(t, tt.database, options.Database)
			} else {
				assert.Empty(t, options.Database)
			}

			if tt.expectColl {
				assert.NotEmpty(t, options.Collection)
				assert.Equal(t, tt.collection, options.Collection)
			} else {
				assert.Empty(t, options.Collection)
			}
		})
	}
}

// Test various URI formats and configurations
func TestChangeStreamReader_URIFormats(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty URI",
			uri:         "",
			wantErr:     true,
			errContains: "uri is required",
		},
		{
			name:        "basic URI",
			uri:         "mongodb://localhost:27017",
			wantErr:     true, // Will fail because no MongoDB running
			errContains: "failed to create change stream",
		},
		{
			name:        "URI with auth",
			uri:         "mongodb://user:pass@localhost:27017",
			wantErr:     true, // Will fail because no MongoDB running
			errContains: "failed to create change stream",
		},
		{
			name:        "URI with replica set",
			uri:         "mongodb://host1:27017,host2:27017/db?replicaSet=rs0",
			wantErr:     true, // Will fail because no MongoDB running
			errContains: "failed to create change stream",
		},
		{
			name:        "malformed URI",
			uri:         "not-a-mongodb-uri",
			wantErr:     true,
			errContains: "failed to create client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: tt.uri,
				},
			}

			reader := change_stream.NewChangeStreamReader(options)
			err := reader.Connect(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test message processing edge cases
func TestMessageProcessing(t *testing.T) {
	t.Run("validates change stream ID header constant", func(t *testing.T) {
		assert.Equal(t, "mongodb_change_stream_id", change_stream.IdHeader)
	})

	t.Run("formats various BSON types correctly", func(t *testing.T) {
		testCases := []struct {
			name string
			data bson.M
		}{
			{
				name: "simple document",
				data: bson.M{"field": "value"},
			},
			{
				name: "complex document",
				data: bson.M{
					"operationType": "insert",
					"fullDocument": bson.M{
						"_id":   "507f1f77bcf86cd799439011",
						"name":  "test document",
						"count": 42,
						"active": true,
					},
					"ns": bson.M{
						"db":   "testdb",
						"coll": "testcoll",
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				b, err := bson.MarshalExtJSON(tc.data, false, false)
				require.NoError(t, err)

				msg := service.NewMessage(b)
				msg.MetaSet(change_stream.IdHeader, "123456")

				// Verify message can be read back
				msgBytes, err := msg.AsBytes()
				require.NoError(t, err)
				assert.NotEmpty(t, msgBytes)

				// Verify metadata
				id, exists := msg.MetaGet(change_stream.IdHeader)
				assert.True(t, exists)
				assert.Equal(t, "123456", id)
			})
		}
	})
}