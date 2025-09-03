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
		// This test verifies two behaviors:
		// 1. Benthos automatically retries connections on ANY error
		// 2. When wombatwisdom/components errors are properly translated to Benthos errors, then we should see them in Benthos logging

		// Start TCP listener to count connection retries
		listener, err := net.Listen("tcp", "127.0.0.1:65500")
		require.NoError(t, err)
		defer listener.Close()

		attemptCount := int32(0)
		listenerDone := make(chan struct{})

		go func() {
			defer close(listenerDone)
			for {
				conn, err := listener.Accept()
				if err != nil {
					return // Listener closed
				}
				atomic.AddInt32(&attemptCount, 1)
				conn.Close() // Immediately close to simulate connection failure
			}
		}()

		logCap := &logCapture{}

		config := `
input:
  ww_mqtt_3:
    urls:
      - tcp://127.0.0.1:65500
    filters:
      "test/topic": 1
    clean_session: true

output:
  drop: {}  # We don't need output for this test
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

		// Verify automatic retry behavior (Benthos framework feature)
		attempts := atomic.LoadInt32(&attemptCount)
		t.Logf("Detected %d connection attempts", attempts)
		assert.GreaterOrEqual(t, attempts, int32(2), "Should have at least 2 connection attempts - Benthos automatically retries on any connection error")

		// Verify our error translation is working
		messages := logCap.getMessages()
		t.Logf("Captured %d log messages", len(messages))
		for i, msg := range messages {
			t.Logf("Log[%d]: %s", i, msg)
		}

		foundTranslatedError := false
		foundRawError := false

		for _, msg := range messages {
			if strings.Contains(msg, "not connected to target source or sink") {
				foundTranslatedError = true
			}
			if strings.Contains(msg, "network Error") && strings.Contains(msg, "connection reset by peer") {
				foundRawError = true
			}
		}

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
}
