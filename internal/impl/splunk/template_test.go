package splunk

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TemplateDefinition represents the structure of a Benthos template
type TemplateDefinition struct {
	Name        string            `yaml:"name"`
	Type        string            `yaml:"type"`
	Status      string            `yaml:"status"`
	Categories  []string          `yaml:"categories"`
	Summary     string            `yaml:"summary"`
	Description string            `yaml:"description"`
	Fields      []FieldDefinition `yaml:"fields"`
	Mapping     string            `yaml:"mapping"`
	Tests       []TemplateTest    `yaml:"tests"`
}

type FieldDefinition struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Type        string      `yaml:"type"`
	Default     interface{} `yaml:"default,omitempty"`
	Advanced    bool        `yaml:"advanced,omitempty"`
}

type TemplateTest struct {
	Name     string                 `yaml:"name"`
	Config   map[string]interface{} `yaml:"config"`
	Expected map[string]interface{} `yaml:"expected"`
}

func TestTemplateStructure(t *testing.T) {
	var template TemplateDefinition
	err := yaml.Unmarshal(outputTemplate, &template)
	require.NoError(t, err, "Template should unmarshal to TemplateDefinition")

	t.Run("metadata", func(t *testing.T) {
		assert.Equal(t, "splunk_hec", template.Name)
		assert.Equal(t, "output", template.Type)
		assert.Equal(t, "experimental", template.Status)
		assert.Contains(t, template.Categories, "Services")
		assert.NotEmpty(t, template.Summary)
		assert.Contains(t, template.Summary, "Splunk")
		assert.Contains(t, template.Description, "HTTP Endpoint Collector")
	})

	t.Run("fields", func(t *testing.T) {
		assert.Len(t, template.Fields, 13, "Should have 13 fields defined")

		// Validate required fields
		requiredFields := map[string]struct {
			fieldType    string
			hasDefault   bool
			defaultValue interface{}
		}{
			"url":                {fieldType: "string", hasDefault: false},
			"token":              {fieldType: "string", hasDefault: false},
			"gzip":               {fieldType: "bool", hasDefault: true, defaultValue: false},
			"event_host":         {fieldType: "string", hasDefault: true, defaultValue: ""},
			"event_source":       {fieldType: "string", hasDefault: true, defaultValue: ""},
			"event_sourcetype":   {fieldType: "string", hasDefault: true, defaultValue: ""},
			"event_index":        {fieldType: "string", hasDefault: true, defaultValue: ""},
			"batching_count":     {fieldType: "int", hasDefault: true, defaultValue: 100},
			"batching_period":    {fieldType: "string", hasDefault: true, defaultValue: "30s"},
			"batching_byte_size": {fieldType: "int", hasDefault: true, defaultValue: 1000000},
			"rate_limit":         {fieldType: "string", hasDefault: true, defaultValue: ""},
			"max_in_flight":      {fieldType: "int", hasDefault: true, defaultValue: 64},
			"skip_cert_verify":   {fieldType: "bool", hasDefault: true, defaultValue: false},
		}

		for _, field := range template.Fields {
			if expected, ok := requiredFields[field.Name]; ok {
				assert.Equal(t, expected.fieldType, field.Type, "Field %s type mismatch", field.Name)
				assert.NotEmpty(t, field.Description, "Field %s should have description", field.Name)

				if expected.hasDefault {
					assert.Equal(t, expected.defaultValue, field.Default, "Field %s default mismatch", field.Name)
				}
			}
		}

		// Check advanced fields
		advancedFields := []string{"rate_limit", "max_in_flight", "skip_cert_verify"}
		for _, field := range template.Fields {
			if contains(advancedFields, field.Name) {
				assert.True(t, field.Advanced, "Field %s should be marked as advanced", field.Name)
			}
		}
	})

	t.Run("mapping", func(t *testing.T) {
		assert.NotEmpty(t, template.Mapping)

		// Verify key mappings exist
		assert.Contains(t, template.Mapping, "root.http_client.url = this.url")
		assert.Contains(t, template.Mapping, "root.http_client.verb = \"POST\"")
		assert.Contains(t, template.Mapping, "root.http_client.headers.Authorization = \"Splunk \" + this.token")
		assert.Contains(t, template.Mapping, "root.http_client.headers.\"Content-Type\" = \"application/json\"")

		// Verify conditional gzip handling
		assert.Contains(t, template.Mapping, "if this.gzip { \"gzip\"}")
		assert.Contains(t, template.Mapping, "if this.gzip {{ \"algorithm\": \"gzip\" }}")

		// Verify batching configuration
		assert.Contains(t, template.Mapping, "root.http_client.batching.count = this.batching_count")
		assert.Contains(t, template.Mapping, "root.http_client.batching.period = this.batching_period")
		assert.Contains(t, template.Mapping, "root.http_client.batching.byte_size = this.batching_byte_size")

		// Verify TLS configuration
		assert.Contains(t, template.Mapping, "root.http_client.tls.enabled = true")
		assert.Contains(t, template.Mapping, "root.http_client.tls.skip_cert_verify = this.skip_cert_verify")

		// Verify processor configuration
		assert.Contains(t, template.Mapping, "root.processors")
		assert.Contains(t, template.Mapping, "bloblang")
	})

	t.Run("embedded tests", func(t *testing.T) {
		assert.Len(t, template.Tests, 2, "Should have 2 test cases")

		// Test 1: Basic fields
		test1 := template.Tests[0]
		assert.Equal(t, "Basic fields", test1.Name)
		assert.Equal(t, "https://foobar.splunkcloud.com/services/collector/event", test1.Config["url"])
		assert.Equal(t, "footoken", test1.Config["token"])
		assert.Equal(t, false, test1.Config["gzip"])

		// Validate expected output structure
		httpClient := test1.Expected["http_client"].(map[string]interface{})
		assert.Equal(t, "https://foobar.splunkcloud.com/services/collector/event", httpClient["url"])
		assert.Equal(t, "POST", httpClient["verb"])

		headers := httpClient["headers"].(map[string]interface{})
		assert.Equal(t, "application/json", headers["Content-Type"])
		assert.Equal(t, "Splunk footoken", headers["Authorization"])
		assert.NotContains(t, headers, "Content-Encoding", "Should not have Content-Encoding when gzip is false")

		// Test 2: With gzip
		test2 := template.Tests[1]
		assert.Equal(t, "gzip", test2.Name)
		assert.Equal(t, true, test2.Config["gzip"])

		httpClient2 := test2.Expected["http_client"].(map[string]interface{})
		headers2 := httpClient2["headers"].(map[string]interface{})
		assert.Equal(t, "gzip", headers2["Content-Encoding"], "Should have Content-Encoding when gzip is true")

		batching2 := httpClient2["batching"].(map[string]interface{})
		processors2 := batching2["processors"].([]interface{})
		assert.Len(t, processors2, 2, "Should have archive and compress processors when gzip is true")
	})
}

