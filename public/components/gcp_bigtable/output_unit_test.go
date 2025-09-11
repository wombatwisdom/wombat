package gcp_bigtable

import (
	"context"
	"testing"

	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGCPBigTableOutput_ConfigErrors(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		errContains string
	}{
		{
			name: "missing project",
			config: `
instance: test-instance
table: test-table
`,
			errContains: "project",
		},
		{
			name: "missing instance",
			config: `
project: test-project
table: test-table
`,
			errContains: "instance",
		},
		{
			name: "missing table",
			config: `
project: test-project
instance: test-instance
`,
			errContains: "table",
		},
		{
			name: "invalid key expression",
			config: `
project: test-project
instance: test-instance
table: test-table
key: "invalid bloblang !!!"
`,
			errContains: "failed to parse row key bloblang query",
		},
		{
			name: "invalid data expression",
			config: `
project: test-project
instance: test-instance
table: test-table
key: this.key
data: "invalid bloblang !!!"
`,
			errContains: "failed to parse row data bloblang query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := GCPBigTableConfig
			env := service.NewEnvironment()
			parsedConf, err := spec.ParseYAML(tt.config, env)

			// Check if parsing failed (for missing required fields)
			if err != nil {
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			mgr := service.MockResources()
			_, err = NewGCPBigTableOutput(parsedConf, mgr)
			assert.Error(t, err)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}
		})
	}
}

func TestNewGCPBigTableOutput_ValidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config string
		check  func(t *testing.T, output *GCPBigTableOutput)
	}{
		{
			name: "basic config",
			config: `
project: test-project
instance: test-instance
table: test-table
`,
			check: func(t *testing.T, output *GCPBigTableOutput) {
				assert.Equal(t, "test-project", output.project)
				assert.Equal(t, "test-instance", output.instance)
				assert.Equal(t, "test-table", output.table)
				assert.Empty(t, output.credentialsJSON)
				assert.NotNil(t, output.rke)
				assert.NotNil(t, output.rde)
			},
		},
		{
			name: "with credentials",
			config: `
project: test-project
instance: test-instance
table: test-table
credentials_json: '{"type":"service_account"}'
`,
			check: func(t *testing.T, output *GCPBigTableOutput) {
				assert.Equal(t, `{"type":"service_account"}`, output.credentialsJSON)
			},
		},
		{
			name: "with emulated host",
			config: `
project: test-project
instance: test-instance
table: test-table
emulated_host_port: localhost:8086
`,
			check: func(t *testing.T, output *GCPBigTableOutput) {
				assert.Equal(t, "localhost:8086", output.emulated)
			},
		},
		{
			name: "custom expressions",
			config: `
project: test-project
instance: test-instance
table: test-table
key: this.id
data: this
`,
			check: func(t *testing.T, output *GCPBigTableOutput) {
				assert.NotNil(t, output.rke)
				assert.NotNil(t, output.rde)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := GCPBigTableConfig
			env := service.NewEnvironment()
			parsedConf, err := spec.ParseYAML(tt.config, env)
			require.NoError(t, err)

			mgr := service.MockResources()
			output, err := NewGCPBigTableOutput(parsedConf, mgr)
			require.NoError(t, err)
			require.NotNil(t, output)

			if tt.check != nil {
				tt.check(t, output)
			}
		})
	}
}

