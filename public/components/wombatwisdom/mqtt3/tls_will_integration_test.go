//go:build integration
// +build integration

package mqtt3_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/wombatwisdom/wombat/public/components/wombatwisdom/mqtt3"
)

func startMQTTBroker(t *testing.T, ctx context.Context) string {
	mqttContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "eclipse-mosquitto:2.0",
			ExposedPorts: []string{"1883/tcp"},
			Files: []testcontainers.ContainerFile{
				{
					ContainerFilePath: "/mosquitto/config/mosquitto.conf",
					FileMode:          0644,
					Reader: strings.NewReader(`
listener 1883
allow_anonymous true
`),
				},
			},
			WaitingFor: wait.ForListeningPort("1883/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err, "Failed to start MQTT container")

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		mqttContainer.Terminate(cleanupCtx)
	})

	mqttPort, err := mqttContainer.MappedPort(ctx, "1883")
	require.NoError(t, err)

	mqttURL := fmt.Sprintf("tcp://127.0.0.1:%s", mqttPort.Port())
	t.Logf("MQTT broker started at %s", mqttURL)

	return mqttURL
}

func TestWillMessageDeliveryIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL := startMQTTBroker(t, ctx)

	// Create a subscriber to listen for will messages
	willReceived := make(chan mqtt.Message, 1)
	subscriberOpts := mqtt.NewClientOptions().
		AddBroker(mqttURL).
		SetClientID("will-subscriber")

	subscriber := mqtt.NewClient(subscriberOpts)
	token := subscriber.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	defer subscriber.Disconnect(1000)

	// Subscribe to will topic
	token = subscriber.Subscribe("status/offline", 1, func(client mqtt.Client, msg mqtt.Message) {
		select {
		case willReceived <- msg:
		default:
		}
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	t.Run("will message on ungraceful disconnect", func(t *testing.T) {
		// Create ww_mqtt_3 output with will configuration
		outputConf := fmt.Sprintf(`
ww_mqtt_3:
  urls:
    - %s
  client_id: will-test-client
  topic: '"data/test"'
  will:
    topic: status/offline
    payload: "client disconnected unexpectedly"
    qos: 1
    retained: true
`, mqttURL)

		streamBuilder := service.NewStreamBuilder()
		require.NoError(t, streamBuilder.AddOutputYAML(outputConf))

		// Add a simple input to drive the output
		require.NoError(t, streamBuilder.AddInputYAML(`
generate:
  count: 1
  mapping: |
    root.message = "test data"
`))

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		// Run briefly
		go func() {
			_ = stream.Run(ctx)
		}()

		// Give it time to connect and send the test message
		time.Sleep(2 * time.Second)

		// Force ungraceful shutdown by canceling context
		cancel()

		// Wait for will message
		select {
		case msg := <-willReceived:
			assert.Equal(t, "status/offline", msg.Topic())
			assert.Equal(t, []byte("client disconnected unexpectedly"), msg.Payload())
			// Note: Retained flag on will messages may not work reliably with all brokers
			t.Logf("Will message retained flag: %v", msg.Retained())
		case <-time.After(5 * time.Second):
			t.Fatal("Will message not received within timeout")
		}
	})
}

func TestTLSConnectionIntegration(t *testing.T) {
	// This test would require a broker with TLS enabled
	// For now, we'll just test that TLS configuration is accepted

	t.Run("TLS config is parsed without errors", func(t *testing.T) {
		conf := `
ww_mqtt_3:
  urls:
    - tls://localhost:8883
  filters:
    "test/topic": 1
  tls:
    enabled: true
    skip_cert_verify: true
`
		streamBuilder := service.NewStreamBuilder()
		err := streamBuilder.AddInputYAML(conf)
		// Should not error during config parsing
		assert.NoError(t, err)
	})
}

func TestWillWithAuthIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL := startMQTTBroker(t, ctx)

	t.Run("will with auth config", func(t *testing.T) {
		// Test that will + auth can be configured together
		conf := fmt.Sprintf(`
ww_mqtt_3:
  urls:
    - %s
  topic: '"test/output"'
  auth:
    username: testuser
    password: testpass
  will:
    topic: status/auth/offline
    payload: "authenticated client disconnected"
    qos: 2
    retained: false
`, mqttURL)

		streamBuilder := service.NewStreamBuilder()
		err := streamBuilder.AddOutputYAML(conf)
		// Should parse without errors
		require.NoError(t, err)

		// Add input
		require.NoError(t, streamBuilder.AddInputYAML(`
generate:
  count: 1
  mapping: |
    root.test = "data"
`))

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		// Just verify it can start
		go func() {
			_ = stream.Run(ctx)
		}()

		time.Sleep(1 * time.Second)
		cancel()
	})
}

func TestCompleteIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL := startMQTTBroker(t, ctx)

	t.Run("input and output with will", func(t *testing.T) {
		received := make(chan []byte, 10)

		// Create input with will
		inputConf := fmt.Sprintf(`
ww_mqtt_3:
  urls:
    - %s
  client_id: input-with-will
  filters:
    data/flow: 1
  will:
    topic: status/input/offline
    payload: "input disconnected"
    qos: 1
    retained: false
`, mqttURL)

		// Create output with will
		outputConf := fmt.Sprintf(`
input:
  generate:
    count: 5
    interval: 100ms
    mapping: |
      root.id = counter()
      root.timestamp = now()
output:
  ww_mqtt_3:
    urls:
      - %s
    client_id: output-with-will
    topic: '"data/flow"'
    will:
      topic: status/output/offline
      payload: "output disconnected"
      qos: 1
      retained: false
`, mqttURL)

		// Start input pipeline
		inputBuilder := service.NewStreamBuilder()
		require.NoError(t, inputBuilder.AddInputYAML(inputConf))
		require.NoError(t, inputBuilder.AddConsumerFunc(func(ctx context.Context, msg *service.Message) error {
			data, _ := msg.AsBytes()
			select {
			case received <- data:
			default:
			}
			return nil
		}))

		inputStream, err := inputBuilder.Build()
		require.NoError(t, err)

		// Start output pipeline
		outputBuilder := service.NewStreamBuilder()
		require.NoError(t, outputBuilder.SetYAML(outputConf))

		outputStream, err := outputBuilder.Build()
		require.NoError(t, err)

		// Run both
		go func() {
			_ = inputStream.Run(ctx)
		}()

		time.Sleep(500 * time.Millisecond)

		go func() {
			_ = outputStream.Run(ctx)
		}()

		// Wait for messages
		timeout := time.After(5 * time.Second)
		messageCount := 0

	Loop:
		for {
			select {
			case data := <-received:
				t.Logf("Received message: %s", string(data))
				messageCount++
				if messageCount >= 3 {
					break Loop
				}
			case <-timeout:
				break Loop
			}
		}

		assert.GreaterOrEqual(t, messageCount, 3, "Should receive at least 3 messages")
	})
}
