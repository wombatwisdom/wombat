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

		// Create output config with interpolated queue_name
		outputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: 'DEV.QUEUE.${! json("queue_num") }'
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

		// Send a test message with queue_num in JSON structure
		testMessage := service.NewMessage([]byte("Hello IBM MQ!"))
		testMessage.MetaSetMut("test_id", "123")
		testMessage.SetStructured(map[string]interface{}{
			"queue_num": 1,
			"content":   "Hello IBM MQ!",
		})

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

		// Verify message content - it should be the JSON structure
		receivedMsg := receivedBatch[0]
		content, err := receivedMsg.AsBytes()
		require.NoError(t, err)
		assert.JSONEq(t, `{"content":"Hello IBM MQ!","queue_num":1}`, string(content))

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

	t.Run("batch_size_exact_match", func(t *testing.T) {
		connectionString, _ := startIBMMQContainer(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		mgr := service.MockResources()

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

		output, bp, maxInFlight, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)
		assert.Equal(t, 1, bp.Count)
		assert.Equal(t, 1, maxInFlight)

		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Send exactly 5 messages
		batchSize := 5
		for i := 0; i < batchSize; i++ {
			msg := service.NewMessage([]byte(fmt.Sprintf("batch_test_message_%d", i)))
			batch := service.MessageBatch{msg}
			err = output.WriteBatch(ctx, batch)
			require.NoError(t, err)
		}

		// Create input with batch_size = 5
		inputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
batch_size: %d
wait_time: 1s
enable_auto_ack: true
`, connectionString, batchSize)

		inputSpec := inputConfig()
		parsedInputConf, err := inputSpec.ParseYAML(inputConf, nil)
		require.NoError(t, err)

		input, err := newInput(parsedInputConf, mgr)
		require.NoError(t, err)

		err = input.Connect(ctx)
		require.NoError(t, err)
		defer input.Close(ctx)

		// Read batch - should get exactly 5 messages
		readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
		receivedBatch, _, err := input.ReadBatch(readCtx)
		readCancel()

		require.NoError(t, err)
		assert.Equal(t, batchSize, len(receivedBatch), "Should receive exact batch size")

		// Verify message contents
		for i, msg := range receivedBatch {
			expectedContent := fmt.Sprintf("batch_test_message_%d", i)
			content, err := msg.AsBytes()
			require.NoError(t, err)
			actualContent := string(content)
			assert.Equal(t, expectedContent, actualContent, "Message %d content mismatch", i)
		}
	})

	t.Run("batch_wait_timeout", func(t *testing.T) {
		connectionString, _ := startIBMMQContainer(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		mgr := service.MockResources()

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

		output, _, _, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)

		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Send only 3 messages (less than batch_size)
		for i := 0; i < 3; i++ {
			msg := service.NewMessage([]byte(fmt.Sprintf("timeout_test_message_%d", i)))
			batch := service.MessageBatch{msg}
			err = output.WriteBatch(ctx, batch)
			require.NoError(t, err)
		}

		// Create input with batch_size = 5, wait_time = 2s
		inputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
batch_size: 5
wait_time: 2s
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

		// Read batch - should wait for wait_time trying to fill to batch_size, then return partial batch
		startTime := time.Now()
		readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
		receivedBatch, _, err := input.ReadBatch(readCtx)
		readCancel()
		elapsed := time.Since(startTime)

		// Should get partial batch without error
		require.NoError(t, err, "Should successfully read partial batch")
		assert.Equal(t, 3, len(receivedBatch), "Should receive partial batch of 3 messages")

		// Check that it waited approximately the wait_time before returning partial batch
		assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "Should wait at least wait_time")
		assert.LessOrEqual(t, elapsed, 3*time.Second, "Should not wait much longer than wait_time")

		// Verify message contents
		for i, msg := range receivedBatch {
			expectedContent := fmt.Sprintf("timeout_test_message_%d", i)
			content, err := msg.AsBytes()
			require.NoError(t, err)
			actualContent := string(content)
			assert.Equal(t, expectedContent, actualContent, "Message %d content mismatch", i)
		}
	})

	t.Run("multiple_complete_batches", func(t *testing.T) {
		connectionString, _ := startIBMMQContainer(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		mgr := service.MockResources()

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

		output, _, _, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)

		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Send 10 messages
		totalMessages := 10
		for i := 0; i < totalMessages; i++ {
			msg := service.NewMessage([]byte(fmt.Sprintf("multi_batch_message_%d", i)))
			batch := service.MessageBatch{msg}
			err = output.WriteBatch(ctx, batch)
			require.NoError(t, err)
		}

		// Create input with batch_size = 3
		batchSize := 3
		inputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
batch_size: %d
wait_time: 1s
enable_auto_ack: true
`, connectionString, batchSize)

		inputSpec := inputConfig()
		parsedInputConf, err := inputSpec.ParseYAML(inputConf, nil)
		require.NoError(t, err)

		input, err := newInput(parsedInputConf, mgr)
		require.NoError(t, err)

		err = input.Connect(ctx)
		require.NoError(t, err)
		defer input.Close(ctx)

		// Read multiple batches
		totalRead := 0
		batchCount := 0
		expectedBatches := 4 // 3 complete batches of 3 + 1 partial batch of 1

		for totalRead < totalMessages && batchCount < expectedBatches {
			readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
			receivedBatch, _, err := input.ReadBatch(readCtx)
			readCancel()

			if err != nil {
				break
			}

			batchCount++
			totalRead += len(receivedBatch)

			// First 3 batches should be complete (size = 3)
			if batchCount <= 3 {
				assert.Equal(t, batchSize, len(receivedBatch),
					"Batch %d should be complete with %d messages", batchCount, batchSize)
			} else {
				// Last batch should be partial (size = 1)
				assert.Equal(t, 1, len(receivedBatch),
					"Last batch should contain remaining message")
			}
		}

		assert.Equal(t, totalMessages, totalRead, "Should read all %d messages", totalMessages)
		assert.Equal(t, expectedBatches, batchCount, "Should have %d batches", expectedBatches)
	})

	t.Run("batch_size_one_behaves_as_single_message", func(t *testing.T) {
		connectionString, _ := startIBMMQContainer(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		mgr := service.MockResources()

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

		output, _, _, err := newOutput(parsedOutputConf, mgr)
		require.NoError(t, err)

		err = output.Connect(ctx)
		require.NoError(t, err)
		defer output.Close(ctx)

		// Send 3 messages
		for i := 0; i < 3; i++ {
			msg := service.NewMessage([]byte(fmt.Sprintf("single_test_message_%d", i)))
			batch := service.MessageBatch{msg}
			err = output.WriteBatch(ctx, batch)
			require.NoError(t, err)
		}

		// Create input with batch_size = 1 (default)
		inputConf := fmt.Sprintf(`
queue_manager_name: QM1
queue_name: DEV.QUEUE.1
channel_name: DEV.APP.SVRCONN
connection_name: %s
user_id: app
password: passw0rd
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

		// Read messages one by one
		for i := 0; i < 3; i++ {
			readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
			receivedBatch, _, err := input.ReadBatch(readCtx)
			readCancel()

			require.NoError(t, err)
			assert.Equal(t, 1, len(receivedBatch), "Batch size 1 should return single message")

			expectedContent := fmt.Sprintf("single_test_message_%d", i)
			content, err := receivedBatch[0].AsBytes()
			require.NoError(t, err)
			actualContent := string(content)
			assert.Equal(t, expectedContent, actualContent, "Message %d content mismatch", i)
		}
	})
}
