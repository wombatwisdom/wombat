//go:build mqclient

package ibmmq

import (
	"fmt"
	"strings"
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputConfigParsing(t *testing.T) {
	spec := inputConfig()

	t.Run("basic config", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Verify configuration fields can be extracted
		qmName, err := parsedConf.FieldString(fldQueueManagerName)
		assert.NoError(t, err)
		assert.Equal(t, "QM1", qmName)

		qName, err := parsedConf.FieldString(fldQueueName)
		assert.NoError(t, err)
		assert.Equal(t, "DEV.QUEUE.1", qName)

		chName, err := parsedConf.FieldString(fldChannelName)
		assert.NoError(t, err)
		assert.Equal(t, "DEV.APP.SVRCONN", chName)

		connName, err := parsedConf.FieldString(fldConnectionName)
		assert.NoError(t, err)
		assert.Equal(t, "localhost(1414)", connName)
	})

	t.Run("config with authentication", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
user_id: mquser
password: mqpass
application_name: myapp
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		userId, err := parsedConf.FieldString(fldUserId)
		assert.NoError(t, err)
		assert.Equal(t, "mquser", userId)

		password, err := parsedConf.FieldString(fldPassword)
		assert.NoError(t, err)
		assert.Equal(t, "mqpass", password)

		appName, err := parsedConf.FieldString(fldApplicationName)
		assert.NoError(t, err)
		assert.Equal(t, "myapp", appName)
	})

	t.Run("config with TLS", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
tls:
  enabled: true
  cipher_spec: TLS_RSA_WITH_AES_256_CBC_SHA256
  key_repository: /opt/mqm/ssl/key
  key_repository_password: keypass
  certificate_label: ibmwebspheremqapp
  ssl_peer_name: CN=*.example.com
  fips_required: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Verify TLS configuration exists
		tlsRaw, err := parsedConf.FieldAny(fldTLS)
		require.NoError(t, err)

		tlsMap, ok := tlsRaw.(map[string]interface{})
		require.True(t, ok)

		enabled, ok := tlsMap[fldTLSEnabled].(bool)
		assert.True(t, ok)
		assert.True(t, enabled)

		cipherSpec, ok := tlsMap[fldTLSCipherSpec].(string)
		assert.True(t, ok)
		assert.Equal(t, "TLS_RSA_WITH_AES_256_CBC_SHA256", cipherSpec)

		keyRepo, ok := tlsMap[fldTLSKeyRepository].(string)
		assert.True(t, ok)
		assert.Equal(t, "/opt/mqm/ssl/key", keyRepo)

		keyPass, ok := tlsMap[fldTLSKeyRepoPass].(string)
		assert.True(t, ok)
		assert.Equal(t, "keypass", keyPass)

		certLabel, ok := tlsMap[fldTLSCertLabel].(string)
		assert.True(t, ok)
		assert.Equal(t, "ibmwebspheremqapp", certLabel)

		peerName, ok := tlsMap[fldTLSPeerName].(string)
		assert.True(t, ok)
		assert.Equal(t, "CN=*.example.com", peerName)

		fips, ok := tlsMap[fldTLSFipsRequired].(bool)
		assert.True(t, ok)
		assert.True(t, fips)
	})

	t.Run("config with processing options", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
batch_size: 10
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		batchSize, err := parsedConf.FieldInt(fldBatchSize)
		assert.NoError(t, err)
		assert.Equal(t, 10, batchSize)

		assert.NoError(t, err)
	})

	t.Run("missing required fields", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
`
		_, err := spec.ParseYAML(conf, nil)
		require.Error(t, err)
		// Either queue_name or connection_name could be reported as missing
		assert.True(t,
			strings.Contains(err.Error(), "queue_name") ||
				strings.Contains(err.Error(), "connection_name") || strings.Contains(err.Error(), "channel_name"),
			"Expected error about missing required field, got: %s", err.Error())
	})

	t.Run("default values", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Check default values
		mgr := service.MockResources()
		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		// Check input defaults
		i := inputObj.(*input)
		assert.Equal(t, "wombat", i.inputConfig.ApplicationName)
		assert.Equal(t, 1, i.inputConfig.BatchSize)
		assert.Equal(t, "100ms", i.inputConfig.BatchWaitTime)
	})

	t.Run("config with batching parameters and policy", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
batch_size: 25
batch_wait_time: 30s
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Create input and verify batch configuration
		mgr := service.MockResources()
		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		// Check that BatchSize is properly configured
		i := inputObj.(*input)
		assert.Equal(t, 25, i.inputConfig.BatchSize)
		assert.Equal(t, "30s", i.inputConfig.BatchWaitTime)

		// Verify BatchPolicy reflects batch_size
		bp := service.BatchPolicy{Count: 1}
		batchSize, _ := parsedConf.FieldInt(fldBatchSize)
		if batchSize <= 0 {
			batchSize = 1
		}
		bp.Count = batchSize
		assert.Equal(t, 25, bp.Count)
	})

	t.Run("batch_wait_time duration parsing", func(t *testing.T) {
		testCases := []struct {
			waitTime     string
			expectedTime string
		}{
			{"100ms", "100ms"},
			{"1s", "1s"},
			{"5m", "5m0s"},
			{"1h30m", "1h30m0s"},
		}

		for _, tc := range testCases {
			conf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
batch_wait_time: %s
`, tc.waitTime)
			parsedConf, err := spec.ParseYAML(conf, nil)
			require.NoError(t, err)

			mgr := service.MockResources()
			inputObj, err := newInput(parsedConf, mgr)
			require.NoError(t, err)

			i := inputObj.(*input)
			assert.Equal(t, tc.expectedTime, i.inputConfig.BatchWaitTime,
				"Expected batch_wait_time %s to be parsed as %s", tc.waitTime, tc.expectedTime)
		}
	})

	t.Run("large batch_size configuration", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
batch_size: 1000
batch_wait_time: 1m
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()
		inputObj, err := newInput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, inputObj)

		i := inputObj.(*input)
		assert.Equal(t, 1000, i.inputConfig.BatchSize)
		assert.Equal(t, "1m0s", i.inputConfig.BatchWaitTime)
	})
}
