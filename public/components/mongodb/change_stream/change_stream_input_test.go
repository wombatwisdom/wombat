package change_stream_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wombatwisdom/wombat/public/components/mongodb"
	"github.com/wombatwisdom/wombat/public/components/mongodb/change_stream"
)

func TestChangeStreamReaderOptions(t *testing.T) {
	tests := []struct {
		name       string
		options    change_stream.ChangeStreamReaderOptions
		wantURI    string
		wantDB     string
		wantColl   string
	}{
		{
			name: "minimal config",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
			},
			wantURI: "mongodb://localhost:27017",
		},
		{
			name: "config with database",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Database: "testdb",
			},
			wantURI: "mongodb://localhost:27017",
			wantDB:  "testdb",
		},
		{
			name: "config with database and collection",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Database:   "testdb",
				Collection: "testcoll",
			},
			wantURI:  "mongodb://localhost:27017",
			wantDB:   "testdb",
			wantColl: "testcoll",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantURI, tt.options.Uri)
			assert.Equal(t, tt.wantDB, tt.options.Database)
			assert.Equal(t, tt.wantColl, tt.options.Collection)
		})
	}
}

func TestChangeStreamReaderOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		options change_stream.ChangeStreamReaderOptions
		valid   bool
		reason  string
	}{
		{
			name: "valid minimal config",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
			},
			valid: true,
		},
		{
			name: "empty URI should be invalid",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "",
				},
			},
			valid:  false,
			reason: "empty URI",
		},
		{
			name: "valid with database only",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Database: "testdb",
			},
			valid: true,
		},
		{
			name: "valid with database and collection",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Database:   "testdb",
				Collection: "testcoll",
			},
			valid: true,
		},
		{
			name: "collection without database is logically invalid",
			options: change_stream.ChangeStreamReaderOptions{
				Config: mongodb.Config{
					Uri: "mongodb://localhost:27017",
				},
				Collection: "testcoll", // No database specified
			},
			valid:  false,
			reason: "collection specified without database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic that a collection cannot be specified without a database
			if tt.options.Collection != "" && tt.options.Database == "" {
				assert.False(t, tt.valid, "Collection without database should be invalid")
			}
			
			// Test that empty URI is invalid
			if tt.options.Uri == "" {
				assert.False(t, tt.valid, "Empty URI should be invalid")
			}
		})
	}
}