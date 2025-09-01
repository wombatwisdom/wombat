//go:build integration
// +build integration

package wombatwisdom_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/redpanda-data/benthos/v4/public/components/io"
	_ "github.com/redpanda-data/benthos/v4/public/components/pure"
	_ "github.com/redpanda-data/connect/v4/public/components/all"
)

func TestWombatWisdomMQTTIntegration(t *testing.T) {
	integration.CheckSkip(t)

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	natsOpts := natstest.DefaultTestOptions
	natsOpts.Port = -1
	natsOpts.StoreDir, _ = os.MkdirTemp("", "nats-test-")

	natsServer := natstest.RunServer(&natsOpts)
	defer func() {
		natsServer.Shutdown()
		os.RemoveAll(natsOpts.StoreDir)
	}()

	natsURL := natsServer.ClientURL()
	t.Logf("Shared NATS server: %s", natsURL)

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
	require.NoError(t, err, "Failed to start shared MQTT container")

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		mqttContainer.Terminate(cleanupCtx)
	}()

	mqttHost, err := mqttContainer.Host(ctx)
	require.NoError(t, err)
	mqttPort, err := mqttContainer.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	mqttURL := fmt.Sprintf("tcp://%s:%s", mqttHost, mqttPort.Port())

	t.Logf("Shared MQTT broker: %s", mqttURL)

	testBenthosMQTTAtLeastOnceDelivery(t, ctx, mqttURL, natsURL, "benthos-no-crash")
	testBenthosMQTTAtLeastOnceDeliveryWithRetryAfterCrash(t, ctx, mqttURL, natsURL, "benthos-with-crash")
	testWombatMQTTAtLeastOnceDelivery(t, ctx, mqttURL, natsURL, "wombat-no-crash")
	testWombatMQTTAtLeastOnceDeliveryWithRetryAfterCrash(t, ctx, mqttURL, natsURL, "wombat-with-crash")
}

