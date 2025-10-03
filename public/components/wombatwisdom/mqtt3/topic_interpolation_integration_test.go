//go:build integration
// +build integration

package mqtt3_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/wombatwisdom/wombat/public/components/wombatwisdom/mqtt3"
)

func startMochiMQTTServer(t *testing.T) (string, func()) {
	server := mochi.New(nil)
	// Allow all connections
	_ = server.AddHook(new(auth.AllowHook), nil)

	// Find a free port
	port := 0
	for p := 11883; p < 12000; p++ {
		tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: fmt.Sprintf(":%d", p)})
		if err := server.AddListener(tcp); err == nil {
			port = p
			break
		}
	}
	require.NotEqual(t, 0, port, "Failed to find free port for MQTT server")

	go func() {
		if err := server.Serve(); err != nil {
			t.Logf("MQTT server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	url := fmt.Sprintf("tcp://localhost:%d", port)
	t.Logf("Started Mochi MQTT server at %s", url)

	cleanup := func() {
		_ = server.Close()
	}

	return url, cleanup
}

type receivedMessage struct {
	Topic   string
	Payload []byte
}

func createSubscriber(t *testing.T, mqttURL string, topics []string) (chan receivedMessage, mqtt.Client) {
	received := make(chan receivedMessage, 10)

	opts := mqtt.NewClientOptions().
		AddBroker(mqttURL).
		SetClientID("test-subscriber").
		SetAutoReconnect(false)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())

	// Subscribe to all topics
	for _, topic := range topics {
		token = client.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
			select {
			case received <- receivedMessage{
				Topic:   msg.Topic(),
				Payload: msg.Payload(),
			}:
			default:
				t.Logf("Warning: received channel full, dropping message")
			}
		})
		require.True(t, token.WaitTimeout(5*time.Second))
		require.NoError(t, token.Error())
		t.Logf("Subscribed to topic: %s", topic)
	}

	return received, client
}

func TestTopicInterpolationIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL, cleanup := startMochiMQTTServer(t)
	defer cleanup()

	t.Run("static topic", func(t *testing.T) {
		received, subscriber := createSubscriber(t, mqttURL, []string{"test/static"})
		defer subscriber.Disconnect(250)

		// Create wombat pipeline with static topic
		conf := fmt.Sprintf(`
input:
  generate:
    count: 1
    mapping: |
      root.message = "static topic test"
      root.id = 123

output:
  ww_mqtt_3:
    urls:
      - %s
    client_id: static-test-client
    topic: "test/static"
    qos: 1
`, mqttURL)

		streamBuilder := service.NewStreamBuilder()
		require.NoError(t, streamBuilder.SetYAML(conf))

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			_ = stream.Run(ctx)
		}()

		// Wait for message
		select {
		case msg := <-received:
			assert.Equal(t, "test/static", msg.Topic)

			var payload map[string]interface{}
			err := json.Unmarshal(msg.Payload, &payload)
			require.NoError(t, err)
			assert.Equal(t, "static topic test", payload["message"])
			assert.Equal(t, float64(123), payload["id"])
		case <-time.After(5 * time.Second):
			t.Fatal("Did not receive message within timeout")
		}
	})

	t.Run("interpolated topic with JSON field", func(t *testing.T) {
		// Subscribe to multiple possible topics
		topics := []string{
			"test/output/temperature",
			"test/output/humidity",
			"test/output/pressure",
		}
		received, subscriber := createSubscriber(t, mqttURL, topics)
		defer subscriber.Disconnect(250)

		// Create wombat pipeline with interpolated topic
		conf := fmt.Sprintf(`
input:
  generate:
    count: 3
    interval: 100ms
    mapping: |
      root.sensor_type = ["temperature", "humidity", "pressure"].index(counter() %% 3)
      root.value = random_int() %% 100
      root.timestamp = now()

output:
  ww_mqtt_3:
    urls:
      - %s
    client_id: interpolation-test-client
    topic: 'test/output/${! json("sensor_type") }'
    qos: 1
`, mqttURL)

		streamBuilder := service.NewStreamBuilder()
		require.NoError(t, streamBuilder.SetYAML(conf))

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			_ = stream.Run(ctx)
		}()

		// Collect messages
		messages := make(map[string]int)
		mu := sync.Mutex{}

		timeout := time.After(10 * time.Second)
		for i := 0; i < 3; i++ {
			select {
			case msg := <-received:
				mu.Lock()
				messages[msg.Topic]++
				mu.Unlock()

				var payload map[string]interface{}
				err := json.Unmarshal(msg.Payload, &payload)
				require.NoError(t, err)

				// Verify the sensor_type matches the topic
				expectedTopic := fmt.Sprintf("test/output/%s", payload["sensor_type"])
				assert.Equal(t, expectedTopic, msg.Topic)

				t.Logf("Received message on topic %s with sensor_type %s", msg.Topic, payload["sensor_type"])
			case <-timeout:
				t.Fatalf("Did not receive all messages within timeout. Got %d messages", i)
			}
		}

		// Verify we got messages on all expected topics
		assert.Equal(t, 1, messages["test/output/temperature"])
		assert.Equal(t, 1, messages["test/output/humidity"])
		assert.Equal(t, 1, messages["test/output/pressure"])
	})

	t.Run("interpolated topic with metadata", func(t *testing.T) {
		received, subscriber := createSubscriber(t, mqttURL, []string{"test/meta/device-001", "test/meta/device-002"})
		defer subscriber.Disconnect(250)

		// Create wombat pipeline with metadata interpolation
		conf := fmt.Sprintf(`
input:
  generate:
    count: 2
    interval: 100ms
    mapping: |
      root.data = "test data " + counter().string()
      meta device_id = if (counter() %% 2) == 0 { "device-001" } else { "device-002" }

output:
  ww_mqtt_3:
    urls:
      - %s
    client_id: meta-test-client
    topic: 'test/meta/${! meta("device_id") }'
    qos: 1
`, mqttURL)

		streamBuilder := service.NewStreamBuilder()
		require.NoError(t, streamBuilder.SetYAML(conf))

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			_ = stream.Run(ctx)
		}()

		// Collect messages
		receivedTopics := make(map[string]bool)
		for i := 0; i < 2; i++ {
			select {
			case msg := <-received:
				receivedTopics[msg.Topic] = true
				t.Logf("Received message on topic: %s", msg.Topic)
			case <-time.After(5 * time.Second):
				t.Fatalf("Did not receive all messages within timeout. Got %d messages", i)
			}
		}

		// Verify we got messages on both device topics
		assert.True(t, receivedTopics["test/meta/device-001"])
		assert.True(t, receivedTopics["test/meta/device-002"])
	})

	t.Run("complex interpolation", func(t *testing.T) {
		// Subscribe to the complex topic pattern
		topics := []string{
			"test/warehouse/temperature/data",
			"test/office/humidity/data",
		}
		received, subscriber := createSubscriber(t, mqttURL, topics)
		defer subscriber.Disconnect(250)

		// Create wombat pipeline with complex interpolation
		conf := fmt.Sprintf(`
input:
  generate:
    count: 2
    interval: 100ms
    mapping: |
      root.location = if (counter() %% 2) == 0 { "warehouse" } else { "office" }
      root.sensor_type = if (counter() %% 2) == 0 { "temperature" } else { "humidity" }
      root.reading = random_int() %% 100

output:
  ww_mqtt_3:
    urls:
      - %s
    client_id: complex-test-client
    topic: 'test/${! json("location") }/${! json("sensor_type") }/data'
    qos: 1
`, mqttURL)

		streamBuilder := service.NewStreamBuilder()
		require.NoError(t, streamBuilder.SetYAML(conf))

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			_ = stream.Run(ctx)
		}()

		// Collect messages
		receivedMessages := make(map[string]map[string]interface{})
		for i := 0; i < 2; i++ {
			select {
			case msg := <-received:
				var payload map[string]interface{}
				err := json.Unmarshal(msg.Payload, &payload)
				require.NoError(t, err)

				receivedMessages[msg.Topic] = payload
				t.Logf("Received on %s: location=%s, sensor_type=%s",
					msg.Topic, payload["location"], payload["sensor_type"])
			case <-time.After(5 * time.Second):
				t.Fatalf("Did not receive all messages within timeout. Got %d messages", i)
			}
		}

		// Verify the messages
		warehouseMsg, ok := receivedMessages["test/warehouse/temperature/data"]
		assert.True(t, ok)
		assert.Equal(t, "warehouse", warehouseMsg["location"])
		assert.Equal(t, "temperature", warehouseMsg["sensor_type"])

		officeMsg, ok := receivedMessages["test/office/humidity/data"]
		assert.True(t, ok)
		assert.Equal(t, "office", officeMsg["location"])
		assert.Equal(t, "humidity", officeMsg["sensor_type"])
	})
}
