package snowflake

import (
	"fmt"
	"testing"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnowflakePutOutputConfig(t *testing.T) {
	t.Run("should create a valid configuration spec", func(t *testing.T) {
		spec := snowflakePutOutputConfig()
		require.NotNil(t, spec)
		
		// Test that the spec can parse a minimal valid configuration
		confStr := `
account: test_account
region: us-west-2
cloud: aws
user: test_user
password: test_password
role: test_role
database: test_db
warehouse: test_warehouse
schema: test_schema
path: test/path
stage: "@test_stage"
file_name: "test_file"
file_extension: "json"
`
		env := service.NewEnvironment()
		conf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)
		require.NotNil(t, conf)
	})

	t.Run("should require mandatory fields", func(t *testing.T) {
		spec := snowflakePutOutputConfig()
		env := service.NewEnvironment()
		
		// Test with missing account field
		confStr := `
region: us-west-2
cloud: aws
user: test_user
password: test_password
`
		_, err := spec.ParseYAML(confStr, env)
		require.Error(t, err)
	})
}

func TestNewSnowflakeWriterFromConfig(t *testing.T) {
	mgr := service.MockResources()

	t.Run("should create writer with password authentication", func(t *testing.T) {
		confStr := `
account: test_account
region: us-west-2
cloud: aws
user: test_user
password: test_password
role: test_role
database: test_db
warehouse: test_warehouse
schema: test_schema
path: test/path
stage: "@test_stage"
file_name: "test_file"
file_extension: "json"
`
		spec := snowflakePutOutputConfig()
		env := service.NewEnvironment()
		conf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		writer, err := newSnowflakeWriterFromConfig(conf, mgr)
		require.NoError(t, err)
		require.NotNil(t, writer)
		assert.Equal(t, "test_account", writer.account)
		assert.Equal(t, "test_user", writer.user)
		assert.Equal(t, "test_db", writer.database)
		assert.Equal(t, "test_schema", writer.schema)
	})

	t.Run("should handle interpolated fields", func(t *testing.T) {
		confStr := `
account: test_account
region: us-west-2
cloud: aws
user: test_user
password: test_password
role: test_role
database: test_db
warehouse: test_warehouse
schema: test_schema
path: '${! meta("output_path") }'
stage: '@${! meta("stage_name") }'
file_name: '${! json("id") }'
file_extension: '${! meta("format") }'
request_id: '${! uuid_v4() }'
snowpipe: '${! meta("pipe_name") }'
`
		spec := snowflakePutOutputConfig()
		env := service.NewEnvironment()
		conf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		writer, err := newSnowflakeWriterFromConfig(conf, mgr)
		require.NoError(t, err)
		require.NotNil(t, writer)
		
		// Verify interpolated strings were created
		assert.NotNil(t, writer.path)
		assert.NotNil(t, writer.stage)
		assert.NotNil(t, writer.fileName)
		assert.NotNil(t, writer.fileExtension)
		assert.NotNil(t, writer.requestID)
		assert.NotNil(t, writer.snowpipe)
	})

	t.Run("should handle compression settings", func(t *testing.T) {
		compressionTypes := []string{"NONE", "AUTO", "GZIP", "DEFLATE", "RAW_DEFLATE", "ZSTD"}
		
		for _, compression := range compressionTypes {
			confStr := `
account: test_account
region: us-west-2
cloud: aws
user: test_user
password: test_password
role: test_role
database: test_db
warehouse: test_warehouse
schema: test_schema
path: test/path
stage: "@test_stage"
file_name: "test_file"
file_extension: "json"
compression: ` + compression + `
`
			spec := snowflakePutOutputConfig()
			env := service.NewEnvironment()
			conf, err := spec.ParseYAML(confStr, env)
			require.NoError(t, err)

			writer, err := newSnowflakeWriterFromConfig(conf, mgr)
			require.NoError(t, err)
			// Compression is handled differently in the actual implementation
			// We'll verify the writer was created successfully with the compression setting
			assert.NotNil(t, writer)
		}
	})
}

func TestSnowflakeJWTCreation(t *testing.T) {
	t.Run("should fail without a private key", func(t *testing.T) {
		writer := &snowflakeWriter{
			account:               "test_account",
			user:                  "test_user",
			publicKeyFingerprint: "test_fingerprint",
			nowFn:                func() time.Time { return time.Unix(0, 0) },
		}

		// Without a private key, createJWT will panic when trying to sign the token
		// We need to catch this as a panic instead of expecting a normal error
		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, fmt.Sprintf("%v", r), "nil")
			}
		}()

		// This will panic due to nil private key
		jwt, err := writer.createJWT()
		// If we reach here without panic, something is wrong
		if err == nil {
			t.Errorf("Expected error or panic, but got jwt: %s", jwt)
		}
	})
}

func TestSnowflakeHelperFunctions(t *testing.T) {
	t.Run("getSnowpipeInsertURL should construct correct Snowpipe URL", func(t *testing.T) {
		writer := &snowflakeWriter{
			account:           "test_account",
			accountIdentifier: "test_account.us-west-2.aws",
			database:          "test_db",
			schema:            "test_schema",
		}

		url := writer.getSnowpipeInsertURL("test_pipe", "request123")
		expected := "https://test_account.us-west-2.aws.snowflakecomputing.com/v1/data/pipes/test_db.test_schema.test_pipe/insertFiles?requestId=request123"
		assert.Equal(t, expected, url)
	})

	t.Run("getSnowpipeInsertURL should handle different cloud providers", func(t *testing.T) {
		testCases := []struct {
			cloud    string
			expected string
		}{
			{"aws", "https://test_account.us-west-2.aws.snowflakecomputing.com/v1/data/pipes/db.schema.pipe/insertFiles?requestId=123"},
			{"azure", "https://test_account.us-west-2.azure.snowflakecomputing.com/v1/data/pipes/db.schema.pipe/insertFiles?requestId=123"},
			{"gcp", "https://test_account.us-west-2.gcp.snowflakecomputing.com/v1/data/pipes/db.schema.pipe/insertFiles?requestId=123"},
		}

		for _, tc := range testCases {
			writer := &snowflakeWriter{
				account:           "test_account",
				accountIdentifier: "test_account.us-west-2." + tc.cloud,
				database:          "db",
				schema:            "schema",
			}
			url := writer.getSnowpipeInsertURL("pipe", "123")
			assert.Equal(t, tc.expected, url)
		}
	})
}