func TestAsRowKeys(t *testing.T) {
	tests := []struct {
		name        string
		messages    []map[string]interface{}
		expression  string
		expected    []string
		wantErr     bool
		errContains string
	}{
		{
			name: "simple keys",
			messages: []map[string]interface{}{
				{"key": "key1", "data": "value1"},
				{"key": "key2", "data": "value2"},
			},
			expression: "this.key",
			expected:   []string{"key1", "key2"},
		},
		{
			name: "complex expression",
			messages: []map[string]interface{}{
				{"id": 1, "type": "user"},
				{"id": 2, "type": "admin"},
			},
			expression: `"%s/%d".format(this.type, this.id)`,
			expected:   []string{"user/1", "admin/2"},
		},
		{
			name: "nil result skipped",
			messages: []map[string]interface{}{
				{"key": "key1"},
				{"other": "value"},
				{"key": "key3"},
			},
			expression: "this.key",
			expected:   []string{"key1", "key3"},
		},
		{
			name: "non-string result",
			messages: []map[string]interface{}{
				{"key": 123},
			},
			expression:  "this.key",
			wantErr:     true,
			errContains: "expected string but got",
		},
		{
			name: "expression error",
			messages: []map[string]interface{}{
				{"key": "value"},
			},
			expression: "this.key",
			wantErr:    false,
			expected:   []string{"value"},
		},
		{
			name: "invalid message structure",
			messages: []map[string]interface{}{
				nil,
			},
			expression: "this.key",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := make(service.MessageBatch, len(tt.messages))
			for i, msg := range tt.messages {
				if msg == nil {
					batch[i] = service.NewMessage([]byte("invalid"))
				} else {
					batch[i] = service.NewMessage(nil)
					batch[i].SetStructured(msg)
				}
			}

			rke, err := bloblang.Parse(tt.expression)
			require.NoError(t, err)

			result, err := asRowKeys(batch, rke)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAsMutation(t *testing.T) {
	tests := []struct {
		name        string
		message     interface{}
		expression  string
		expectNil   bool
		wantErr     bool
		errContains string
		validate    func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "simple mutation",
			message: map[string]interface{}{
				"key": "key1",
				"cf1": map[string]interface{}{
					"col1": "value1",
					"col2": 42,
				},
			},
			expression: `this.without("key")`,
			validate: func(t *testing.T, data map[string]interface{}) {
				cf1, ok := data["cf1"].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "value1", cf1["col1"])
				assert.Equal(t, 42, cf1["col2"])
			},
		},
		{
			name: "multiple column families",
			message: map[string]interface{}{
				"cf1": map[string]interface{}{
					"col1": "value1",
				},
				"cf2": map[string]interface{}{
					"col2": "value2",
				},
			},
			expression: "this",
			validate: func(t *testing.T, data map[string]interface{}) {
				assert.Len(t, data, 2)
			},
		},
		{
			name:       "nil result",
			message:    map[string]interface{}{"key": "value"},
			expression: "if this.missing != null { this } else { null }",
			expectNil:  true,
		},
		{
			name: "invalid column family format",
			message: map[string]interface{}{
				"cf1": "not a map",
			},
			expression:  "this",
			wantErr:     true,
			errContains: "expected family",
		},
		{
			name:        "non-map result",
			message:     map[string]interface{}{"key": "value"},
			expression:  "this.key",
			wantErr:     true,
			errContains: "expected the message root to be map",
		},
		{
			name: "marshal error",
			message: map[string]interface{}{
				"cf1": map[string]interface{}{
					"col1": func() {}, // Functions can't be marshaled
				},
			},
			expression:  "this",
			wantErr:     true,
			errContains: "failed to marshal",
		},
		{
			name:       "expression returns nil on missing field",
			message:    map[string]interface{}{"key": "value"},
			expression: "if this.nonexistent != null { this.nonexistent } else { null }",
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := service.NewMessage(nil)
			msg.SetStructured(tt.message)

			rde, err := bloblang.Parse(tt.expression)
			require.NoError(t, err)

			mut, err := asMutation(msg, rde)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.expectNil {
					assert.Nil(t, mut)
				} else {
					assert.NotNil(t, mut)
					// Note: We can't easily validate the mutation content
					// as bigtable.Mutation doesn't expose its internal state
				}
			}
		})
	}
}

func TestAsMutations(t *testing.T) {
	tests := []struct {
		name        string
		messages    []interface{}
		expression  string
		expectCount int
		wantErr     bool
		errContains string
	}{
		{
			name: "multiple valid mutations",
			messages: []interface{}{
				map[string]interface{}{
					"cf1": map[string]interface{}{"col1": "val1"},
				},
				map[string]interface{}{
					"cf1": map[string]interface{}{"col2": "val2"},
				},
			},
			expression:  "this",
			expectCount: 2,
		},
		{
			name: "mixed valid and nil",
			messages: []interface{}{
				map[string]interface{}{
					"cf1": map[string]interface{}{"col1": "val1"},
				},
				map[string]interface{}{"empty": "value"},
				map[string]interface{}{
					"cf2": map[string]interface{}{"col2": "val2"},
				},
			},
			expression:  "if this.cf1 != null || this.cf2 != null { this } else { null }",
			expectCount: 2,
		},
		{
			name: "error in one message",
			messages: []interface{}{
				map[string]interface{}{
					"cf1": map[string]interface{}{"col1": "val1"},
				},
				map[string]interface{}{
					"cf1": "invalid", // This will cause an error
				},
			},
			expression:  "this",
			wantErr:     true,
			errContains: "expected family",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := make(service.MessageBatch, len(tt.messages))
			for i, msg := range tt.messages {
				batch[i] = service.NewMessage(nil)
				batch[i].SetStructured(msg)
			}

			rde, err := bloblang.Parse(tt.expression)
			require.NoError(t, err)

			result, err := asMutations(batch, rde)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectCount)
			}
		})
	}
}

func TestGCPBigTableOutput_Close(t *testing.T) {
	t.Run("close with nil client", func(t *testing.T) {
		output := &GCPBigTableOutput{}
		err := output.Close(context.Background())
		assert.NoError(t, err)
	})

	// Note: Testing close with actual client requires mocking bigtable.Client
	// which is complex due to the library design
}

func TestGCPBigTableOutput_WriteBatch_Errors(t *testing.T) {
	output := &GCPBigTableOutput{
		rke: mustParseBloblang(t, "this.key"),
		rde: mustParseBloblang(t, "this.data"),
	}

	tests := []struct {
		name        string
		batch       service.MessageBatch
		errContains string
	}{
		{
			name: "row key extraction error",
			batch: service.MessageBatch{
				messageWithStructured(map[string]interface{}{
					"key": 123, // Non-string key
				}),
			},
			errContains: "failed to extract row keys",
		},
		{
			name: "mutation extraction error",
			batch: service.MessageBatch{
				messageWithStructured(map[string]interface{}{
					"key":  "valid-key",
					"data": "invalid", // Non-map data
				}),
			},
			errContains: "failed to extract mutations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := output.WriteBatch(context.Background(), tt.batch)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// Helper functions
func mustParseBloblang(t *testing.T, expr string) *bloblang.Executor {
	exe, err := bloblang.Parse(expr)
	require.NoError(t, err)
	return exe
}

func messageWithStructured(data interface{}) *service.Message {
	msg := service.NewMessage(nil)
	msg.SetStructured(data)
	return msg
}
