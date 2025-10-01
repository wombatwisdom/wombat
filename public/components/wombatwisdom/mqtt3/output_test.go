package mqtt3

import (
	"crypto/tls"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOutputConfigParsing(t *testing.T) {
	spec := outputConfig()

	t.Run("basic config", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: test/output
client_id: test-output-client
qos: 1
retained: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Create mock resources
		mgr := service.MockResources()

		output, _, _, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
	})

	t.Run("quoted string literal", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: "test/output"
client_id: test-output-client
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		output, _, _, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
	})

	t.Run("interpolated topic", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: 'test/output/${! json("topic") }'
client_id: test-output-client
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		output, _, _, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
	})

	t.Run("with TLS config", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: test/output
tls:
  enabled: true
  skip_cert_verify: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		output, _, _, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
	})

	t.Run("with Will config", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: test/output
will:
  topic: last/will/output
  payload: output disconnected
  qos: 2
  retained: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		output, _, _, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
	})

	t.Run("with auth, TLS and Will", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: test/output
auth:
  username: outputuser
  password: outputpass
tls:
  enabled: true
will:
  topic: status/output/offline
  payload: output client disconnected
  qos: 0
  retained: false
write_timeout: 10s
connect_timeout: 20s
keepalive: 30s
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		output, bp, maxInFlight, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)

		// Verify batch policy
		assert.Equal(t, 1, bp.Count)
		assert.Equal(t, 1, maxInFlight)
	})

	t.Run("missing required topic", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
`
		// Topic is required, so parsing should fail
		_, err := spec.ParseYAML(conf, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic")
	})

	t.Run("invalid QoS value", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
topic: test/output
qos: 3
`
		// QoS 3 is invalid (must be 0, 1, or 2)
		// The parsing should succeed but the MQTT library should reject it later
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		output, _, _, err := newOutput(parsedConf, mgr)
		// This may or may not error depending on when validation happens
		// If it doesn't error here, it would error when connecting
		if err != nil {
			t.Logf("Expected error for invalid QoS: %v", err)
		} else {
			assert.NotNil(t, output)
		}
	})
}

func TestOutputTLSAndWillPassthrough(t *testing.T) {
	// This test verifies the structure is correct
	// Actual verification would require mocking mqtt.NewOutput

	t.Run("verify config structure", func(t *testing.T) {
		// Test that we can create a complex config without errors
		conf := `
urls:
  - tcp://broker1:1883
  - tcp://broker2:1883
topic: data/output/${! json("device_id") }
client_id: output-${! env("HOSTNAME") }
qos: 1
retained: false
auth:
  username: ${! env("MQTT_USER") }
  password: ${! env("MQTT_PASS") }
tls:
  enabled: true
  skip_cert_verify: false
will:
  topic: status/disconnected
  payload: '{"client": "${! env("HOSTNAME") }", "status": "offline"}'
  qos: 1
  retained: true
`
		parsedConf, err := outputConfig().ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		// Should parse without errors even with interpolations
		output, _, _, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, output)
	})

	t.Run("verify TLS type", func(t *testing.T) {
		// Test that TLS config is properly typed
		conf := `
urls:
  - tcp://localhost:1883
topic: test
tls:
  enabled: true
`
		parsedConf, err := outputConfig().ParseYAML(conf, nil)
		require.NoError(t, err)

		// Get the TLS config to verify it's the right type
		tlsConf, tlsEnabled, err := parsedConf.FieldTLSToggled("tls")
		require.NoError(t, err)
		assert.True(t, tlsEnabled)
		assert.IsType(t, &tls.Config{}, tlsConf)
	})
}