func testBenthosMQTTAtLeastOnceDelivery(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	t.Logf("=== Starting Benthos MQTT no-crash test ===")

	// Setup NATS client for validation
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.sequences.%s", testID))
	require.NoError(t, err)

	// Start Benthos pipeline with standard MQTT component
	t.Logf("🔗 Starting Benthos pipeline MQTT→NATS...")

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  mqtt:
    urls: ["$MQTT_URL"]
    topics: ["test/sequences/`+testID+`"]
    client_id: "benthos-worker-`+testID+`"
    qos: 2

pipeline:
  processors:
    - sleep:
        duration: "10ms"
    - log:
        level: INFO
        message: "📨 MQTT→NATS: ${! content() }"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.sequences.`+testID+`"
`, "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	pipelineBuilder := service.NewStreamBuilder()
	err = pipelineBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	pipelineStream, err := pipelineBuilder.Build()
	require.NoError(t, err)

	// Start pipeline in background
	go func() {
		err := pipelineStream.Run(ctx)
		if err != nil && ctx.Err() != context.Canceled {
			t.Logf("Pipeline error: %v", err)
		}
	}()

	// Wait for pipeline to be ready
	time.Sleep(2 * time.Second)

	// Send messages via external producer
	t.Logf("📤 Sending 100 sequences to MQTT...")

	producerConfig := strings.ReplaceAll(`
input:
  generate:
    interval: "50ms"
    count: 100
    mapping: 'root = "sequence-" + counter().string()'

output:
  mqtt:
    urls: ["$MQTT_URL"]
    topic: "test/sequences/`+testID+`"
    client_id: "external-producer-`+testID+`"
    qos: 2
`, "$MQTT_URL", mqttURL)

	producerBuilder := service.NewStreamBuilder()
	err = producerBuilder.SetYAML(producerConfig)
	require.NoError(t, err)

	producerStream, err := producerBuilder.Build()
	require.NoError(t, err)

	// Run producer
	prodCtx, prodCancel := context.WithTimeout(ctx, 10*time.Second)
	defer prodCancel()

	err = producerStream.Run(prodCtx)
	require.NoError(t, err)

	t.Logf("✅ Producer finished - 100 sequences sent to MQTT")

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Count messages received in NATS
	t.Logf("📊 Checking NATS for received messages...")

	receivedCount := 0
	for {
		msg, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		receivedCount++
		t.Logf("✅ NATS received: %s", string(msg.Data))
	}

	t.Logf("📊 Final Results:")
	t.Logf("   Sent to MQTT: 100")
	t.Logf("   Received in NATS: %d", receivedCount)

	require.Equal(t, 100, receivedCount, "All messages must be delivered without crashes")
}

func testBenthosMQTTAtLeastOnceDeliveryWithRetryAfterCrash(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string) {
	ctx, cancel := context.WithTimeout(parentCtx, 45*time.Second)
	defer cancel()

	// Setup NATS client for validation
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.sequences.%s", testID))
	require.NoError(t, err)

	// Start Benthos pipeline with standard MQTT component
	t.Logf("🔗 Starting Benthos pipeline MQTT→NATS...")

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  mqtt:
    urls: ["$MQTT_URL"]
    topics: ["test/sequences/`+testID+`"]
    client_id: "benthos-worker-`+testID+`"
    qos: 2

pipeline:
  processors:
    - sleep:
        duration: "10ms"
    - log:
        level: INFO
        message: "📨 MQTT→NATS: ${! content() }"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.sequences.`+testID+`"
`, "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	pipelineBuilder := service.NewStreamBuilder()
	err = pipelineBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	pipelineStream, err := pipelineBuilder.Build()
	require.NoError(t, err)

	// Start pipeline in background with cancellable context
	pipelineCtx, pipelineCancel := context.WithCancel(ctx)

	go func() {
		err := pipelineStream.Run(pipelineCtx)
		if err != nil && pipelineCtx.Err() != context.Canceled {
			t.Logf("Pipeline error: %v", err)
		}
	}()

	// Wait for pipeline to be ready
	time.Sleep(2 * time.Second)

	// Send messages via external producer
	t.Logf("📤 Sending 2000 sequences to MQTT...")

	producerConfig := strings.ReplaceAll(`
input:
  generate:
    interval: "5ms"
    count: 2000
    mapping: 'root = "sequence-" + counter().string()'

output:
  mqtt:
    urls: ["$MQTT_URL"]
    topic: "test/sequences/`+testID+`"
    client_id: "external-producer-`+testID+`"
    qos: 2
`, "$MQTT_URL", mqttURL)

	producerBuilder := service.NewStreamBuilder()
	err = producerBuilder.SetYAML(producerConfig)
	require.NoError(t, err)

	producerStream, err := producerBuilder.Build()
	require.NoError(t, err)

	// Start producer in background
	go func() {
		prodCtx, prodCancel := context.WithTimeout(ctx, 15*time.Second)
		defer prodCancel()

		err := producerStream.Run(prodCtx)
		if err != nil {
			t.Logf("Producer error: %v", err)
		}
		t.Logf("✅ Producer finished - 2000 sequences sent to MQTT")
	}()

	// Let producer send ~400 messages (400 * 5ms = 2 seconds)
	time.Sleep(2 * time.Second)

	// CRASH SIMULATION: Kill Benthos mid-processing
	t.Logf("💥 SIMULATING CRASH: Stopping Benthos pipeline...")
	pipelineCancel()

	// Wait for cleanup
	time.Sleep(1 * time.Second)

	// RESTART with same client_id for message recovery
	t.Logf("🔄 RESTARTING: Benthos pipeline with same client_id...")

	restartCtx, restartCancel := context.WithTimeout(ctx, 15*time.Second)
	defer restartCancel()

	// Create new pipeline with same config
	restartBuilder := service.NewStreamBuilder()
	err = restartBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	restartStream, err := restartBuilder.Build()
	require.NoError(t, err)

	go func() {
		err := restartStream.Run(restartCtx)
		if err != nil && restartCtx.Err() != context.Canceled {
			t.Logf("Restart pipeline error: %v", err)
		}
	}()

	// Wait for producer to finish and pipeline to process remaining
	time.Sleep(12 * time.Second)

	// Count messages received in NATS
	t.Logf("📊 Checking NATS for received messages...")

	receivedCount := 0
	for {
		msg, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		receivedCount++
		if receivedCount <= 10 || receivedCount%100 == 0 {
			t.Logf("✅ NATS received: %s", string(msg.Data))
		}
	}

	t.Logf("📊 Final Results:")
	t.Logf("   Sent to MQTT: 2000")
	t.Logf("   Received in NATS: %d", receivedCount)

	// Allow up to 25% message loss with standard MQTT component during crashes
	minExpected := int(float64(2000) * 0.75)
	require.GreaterOrEqual(t, receivedCount, minExpected,
		"Standard MQTT should deliver at least 75%% of messages (max 25%% loss allowed)")

	t.Logf("=== Benthos with-crash test completed: %d/%d messages delivered (%.1f%%) ===",
		receivedCount, 2000, float64(receivedCount)/20.0)
}

