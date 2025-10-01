package mqtt3

import (
	"crypto/tls"
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputConfigParsing(t *testing.T) {
	spec := inputConfig()

	t.Run("basic config", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
filters:
  test/topic: 1
client_id: test-client
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Create mock resources
		mgr := service.MockResources()

		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		// Verify the config was parsed correctly
		i := inputObj.(*input)
		assert.Equal(t, "test-client", i.inputConfig.MqttConfig.ClientId)
		assert.Equal(t, []string{"tcp://localhost:1883"}, i.inputConfig.Urls)
		assert.Equal(t, byte(1), i.inputConfig.Filters["test/topic"])
	})

	t.Run("with TLS config", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
filters:
  test/topic: 1
tls:
  enabled: true
  skip_cert_verify: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		// Verify TLS was configured
		i := inputObj.(*input)
		assert.NotNil(t, i.inputConfig.TLS)
		assert.True(t, i.inputConfig.TLS.InsecureSkipVerify)
	})

	t.Run("with Will config", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
filters:
  test/topic: 1
will:
  topic: last/will
  payload: goodbye
  qos: 1
  retained: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		// Verify Will was configured
		i := inputObj.(*input)
		require.NotNil(t, i.inputConfig.Will)
		assert.Equal(t, "last/will", i.inputConfig.Will.Topic)
		assert.Equal(t, "goodbye", i.inputConfig.Will.Payload)
		assert.Equal(t, uint8(1), i.inputConfig.Will.QoS)
		assert.True(t, i.inputConfig.Will.Retained)
	})

	t.Run("with auth, TLS and Will", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
filters:
  test/topic: 1
auth:
  username: testuser
  password: testpass
tls:
  enabled: true
will:
  topic: status/offline
  payload: client disconnected
  qos: 2
  retained: false
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		// Verify all configs were set
		i := inputObj.(*input)
		assert.Equal(t, "testuser", i.inputConfig.Username)
		assert.Equal(t, "testpass", i.inputConfig.Password)
		assert.NotNil(t, i.inputConfig.TLS)
		require.NotNil(t, i.inputConfig.Will)
		assert.Equal(t, "status/offline", i.inputConfig.Will.Topic)
		assert.Equal(t, uint8(2), i.inputConfig.Will.QoS)
		assert.False(t, i.inputConfig.Will.Retained)
	})

	t.Run("missing required filters", func(t *testing.T) {
		conf := `
urls:
  - tcp://localhost:1883
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()

		_, err = newInput(parsedConf, mgr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least one topic filter")
	})

	t.Run("TLS config with cert files", func(t *testing.T) {
		// Note: TLS cert files are validated at connection time, not parse time
		conf := `
urls:
  - tcp://localhost:1883
filters:
  test/topic: 1
tls:
  enabled: true
  cert_file: /nonexistent/cert.pem
  key_file: /nonexistent/key.pem
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		// Parsing should succeed - cert validation happens later
		require.NoError(t, err)

		mgr := service.MockResources()
		_, err = newInput(parsedConf, mgr)
		// Creation should succeed - connection will fail later
		assert.NoError(t, err)
	})
}

func TestInputConfigPassthrough(t *testing.T) {
	// This test would require mocking the mqtt.NewInput function
	// to verify that the config is passed correctly
	// For now, the tests above verify that we build the config correctly

	t.Run("verify TLS passthrough", func(t *testing.T) {
		// Verify that when TLS is configured, it's passed to mqtt.InputConfig
		conf := `
urls:
  - tcp://localhost:1883
filters:
  test/topic: 1
tls:
  enabled: true
`
		parsedConf, err := inputConfig().ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()
		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)

		i := inputObj.(*input)
		assert.IsType(t, &tls.Config{}, i.inputConfig.TLS)
	})
}
