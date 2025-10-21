//go:build mqclient

package ibmmq

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startIBMMQContainer(t *testing.T) (string, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "icr.io/ibm-messaging/mq:latest",
		ExposedPorts: []string{"1414/tcp", "9443/tcp"},
		Env: map[string]string{
			"LICENSE":         "accept",
			"MQ_QMGR_NAME":    "QM1",
			"MQ_DEV":          "true",     // Enable developer defaults
			"MQ_APP_PASSWORD": "passw0rd", // #nosec G101 - testcontainer default credential
		},
		WaitingFor: wait.ForLog("Started web server").WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "1414")
	require.NoError(t, err)

	connectionString := fmt.Sprintf("%s(%s)", host, mappedPort.Port())

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	// Wait a bit for MQ to be fully ready
	time.Sleep(5 * time.Second)

	return connectionString, cleanup
}

func TestIBMMQIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connectionString, cleanup := startIBMMQContainer(t)
	defer cleanup()

	t.Run("basic message flow", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create output config
		outputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
`, connectionString)

		outputSpec := outputConfig()
		parsedOutputConf, err := outputSpec.ParseYAML(outputConf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()
		output, _, _, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)

		// Connect output
		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Create input config
		inputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
enable_auto_ack: false
`, connectionString)

		inputSpec := inputConfig()
		parsedInputConf, err := inputSpec.ParseYAML(inputConf, nil)
		require.NoError(t, err)

		input, err := newInput(parsedInputConf, mgr)
		require.NoError(t, err)

		// Connect input
		err = input.Connect(ctx)
		require.NoError(t, err)
		defer input.Close(ctx)

		// Send a test message
		testMessage := service.NewMessage([]byte("Hello IBM MQ!"))
		testMessage.MetaSetMut("test_id", "123")

		batch := service.MessageBatch{testMessage}
		err = output.WriteBatch(ctx, batch)
		require.NoError(t, err)

		// Read the message back
		readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
		defer readCancel()

		receivedBatch, ackFunc, err := input.ReadBatch(readCtx)
		require.NoError(t, err)
		require.NotNil(t, receivedBatch)
		require.Len(t, receivedBatch, 1)

		// Verify message content
		receivedMsg := receivedBatch[0]
		content, err := receivedMsg.AsBytes()
		require.NoError(t, err)
		assert.Equal(t, "Hello IBM MQ!", string(content))

		// Acknowledge the message
		err = ackFunc(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("message with metadata", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create output with metadata filtering
		outputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
metadata:
  patterns:
    - "app_*"
  invert: false
`, connectionString)

		outputSpec := outputConfig()
		parsedOutputConf, err := outputSpec.ParseYAML(outputConf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()
		output, _, _, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)

		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Send message with metadata
		testMessage := service.NewMessage([]byte("Message with metadata"))
		testMessage.MetaSetMut("app_version", "1.0.0")
		testMessage.MetaSetMut("app_user", "testuser")
		testMessage.MetaSetMut("internal_id", "should_be_filtered")

		batch := service.MessageBatch{testMessage}
		err = output.WriteBatch(ctx, batch)
		require.NoError(t, err)
	})

	t.Run("batch processing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create output
		outputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
`, connectionString)

		outputSpec := outputConfig()
		parsedOutputConf, err := outputSpec.ParseYAML(outputConf, nil)
		require.NoError(t, err)

		mgr := service.MockResources()
		output, _, _, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)

		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Send batch of messages
		batch := service.MessageBatch{}
		for i := 0; i < 5; i++ {
			msg := service.NewMessage([]byte(fmt.Sprintf("Batch message %d", i)))
			msg.MetaSetMut("index", fmt.Sprintf("%d", i))
			batch = append(batch, msg)
		}

		err = output.WriteBatch(ctx, batch)
		require.NoError(t, err)

		// Create input with batch size
		inputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
batch_size: 3
enable_auto_ack: true
`, connectionString)

		inputSpec := inputConfig()
		parsedInputConf, err := inputSpec.ParseYAML(inputConf, nil)
		require.NoError(t, err)

		input, err := newInput(parsedInputConf, mgr)
		require.NoError(t, err)

		err = input.Connect(ctx)
		require.NoError(t, err)
		defer input.Close(ctx)

		// Read messages
		messagesRead := 0
		for messagesRead < 5 {
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
			receivedBatch, _, err := input.ReadBatch(readCtx)
			readCancel()

			if err != nil {
				break
			}

			messagesRead += len(receivedBatch)
		}

		assert.GreaterOrEqual(t, messagesRead, 5, "Should have read all 5 messages")
	})
}