func TestBloblangProcessor(t *testing.T) {
	var template TemplateDefinition
	err := yaml.Unmarshal(outputTemplate, &template)
	require.NoError(t, err)

	// Extract the bloblang processor from the mapping
	lines := strings.Split(template.Mapping, "\n")
	inBloblang := false
	var bloblangLines []string

	for _, line := range lines {
		if strings.Contains(line, "root.processors") && strings.Contains(line, "bloblang") {
			inBloblang = true
			continue
		}
		if inBloblang {
			if strings.TrimSpace(line) == `"""` {
				if len(bloblangLines) > 0 {
					break
				}
				continue
			}
			bloblangLines = append(bloblangLines, line)
		}
	}

	bloblangScript := strings.Join(bloblangLines, "\n")

	t.Run("handles event wrapping", func(t *testing.T) {
		// Check that the script wraps non-event messages
		assert.Contains(t, bloblangScript, `root = if (this | {}).exists("event") { this } else {`)
		assert.Contains(t, bloblangScript, `{ "event": content().string() }`)
	})

	t.Run("sets metadata fields conditionally", func(t *testing.T) {
		// Check conditional field setting
		assert.Contains(t, bloblangScript, `root.host = if $config_host != "" { $config_host }`)
		assert.Contains(t, bloblangScript, `root.source = if $config_source != "" { $config_source}`)
		assert.Contains(t, bloblangScript, `root.sourcetype = if $config_sourcetype != "" { $config_sourcetype }`)
		assert.Contains(t, bloblangScript, `root.index = if $config_index != "" { $config_index }`)
	})

	t.Run("uses format placeholders", func(t *testing.T) {
		// The mapping should contain format placeholders
		assert.Contains(t, template.Mapping, `.format(this.event_host, this.event_source, this.event_sourcetype, this.event_index)`)
	})
}

func TestBatchingConfiguration(t *testing.T) {
	var template TemplateDefinition
	err := yaml.Unmarshal(outputTemplate, &template)
	require.NoError(t, err)

	t.Run("batching defaults", func(t *testing.T) {
		// Find batching fields and verify defaults
		batchingFields := map[string]interface{}{
			"batching_count":     100,
			"batching_period":    "30s",
			"batching_byte_size": 1000000,
		}

		for _, field := range template.Fields {
			if expected, ok := batchingFields[field.Name]; ok {
				assert.Equal(t, expected, field.Default, "Field %s default mismatch", field.Name)
			}
		}
	})

	t.Run("batching processors", func(t *testing.T) {
		// Verify archive processor is always added
		assert.Contains(t, template.Mapping, `root.http_client.batching.processors."-".archive = { "format": "lines" }`)

		// Verify compress processor is conditional
		assert.Contains(t, template.Mapping, `root.http_client.batching.processors."-".compress = if this.gzip {{ "algorithm": "gzip" }}`)
	})
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
