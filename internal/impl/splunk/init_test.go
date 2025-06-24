package splunk

import (
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSplunkTemplateRegistration(t *testing.T) {
	// The init() function should have already registered the template
	// Since the template is registered during init(), we just need to verify
	// the template content is valid and properly structured
	
	t.Run("template is loaded and valid", func(t *testing.T) {
		// Verify template content exists
		require.NotEmpty(t, outputTemplate, "Template should be loaded from embedded file")
		
		// Verify it's valid YAML
		var data map[string]interface{}
		err := yaml.Unmarshal(outputTemplate, &data)
		require.NoError(t, err, "Template should be valid YAML")
		
		// Verify it has the expected structure
		assert.Equal(t, "splunk_hec", data["name"])
		assert.Equal(t, "output", data["type"])
		assert.Contains(t, data, "fields")
		assert.Contains(t, data, "mapping")
	})
}

func TestSplunkTemplateContent(t *testing.T) {
	t.Run("template content is valid YAML", func(t *testing.T) {
		require.NotEmpty(t, outputTemplate, "Template content should not be empty")
		
		// Parse the template as YAML to ensure it's valid
		var templateData map[string]interface{}
		err := yaml.Unmarshal(outputTemplate, &templateData)
		require.NoError(t, err, "Template should be valid YAML")
		
		// Verify required fields exist
		assert.Contains(t, templateData, "name")
		assert.Equal(t, "splunk_hec", templateData["name"])
		
		assert.Contains(t, templateData, "type")
		assert.Equal(t, "output", templateData["type"])
		
		assert.Contains(t, templateData, "fields")
		fields, ok := templateData["fields"].([]interface{})
		require.True(t, ok, "fields should be an array")
		assert.Greater(t, len(fields), 0, "Should have field definitions")
		
		assert.Contains(t, templateData, "mapping")
		assert.Contains(t, templateData, "tests")
	})
	
	t.Run("template fields are properly defined", func(t *testing.T) {
		var templateData map[string]interface{}
		err := yaml.Unmarshal(outputTemplate, &templateData)
		require.NoError(t, err)
		
		fields, ok := templateData["fields"].([]interface{})
		require.True(t, ok)
		
		// Expected fields
		expectedFields := map[string]bool{
			"url":               false,
			"token":             false,
			"gzip":              false,
			"event_host":        false,
			"event_source":      false,
			"event_sourcetype":  false,
			"event_index":       false,
			"batching_count":    false,
			"batching_period":   false,
			"batching_byte_size": false,
			"rate_limit":        false,
			"max_in_flight":     false,
			"skip_cert_verify":  false,
		}
		
		// Check all expected fields are present
		for _, fieldRaw := range fields {
			field, ok := fieldRaw.(map[string]interface{})
			require.True(t, ok)
			
			name, ok := field["name"].(string)
			require.True(t, ok)
			
			if _, expected := expectedFields[name]; expected {
				expectedFields[name] = true
				
				// Verify field has required attributes
				assert.Contains(t, field, "type", "Field %s should have a type", name)
				assert.Contains(t, field, "description", "Field %s should have a description", name)
			}
		}
		
		// Ensure all expected fields were found
		for name, found := range expectedFields {
			assert.True(t, found, "Expected field %s was not found", name)
		}
	})
}

func TestSplunkTemplateMapping(t *testing.T) {
	// Test the template mapping logic by examining the template content
	t.Run("mapping generates correct configuration", func(t *testing.T) {
		var templateData map[string]interface{}
		err := yaml.Unmarshal(outputTemplate, &templateData)
		require.NoError(t, err)
		
		mapping, ok := templateData["mapping"].(string)
		require.True(t, ok, "Template should have mapping field")
		
		// Verify HTTP client configuration
		assert.Contains(t, mapping, "root.http_client.url = this.url")
		assert.Contains(t, mapping, "root.http_client.verb = \"POST\"")
		assert.Contains(t, mapping, "root.http_client.headers.\"Content-Type\" = \"application/json\"")
		assert.Contains(t, mapping, "root.http_client.headers.Authorization = \"Splunk \" + this.token")
		
		// Verify conditional gzip
		assert.Contains(t, mapping, "root.http_client.headers.\"Content-Encoding\" = if this.gzip { \"gzip\"}")
		
		// Verify batching configuration
		assert.Contains(t, mapping, "root.http_client.batching.count = this.batching_count")
		assert.Contains(t, mapping, "root.http_client.batching.period = this.batching_period")
		assert.Contains(t, mapping, "root.http_client.batching.byte_size = this.batching_byte_size")
		
		// Verify TLS configuration
		assert.Contains(t, mapping, "root.http_client.tls.enabled = true")
		assert.Contains(t, mapping, "root.http_client.tls.skip_cert_verify = this.skip_cert_verify")
	})
}


func TestSplunkTemplateTests(t *testing.T) {
	t.Run("embedded tests are valid", func(t *testing.T) {
		var templateData map[string]interface{}
		err := yaml.Unmarshal(outputTemplate, &templateData)
		require.NoError(t, err)
		
		tests, ok := templateData["tests"].([]interface{})
		require.True(t, ok, "Template should have tests")
		require.Greater(t, len(tests), 0, "Should have at least one test")
		
		for i, testRaw := range tests {
			test, ok := testRaw.(map[string]interface{})
			require.True(t, ok, "Test %d should be a map", i)
			
			assert.Contains(t, test, "name", "Test %d should have a name", i)
			assert.Contains(t, test, "config", "Test %d should have a config", i)
			assert.Contains(t, test, "expected", "Test %d should have expected output", i)
			
			// Verify the config is valid
			config, ok := test["config"].(map[string]interface{})
			require.True(t, ok, "Test %d config should be a map", i)
			assert.Contains(t, config, "url", "Test %d config should have url", i)
			assert.Contains(t, config, "token", "Test %d config should have token", i)
		}
	})
}

func TestSplunkInit(t *testing.T) {
	t.Run("init registers template without panic", func(t *testing.T) {
		// If we get here, init() has already run successfully
		// This test verifies that the template registration doesn't panic
		assert.NotPanics(t, func() {
			// Try to use service functions that would fail if template wasn't registered
			_ = service.NewEnvironment()
		})
	})
	
	t.Run("template registration error handling", func(t *testing.T) {
		// Test what happens with invalid template
		// Since RegisterTemplateYAML panics on error, we test that behavior
		assert.Panics(t, func() {
			// Try to register invalid YAML
			err := service.RegisterTemplateYAML("invalid yaml {{{")
			if err != nil {
				panic(err)
			}
		})
	})
}