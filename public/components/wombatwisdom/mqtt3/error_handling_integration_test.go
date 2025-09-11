//go:build integration
// +build integration

package mqtt3_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/redpanda-data/benthos/v4/public/components/io"
	_ "github.com/redpanda-data/benthos/v4/public/components/pure"

	_ "github.com/wombatwisdom/wombat/public/components/all"
)

// logCapture implements PrintLogger interface to capture log messages
type logCapture struct {
	mu       sync.Mutex
	messages []string
}

func (l *logCapture) Printf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, v...))
}

func (l *logCapture) Println(v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintln(v...))
}

func (l *logCapture) getMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.messages...)
}

func TestErrorHandlingIntegration(t *testing.T) {
	integration.CheckSkip(t)

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	natsReq := testcontainers.ContainerRequest{
		Image:        "nats:latest",
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForListeningPort("4222/tcp"),
	}
	natsContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: natsReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer natsContainer.Terminate(ctx)

	natsPort, err := natsContainer.MappedPort(ctx, "4222")
	require.NoError(t, err)
	natsURL := fmt.Sprintf("nats://localhost:%s", natsPort.Port())

	t.Run("should_retry_on_connection_failure", func(t *testing.T) {
		// This test verifies two behaviors for both input and output:
		// 1. Benthos automatically retries connections on ANY error
		// 2. When wombatwisdom/components errors are properly translated to Benthos errors, then we should see them in Benthos logging

		// Start TCP listener for INPUT to count connection retries
		inputListener, err := net.Listen("tcp", "127.0.0.1:65500")
		require.NoError(t, err)
		defer inputListener.Close()

		inputAttemptCount := int32(0)
		inputListenerDone := make(chan struct{})

		go func() {
			defer close(inputListenerDone)
			for {
				conn, err := inputListener.Accept()
				if err != nil {
					return // Listener closed
				}
				atomic.AddInt32(&inputAttemptCount, 1)
				conn.Close() // Immediately close to simulate connection failure
			}
		}()

		// Start TCP listener for OUTPUT to count connection retries
		outputListener, err := net.Listen("tcp", "127.0.0.1:65501")
		require.NoError(t, err)
		defer outputListener.Close()

		outputAttemptCount := int32(0)
		outputListenerDone := make(chan struct{})

		go func() {
			defer close(outputListenerDone)
			for {
				conn, err := outputListener.Accept()
				if err != nil {
					return // Listener closed
				}
				atomic.AddInt32(&outputAttemptCount, 1)
				conn.Close() // Immediately close to simulate connection failure
			}
		}()

		logCap := &logCapture{}

		// Pipeline with both failing input and output
		config := `
input:
  ww_mqtt_3:
    urls:
      - tcp://127.0.0.1:65500
    filters:
      "test/topic": 1
    clean_session: true
    client_id: test_input

output:
  ww_mqtt_3:
    urls:
      - tcp://127.0.0.1:65501
    topic: "output/topic"
    client_id: test_output
`

		builder := service.NewStreamBuilder()
		builder.SetPrintLogger(logCap)
		err = builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		testCtx, testCancel := context.WithTimeout(ctx, 5*time.Second)
		defer testCancel()

		go stream.Run(testCtx)
		defer stream.Stop(testCtx)

		// Wait and check for multiple connection attempts
		time.Sleep(3 * time.Second)

		// Verify automatic retry behavior for INPUT (Benthos framework feature)
		inputAttempts := atomic.LoadInt32(&inputAttemptCount)
		t.Logf("Detected %d INPUT connection attempts", inputAttempts)
		assert.GreaterOrEqual(t, inputAttempts, int32(2), "Should have at least 2 INPUT connection attempts - Benthos automatically retries on any connection error")

		// Verify automatic retry behavior for OUTPUT (Benthos framework feature)
		outputAttempts := atomic.LoadInt32(&outputAttemptCount)
		t.Logf("Detected %d OUTPUT connection attempts", outputAttempts)
		assert.GreaterOrEqual(t, outputAttempts, int32(2), "Should have at least 2 OUTPUT connection attempts - Benthos automatically retries on any connection error")

		// Verify our error translation is working for both input and output
		messages := logCap.getMessages()
		t.Logf("Captured %d log messages", len(messages))
		for i, msg := range messages {
			t.Logf("Log[%d]: %s", i, msg)
		}

		foundTranslatedError := false
		foundRawError := false

		for _, msg := range messages {
			if strings.Contains(msg, "not connected to target source or sink") {
				if strings.Contains(msg, "ww_mqtt_3") && strings.Contains(msg, "Failed to connect") {
					// Could be input or output, check for both
					foundTranslatedError = true
				}
			}
			// Check for raw network errors that shouldn't appear
			if strings.Contains(msg, "network Error") && strings.Contains(msg, "connection reset by peer") {
				foundRawError = true
			}
		}

		// Since both input and output use the same error translation, we expect to see the translated error
		assert.True(t, foundTranslatedError, "Should see translated error message 'not connected to target source or sink' - our error translation is working")
		assert.False(t, foundRawError, "Should NOT see raw network error 'connection reset by peer' - error should be translated")
	})

	t.Run("should_never_return_ErrEndOfInput_for_MQTT", func(t *testing.T) {
		mosqReq := testcontainers.ContainerRequest{
			Image:        "eclipse-mosquitto:2.0",
			ExposedPorts: []string{"1883/tcp"},
			Files: []testcontainers.ContainerFile{
				{
					ContainerFilePath: "/mosquitto/config/mosquitto.conf",
					FileMode:          0644,
					Reader: strings.NewReader(`persistence false
allow_anonymous true
listener 1883`),
				},
			},
			WaitingFor: wait.ForListeningPort("1883/tcp"),
		}
		mosqContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: mosqReq,
			Started:          true,
		})
		require.NoError(t, err)
		defer mosqContainer.Terminate(ctx)

		mosqPort, err := mosqContainer.MappedPort(ctx, "1883")
		require.NoError(t, err)
		mqttURL := fmt.Sprintf("tcp://localhost:%s", mosqPort.Port())

		nc, err := nats.Connect(natsURL)
		require.NoError(t, err)
		defer nc.Close()

		statusChan := make(chan *nats.Msg, 10)
		sub, err := nc.ChanSubscribe("mqtt.pipeline.status", statusChan)
		require.NoError(t, err)
		defer sub.Unsubscribe()

		config := `
input:
  ww_mqtt_3:
    urls:
      - %s
    filters:
      "empty/topic": 1  # Topic with no messages
    clean_session: true

output:
  nats:
    urls: [%s]
    subject: mqtt.pipeline.status
    max_in_flight: 1
`

		pipelineConfig := fmt.Sprintf(config, mqttURL, natsURL)
		builder := service.NewStreamBuilder()
		err = builder.SetYAML(pipelineConfig)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		testCtx, testCancel := context.WithTimeout(ctx, 5*time.Second)
		defer testCancel()

		// Capture the stream.Run() result
		streamErr := make(chan error, 1)
		go func() {
			err := stream.Run(testCtx)
			streamErr <- err
		}()
		defer stream.Stop(testCtx)

		// Let it run for 3 seconds
		time.Sleep(3 * time.Second)

		// Check if stream exited early with an error
		select {
		case err := <-streamErr:
			// If stream exited, make sure it's NOT ErrEndOfInput
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				if errors.Is(err, service.ErrEndOfInput) {
					t.Fatal("MQTT input should never return ErrEndOfInput")
				}
				t.Fatalf("Stream exited with unexpected error: %v", err)
			}
			// Stream exited due to context cancellation - this is fine
		default:
			t.Log("Stream is still running as expected (no ErrEndOfInput)")
		}

		// Verify no messages were received on the empty topic
		select {
		case <-statusChan:
			t.Fatal("Should not have received any messages on empty topic")
		default:
		}
	})

	t.Run("should_shutdown_cleanly_after_processing_messages", func(t *testing.T) {
		// MQTT uses a persistent subscription model where Read() blocks indefinitely.
		// We should handle context cancellation to allow clean pipeline shutdown.
		mosqReq := testcontainers.ContainerRequest{
			Image:        "eclipse-mosquitto:2.0",
			ExposedPorts: []string{"1883/tcp"},
			Files: []testcontainers.ContainerFile{
				{
					ContainerFilePath: "/mosquitto/config/mosquitto.conf",
					FileMode:          0644,
					Reader: strings.NewReader(`persistence false
allow_anonymous true
listener 1883`),
				},
			},
			WaitingFor: wait.ForListeningPort("1883/tcp"),
		}
		mosqContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: mosqReq,
			Started:          true,
		})
		require.NoError(t, err)
		defer mosqContainer.Terminate(ctx)

		mosqPort, err := mosqContainer.MappedPort(ctx, "1883")
		require.NoError(t, err)
		mqttURL := fmt.Sprintf("tcp://localhost:%s", mosqPort.Port())

		config := fmt.Sprintf(`
input:
  ww_mqtt_3:
    urls: [%s]
    filters:
      "test/shutdown": 1
    client_id: shutdown_test
    clean_session: false

pipeline:
  processors:
    - mapping: |
        root = this
        meta processed_at = now()

output:
  drop: {}

shutdown_timeout: 2s
`, mqttURL)

		logCap := &logCapture{}
		builder := service.NewStreamBuilder()
		builder.SetPrintLogger(logCap)
		err = builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		streamCtx, streamCancel := context.WithCancel(ctx)

		// Track when pipeline starts
		pipelineReady := make(chan struct{})
		go func() {
			// Wait for pipeline to be ready
			for i := 0; i < 20; i++ {
				messages := logCap.getMessages()
				for _, msg := range messages {
					if strings.Contains(msg, "Input type ww_mqtt_3 is now active") {
						close(pipelineReady)
						return
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()

		// Run pipeline
		go stream.Run(streamCtx)

		// Wait for pipeline to be ready
		select {
		case <-pipelineReady:
			t.Log("Pipeline is ready")
		case <-time.After(3 * time.Second):
			t.Fatal("Pipeline failed to start")
		}

		// Now attempt shutdown
		t.Log("Initiating shutdown...")
		streamCancel()

		shutdownStart := time.Now()
		shutdownDone := make(chan error, 1)

		go func() {
			shutdownDone <- stream.Stop(context.Background())
		}()

		// Wait for shutdown with timeout
		select {
		case <-shutdownDone:
			shutdownDuration := time.Since(shutdownStart)
			t.Logf("Shutdown completed in %v", shutdownDuration)

			// Check for forced termination warning
			foundWarning := false
			messages := logCap.getMessages()
			for _, msg := range messages {
				if strings.Contains(msg, "prevented forced termination") {
					foundWarning = true
					t.Log("Found forced termination warning")
					break
				}
			}

			// After fix: Expect clean shutdown without warnings
			assert.False(t, foundWarning, "Should not have forced termination warning after fix")
			assert.Less(t, shutdownDuration.Seconds(), 1.0, "Should shutdown quickly with context cancellation")

		case <-time.After(5 * time.Second):
			t.Fatal("Shutdown timed out after 5 seconds")
		}
	})
}
