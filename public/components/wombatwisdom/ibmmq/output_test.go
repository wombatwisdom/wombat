package ibmmq

import (
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputConfigParsing(t *testing.T) {
	spec := outputConfig()

	t.Run("basic config with static queue", func(t *testing.T) {
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

		qName, err := parsedConf.FieldString(fldOutputQueueName)
		assert.NoError(t, err)
		assert.Equal(t, "DEV.QUEUE.1", qName)

		chName, err := parsedConf.FieldString(fldChannelName)
		assert.NoError(t, err)
		assert.Equal(t, "DEV.APP.SVRCONN", chName)

		connName, err := parsedConf.FieldString(fldConnectionName)
		assert.NoError(t, err)
		assert.Equal(t, "localhost(1414)", connName)
	})

	t.Run("config with dynamic queue expression", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_expr: '${! meta("target_queue") }'
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Verify queue expression exists
		assert.True(t, parsedConf.Contains(fldQueueName))

		// Create output to verify expression parsing
		mgr := service.MockResources()
		outputObj, bp, maxInFlight, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, outputObj)

		o := outputObj.(*output)
		assert.NotNil(t, o.queueExpr)
		assert.Equal(t, 1, bp.Count)
		assert.Equal(t, 1, maxInFlight)
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
connection_name: secure-mq.example.com(1414)
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
	})

	t.Run("config with metadata filtering", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
metadata:
  patterns:
    - "mq_*"
    - "jms_*"
  invert: true
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		// Verify metadata configuration exists
		metaRaw, err := parsedConf.FieldAny(fldMetadata)
		require.NoError(t, err)

		metaMap, ok := metaRaw.(map[string]interface{})
		require.True(t, ok)

		patternsRaw, ok := metaMap[fldMetaPatterns].([]interface{})
		assert.True(t, ok)
		assert.Len(t, patternsRaw, 2)
		assert.Equal(t, "mq_*", patternsRaw[0])
		assert.Equal(t, "jms_*", patternsRaw[1])

		invert, ok := metaMap[fldMetaInvert].(bool)
		assert.True(t, ok)
		assert.True(t, invert)
	})

	t.Run("config with processing options", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: localhost(1414)
num_threads: 4
write_timeout: 60s
`
		parsedConf, err := spec.ParseYAML(conf, nil)
		require.NoError(t, err)

		numThreads, err := parsedConf.FieldInt(fldOutputNumThreads)
		assert.NoError(t, err)
		assert.Equal(t, 4, numThreads)

		writeTimeout, err := parsedConf.FieldDuration(fldWriteTimeout)
		assert.NoError(t, err)
		assert.Equal(t, "1m0s", writeTimeout.String())
	})

	t.Run("missing required fields", func(t *testing.T) {
		conf := `
queue_manager_name: QM1
`
		_, err := spec.ParseYAML(conf, nil)
		require.Error(t, err)
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
		outputObj, bp, maxInFlight, err := newOutput(parsedConf, mgr)
		require.NoError(t, err)
		require.NotNil(t, outputObj)

		// Check batch policy defaults
		assert.Equal(t, 1, bp.Count)
		assert.Equal(t, 1, maxInFlight)

		// Check output defaults
		o := outputObj.(*output)
		assert.Equal(t, "wombat", o.outputConfig.ApplicationName)
		assert.Equal(t, 1, o.outputConfig.NumThreads)
		assert.Equal(t, "30s", o.writeTimeout.String())
	})
}