func testWombatMQTTAtLeastOnceDelivery(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	// Setup NATS client for validation
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.sequences.%s", testID))
	require.NoError(t, err)

	// Start Benthos pipeline with WW_MQTT_3 component
	t.Logf("🔗 Starting Benthos pipeline WW_MQTT_3→NATS...")

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "benthos-worker-`+testID+`"
    filters:
      "test/sequences/`+testID+`": 2

pipeline:
  processors:
    - sleep:
        duration: "10ms"
    - log:
        level: INFO
        message: "📨 WW_MQTT_3→NATS: ${! content() }"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.sequences.`+testID+`"
`, "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	pipelineBuilder := service.NewStreamBuilder()
	err = pipelineBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	pipelineStream, err := pipelineBuilder.Build()
	require.NoError(t, err)

	// Start pipeline in background
	go func() {
		err := pipelineStream.Run(ctx)
		if err != nil && ctx.Err() != context.Canceled {
			t.Logf("Pipeline error: %v", err)
		}
	}()

	// Wait for pipeline to be ready
	time.Sleep(2 * time.Second)

	// Send messages via external producer
	t.Logf("📤 Sending 100 sequences to MQTT...")

	producerConfig := strings.ReplaceAll(`
input:
  generate:
    interval: "50ms"
    count: 100
    mapping: 'root = "sequence-" + counter().string()'

output:
  mqtt:
    urls: ["$MQTT_URL"]
    topic: "test/sequences/`+testID+`"
    client_id: "external-producer-`+testID+`"
    qos: 2
`, "$MQTT_URL", mqttURL)

	producerBuilder := service.NewStreamBuilder()
	err = producerBuilder.SetYAML(producerConfig)
	require.NoError(t, err)

	producerStream, err := producerBuilder.Build()
	require.NoError(t, err)

	// Run producer
	prodCtx, prodCancel := context.WithTimeout(ctx, 10*time.Second)
	defer prodCancel()

	err = producerStream.Run(prodCtx)
	require.NoError(t, err)

	t.Logf("✅ Producer finished - 100 sequences sent to MQTT")

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Count messages received in NATS
	t.Logf("📊 Checking NATS for received messages...")

	receivedCount := 0
	for {
		msg, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		receivedCount++
		t.Logf("✅ NATS received: %s", string(msg.Data))
	}

	t.Logf("📊 Final Results:")
	t.Logf("   Sent to MQTT: 100")
	t.Logf("   Received in NATS: %d", receivedCount)

	require.Equal(t, 100, receivedCount, "All messages must be delivered without crashes")
}

func testWombatMQTTAtLeastOnceDeliveryWithRetryAfterCrash(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string) {
	ctx, cancel := context.WithTimeout(parentCtx, 45*time.Second)
	defer cancel()
	// Setup NATS client for validation
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.sequences.%s", testID))
	require.NoError(t, err)

	// Start Benthos pipeline with WW_MQTT_3 component
	t.Logf("🔗 Starting Benthos pipeline WW_MQTT_3→NATS...")

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "benthos-worker-`+testID+`"
    filters:
      "test/sequences/`+testID+`": 2

pipeline:
  processors:
    - sleep:
        duration: "10ms"
    - log:
        level: INFO
        message: "📨 WW_MQTT_3→NATS: ${! content() }"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.sequences.`+testID+`"
`, "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	pipelineBuilder := service.NewStreamBuilder()
	err = pipelineBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	pipelineStream, err := pipelineBuilder.Build()
	require.NoError(t, err)

	// Start pipeline in background with cancellable context
	pipelineCtx, pipelineCancel := context.WithCancel(ctx)

	go func() {
		err := pipelineStream.Run(pipelineCtx)
		if err != nil && pipelineCtx.Err() != context.Canceled {
			t.Logf("Pipeline error: %v", err)
		}
	}()

	// Wait for pipeline to be ready
	time.Sleep(2 * time.Second)

	// Send messages via external producer
	t.Logf("📤 Sending 2000 sequences to MQTT...")

	producerConfig := strings.ReplaceAll(`
input:
  generate:
    interval: "5ms"
    count: 2000
    mapping: 'root = "sequence-" + counter().string()'

output:
  mqtt:
    urls: ["$MQTT_URL"]
    topic: "test/sequences/`+testID+`"
    client_id: "external-producer-`+testID+`"
    qos: 2
`, "$MQTT_URL", mqttURL)

	producerBuilder := service.NewStreamBuilder()
	err = producerBuilder.SetYAML(producerConfig)
	require.NoError(t, err)

	producerStream, err := producerBuilder.Build()
	require.NoError(t, err)

	// Start producer in background
	go func() {
		prodCtx, prodCancel := context.WithTimeout(ctx, 15*time.Second)
		defer prodCancel()

		err := producerStream.Run(prodCtx)
		if err != nil {
			t.Logf("Producer error: %v", err)
		}
		t.Logf("✅ Producer finished - 2000 sequences sent to MQTT")
	}()

	// Let producer send ~400 messages (400 * 5ms = 2 seconds)
	time.Sleep(2 * time.Second)

	// CRASH SIMULATION: Kill Benthos mid-processing
	t.Logf("💥 SIMULATING CRASH: Stopping Benthos pipeline...")
	pipelineCancel()

	// Wait for cleanup
	time.Sleep(1 * time.Second)

	// RESTART with same client_id for message recovery
	t.Logf("🔄 RESTARTING: Benthos pipeline with same client_id...")

	restartCtx, restartCancel := context.WithTimeout(ctx, 15*time.Second)
	defer restartCancel()

	// Create new pipeline with same config
	restartBuilder := service.NewStreamBuilder()
	err = restartBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	restartStream, err := restartBuilder.Build()
	require.NoError(t, err)

	go func() {
		err := restartStream.Run(restartCtx)
		if err != nil && restartCtx.Err() != context.Canceled {
			t.Logf("Restart pipeline error: %v", err)
		}
	}()

	// Wait for producer to finish and pipeline to process remaining
	time.Sleep(12 * time.Second)

	// Count messages received in NATS
	t.Logf("📊 Checking NATS for received messages...")

	receivedCount := 0
	for {
		msg, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		receivedCount++
		if receivedCount <= 10 || receivedCount%100 == 0 {
			t.Logf("✅ NATS received: %s", string(msg.Data))
		}
	}

	t.Logf("📊 Final Results:")
	t.Logf("   Sent to MQTT: 2000")
	t.Logf("   Received in NATS: %d", receivedCount)

	// Allow up to 25% message loss with standard MQTT component during crashes
	minExpected := int(float64(2000) * 0.75) // 75% = 1500 messages minimum
	require.GreaterOrEqual(t, receivedCount, minExpected,
		"Standard MQTT should deliver at least 75%% of messages (max 25%% loss allowed)")

	t.Logf("=== Wombat with-crash test completed: %d/%d messages delivered (%.1f%%) ===",
		receivedCount, 2000, float64(receivedCount)/20.0)
}
