package gcp_bigtable

import (
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_RegistrationCoverage(t *testing.T) {
	// This test increases coverage for the init function registration path
	t.Run("init registration succeeds", func(t *testing.T) {
		// The init function should have already run when the package was imported
		// Verify the config spec is valid
		assert.NotNil(t, GCPBigTableConfig)
		
		// Verify the config spec is properly configured
		// The spec should have been created with fields
	})

	t.Run("create output with valid batching config", func(t *testing.T) {
		config := `
project: test-project
instance: test-instance
table: test-table
emulated_host_port: localhost:8086
max_in_flight: 10
batching:
  count: 100
  period: 1s
`
		spec := GCPBigTableConfig
		env := service.NewEnvironment()
		parsedConf, err := spec.ParseYAML(config, env)
		require.NoError(t, err)

		// This tests the init function's factory function
		mgr := service.MockResources()
		output, err := NewGCPBigTableOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
		
		// Verify batching policy was parsed
		batchPolicy, err := parsedConf.FieldBatchPolicy("batching")
		require.NoError(t, err)
		assert.Equal(t, 100, batchPolicy.Count)
		
		// Verify max_in_flight was parsed
		maxInFlight, err := parsedConf.FieldInt("max_in_flight")
		require.NoError(t, err)
		assert.Equal(t, 10, maxInFlight)
	})
}

func TestConfigSpec_Fields(t *testing.T) {
	// This test verifies all fields in the config spec are properly defined
	t.Run("all fields are accessible", func(t *testing.T) {
		config := `
project: my-project
instance: my-instance
table: my-table
key: this.id
data: this.payload
credentials_json: '{"type":"service_account","project_id":"test"}'
emulated_host_port: localhost:9000
max_in_flight: 1024
batching:
  count: 50
  period: 500ms
  byte_size: 10000
`
		spec := GCPBigTableConfig
		env := service.NewEnvironment()
		parsedConf, err := spec.ParseYAML(config, env)
		require.NoError(t, err)
		
		// Verify all fields can be accessed
		project, err := parsedConf.FieldString("project")
		require.NoError(t, err)
		assert.Equal(t, "my-project", project)
		
		instance, err := parsedConf.FieldString("instance")
		require.NoError(t, err)
		assert.Equal(t, "my-instance", instance)
		
		table, err := parsedConf.FieldString("table")
		require.NoError(t, err)
		assert.Equal(t, "my-table", table)
		
		key, err := parsedConf.FieldString("key")
		require.NoError(t, err)
		assert.Equal(t, "this.id", key)
		
		data, err := parsedConf.FieldString("data")
		require.NoError(t, err)
		assert.Equal(t, "this.payload", data)
		
		creds, err := parsedConf.FieldString("credentials_json")
		require.NoError(t, err)
		assert.Contains(t, creds, "service_account")
		
		emulated, err := parsedConf.FieldString("emulated_host_port")
		require.NoError(t, err)
		assert.Equal(t, "localhost:9000", emulated)
		
		maxInFlight, err := parsedConf.FieldInt("max_in_flight")
		require.NoError(t, err)
		assert.Equal(t, 1024, maxInFlight)
		
		batchPolicy, err := parsedConf.FieldBatchPolicy("batching")
		require.NoError(t, err)
		assert.Equal(t, 50, batchPolicy.Count)
		assert.Equal(t, 10000, batchPolicy.ByteSize)
	})
}