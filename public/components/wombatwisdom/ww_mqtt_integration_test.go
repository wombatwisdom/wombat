//go:build integration
// +build integration

package wombatwisdom_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"github.com/stretchr/testify/assert"
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
	testWombatMQTTBufferLossDetection(t, ctx, mqttURL, natsURL, "wombat-buffer-loss")
	testSimpleBufferLossDemo(t, ctx, mqttURL, natsURL, "simple-buffer-loss")
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

func testWombatMQTTBufferLossDetection(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string) {
	ctx, cancel := context.WithTimeout(parentCtx, 60*time.Second)
	defer cancel()

	t.Logf("=== Starting Wombat MQTT buffer loss detection test ===")

	// Setup NATS client for validation
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.sequences.%s", testID))
	require.NoError(t, err)

	// Track received messages with sequence numbers and UUIDs
	receivedSequences := make(map[int]string) // seq -> uuid
	allMessages := []struct {
		UUID string
		Seq  int
	}{}

	// Start Benthos pipeline with WW_MQTT_3 component
	t.Logf("🔗 Starting Benthos pipeline WW_MQTT_3→NATS with crash trigger...")

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "benthos-worker-`+testID+`"
    filters:
      "test/sequences/`+testID+`": 2

pipeline:
  processors:
    - mapping: |
        root = content()
        let parsed = content().parse_json()
        meta crash_on_seq = if parsed.seq == 500 { "true" } else { "false" }
        meta seq_num = parsed.seq
    - log:
        level: INFO
        message: 'Processing seq: ${! meta("seq_num") }'
    - switch:
        - check: 'meta("crash_on_seq") == "true"'
          processors:
            - log:
                level: ERROR
                message: "CRASHING ON SEQ 500!"
            - mapping: 'throw("Simulated crash for testing")'
    - sleep:
        duration: "5ms"

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
	pipelineCtx, _ := context.WithCancel(ctx)

	go func() {
		err := pipelineStream.Run(pipelineCtx)
		if err != nil && pipelineCtx.Err() != context.Canceled {
			t.Logf("Pipeline error (expected): %v", err)
		}
	}()

	// Wait for pipeline to be ready
	time.Sleep(2 * time.Second)

	// Send messages with UUIDs and sequence numbers
	t.Logf("📤 Sending 1000 sequenced messages with UUIDs to MQTT...")

	producerConfig := strings.ReplaceAll(`
input:
  generate:
    interval: "2ms"
    count: 1000
    mapping: |
      root.uuid = uuid_v4()
      root.seq = counter()
      root.data = "test-message-" + root.seq.string()

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
	go func() {
		prodCtx, prodCancel := context.WithTimeout(ctx, 20*time.Second)
		defer prodCancel()

		err := producerStream.Run(prodCtx)
		if err != nil {
			t.Logf("Producer error: %v", err)
		}
		t.Logf("✅ Producer finished - 1000 messages sent to MQTT")
	}()

	// Collect messages before crash
	t.Logf("📊 Collecting messages before crash trigger...")
	crashed := false
	precrashTimeout := time.After(10 * time.Second)

	for !crashed {
		select {
		case <-precrashTimeout:
			crashed = true
			t.Logf("💥 Pipeline should have crashed by now")
		default:
			msg, err := sub.NextMsg(100 * time.Millisecond)
			if err == nil {
				var data map[string]interface{}
				if err := json.Unmarshal(msg.Data, &data); err == nil {
					seq := int(data["seq"].(float64))
					uuid := data["uuid"].(string)
					receivedSequences[seq] = uuid
					allMessages = append(allMessages, struct {
						UUID string
						Seq  int
					}{UUID: uuid, Seq: seq})

					if seq >= 495 && seq <= 505 {
						t.Logf("⚠️  Received seq %d (near crash point)", seq)
					}
					if seq >= 500 {
						crashed = true
						t.Logf("💥 Seq %d received - pipeline should have crashed", seq)
					}
				}
			}
		}
	}

	// RESTART with same client_id for message recovery
	t.Logf("🔄 RESTARTING: Benthos pipeline with same client_id...")

	restartCtx, restartCancel := context.WithTimeout(ctx, 20*time.Second)
	defer restartCancel()

	// Create new pipeline with same config but no crash processor
	restartConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "benthos-worker-`+testID+`"
    filters:
      "test/sequences/`+testID+`": 2

pipeline:
  processors:
    - sleep:
        duration: "5ms"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.sequences.`+testID+`"
`, "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	restartBuilder := service.NewStreamBuilder()
	err = restartBuilder.SetYAML(restartConfig)
	require.NoError(t, err)

	restartStream, err := restartBuilder.Build()
	require.NoError(t, err)

	go func() {
		err := restartStream.Run(restartCtx)
		if err != nil && restartCtx.Err() != context.Canceled {
			t.Logf("Restart pipeline error: %v", err)
		}
	}()

	// Continue collecting after restart
	time.Sleep(15 * time.Second)

	// Collect remaining messages
	t.Logf("📊 Collecting messages after restart...")
	for {
		msg, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err == nil {
			seq := int(data["seq"].(float64))
			uuid := data["uuid"].(string)

			// Check for duplicates
			if existingUUID, exists := receivedSequences[seq]; exists && existingUUID != uuid {
				t.Logf("⚠️  Sequence %d received with different UUID! Original: %s, New: %s", seq, existingUUID, uuid)
			}

			receivedSequences[seq] = uuid
			allMessages = append(allMessages, struct {
				UUID string
				Seq  int
			}{UUID: uuid, Seq: seq})
		}
	}

	// Analysis
	t.Logf("📊 Analyzing results...")

	// Check for missing sequences (gaps)
	missingSeqs := []int{}
	for i := 1; i <= 1000; i++ {
		if _, exists := receivedSequences[i]; !exists {
			missingSeqs = append(missingSeqs, i)
		}
	}

	// Check for duplicate UUIDs
	uuidCounts := make(map[string]int)
	duplicateUUIDs := []string{}
	for _, msg := range allMessages {
		uuidCounts[msg.UUID]++
		if uuidCounts[msg.UUID] == 2 {
			duplicateUUIDs = append(duplicateUUIDs, msg.UUID)
		}
	}

	t.Logf("📊 Final Results:")
	t.Logf("   Total messages sent: 1000")
	t.Logf("   Total messages received: %d", len(allMessages))
	t.Logf("   Unique sequences received: %d", len(receivedSequences))
	t.Logf("   Missing sequences (gaps): %d", len(missingSeqs))
	t.Logf("   Duplicate UUIDs: %d", len(duplicateUUIDs))

	if len(missingSeqs) > 0 {
		t.Logf("   First 10 missing sequences: %v", missingSeqs[:min(10, len(missingSeqs))])

		// Check if missing sequences are in the buffer range (around crash point)
		bufferRangeStart := 450
		bufferRangeEnd := 550
		missingInBufferRange := 0
		for _, seq := range missingSeqs {
			if seq >= bufferRangeStart && seq <= bufferRangeEnd {
				missingInBufferRange++
			}
		}
		t.Logf("   Missing sequences in buffer range (%d-%d): %d", bufferRangeStart, bufferRangeEnd, missingInBufferRange)
	}

	// The test reveals buffer loss if:
	// 1. We have gaps in sequences (messages lost from buffer)
	// 2. No duplicate UUIDs (messages were ACK'd, not re-delivered)
	if len(missingSeqs) > 0 && len(duplicateUUIDs) == 0 {
		t.Logf("❌ BUFFER LOSS DETECTED: %d messages were in the buffer and lost during crash", len(missingSeqs))
		t.Logf("   These messages were ACK'd to MQTT but never processed")
		t.Logf("   This demonstrates the at-most-once delivery semantic")
	}

	// Allow some message loss but it should be limited to buffer size
	assert.LessOrEqual(t, len(missingSeqs), 150, "Message loss should be limited to approximate buffer size")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func testSimpleBufferLossDemo(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	t.Logf("=== Starting Simple Buffer Loss Demo ===")
	t.Logf("This test demonstrates that ww_mqtt_3 uses at-most-once delivery")
	t.Logf("Messages are ACK'd to MQTT after buffering, but before processing")

	// Setup NATS for receiving processed messages
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.%s", testID))
	require.NoError(t, err)

	// Track what we receive
	receivedCount := 0

	// Pipeline that processes slowly and crashes mid-way
	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "worker-`+testID+`"
    filters:
      "test/`+testID+`": 2

pipeline:
  processors:
    # Add significant processing delay to fill up the buffer
    - sleep:
        duration: "50ms"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.`+testID+`"
`, "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	builder := service.NewStreamBuilder()
	err = builder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	stream, err := builder.Build()
	require.NoError(t, err)

	// Run pipeline
	pipelineCtx, pipelineCancel := context.WithCancel(ctx)
	pipelineDone := make(chan error, 1)

	go func() {
		err := stream.Run(pipelineCtx)
		pipelineDone <- err
	}()

	// Give pipeline time to start
	time.Sleep(1 * time.Second)

	// Send messages rapidly to fill the buffer
	t.Logf("📤 Sending 200 messages rapidly to MQTT...")
	producerConfig := strings.ReplaceAll(`
input:
  generate:
    interval: "1ms"  # Very fast to ensure buffer fills
    count: 200
    mapping: |
      root = "message-" + counter().string()

output:
  mqtt:
    urls: ["$MQTT_URL"]
    topic: "test/`+testID+`"
    client_id: "producer-`+testID+`"
    qos: 2
`, "$MQTT_URL", mqttURL)

	prodBuilder := service.NewStreamBuilder()
	err = prodBuilder.SetYAML(producerConfig)
	require.NoError(t, err)

	prodStream, err := prodBuilder.Build()
	require.NoError(t, err)

	// Send all messages
	err = prodStream.Run(ctx)
	require.NoError(t, err)
	t.Logf("✅ Sent 200 messages to MQTT")

	// Let some messages get processed, then kill the pipeline
	time.Sleep(2 * time.Second)

	t.Logf("💥 Killing pipeline to simulate crash...")
	pipelineCancel()

	// Wait for pipeline to stop
	select {
	case err := <-pipelineDone:
		t.Logf("Pipeline stopped: %v", err)
	case <-time.After(1 * time.Second):
		t.Logf("Pipeline stop timeout")
	}

	// Count received messages
	t.Logf("📊 Counting messages that made it to NATS...")
	for {
		_, err := sub.NextMsg(100 * time.Millisecond)
		if err != nil {
			break
		}
		receivedCount++
	}

	t.Logf("📊 Results:")
	t.Logf("   Messages sent to MQTT: 200")
	t.Logf("   Messages received in NATS: %d", receivedCount)
	t.Logf("   Messages lost: %d", 200-receivedCount)

	// The key insight:
	if receivedCount < 200 {
		messagesLost := 200 - receivedCount
		t.Logf("❌ BUFFER LOSS: %d messages were lost", messagesLost)
		t.Logf("   These messages were likely:")
		t.Logf("   - In the 100-message buffer when pipeline was killed")
		t.Logf("   - Already ACK'd to MQTT (so not re-delivered)")
		t.Logf("   This demonstrates at-most-once delivery")
	}

	// We should lose some messages due to buffer overflow or pipeline termination
	assert.Less(t, receivedCount, 200, "Should not receive all messages")
	assert.Greater(t, 200-receivedCount, 0, "Should have lost some messages")
}
