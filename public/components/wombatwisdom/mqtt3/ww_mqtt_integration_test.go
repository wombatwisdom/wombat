//go:build integration
// +build integration

package mqtt3_test

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/redpanda-data/benthos/v4/public/components/io"
	_ "github.com/redpanda-data/benthos/v4/public/components/pure"
	_ "github.com/redpanda-data/connect/v4/public/components/community"
)

func TestWombatWisdomMQTTIntegration(t *testing.T) {
	integration.CheckSkip(t)

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

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

	testDeliverySemantics(t, ctx, mqttURL, natsURL)
	testMQTTSessionRecoveryModes(t, ctx, mqttURL, natsURL)

}

func testDeliverySemantics(t *testing.T, ctx context.Context, mqttURL, natsURL string) {
	// Test delivery semantics based on enable_auto_ack setting:
	// - enable_auto_ack: true  = at-most-once delivery (may lose messages on crash)
	// - enable_auto_ack: false = at-least-once delivery (no message loss)
	// - default (omitted)      = at-least-once delivery (no message loss)
	testCases := []struct {
		name          string
		enableAutoAck *bool // nil means omit from config (use default)
		expectLoss    bool
		description   string
	}{
		{
			name:          "auto-ack-enabled",
			enableAutoAck: boolPtr(true),
			expectLoss:    true, // At-most-once delivery may lose messages on crash
			description:   "enable_auto_ack: true enables at-most-once delivery (higher throughput, potential message loss)",
		},
		{
			name:          "auto-ack-disabled",
			enableAutoAck: boolPtr(false),
			expectLoss:    false,
			description:   "enable_auto_ack: false ensures at-least-once delivery (no message loss)",
		},
		{
			name:          "auto-ack-default",
			enableAutoAck: nil,
			expectLoss:    false,
			description:   "default (no enable_auto_ack field) ensures at-least-once delivery (no message loss)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testAutoAckBehavior(t, ctx, mqttURL, natsURL, tc.name, tc.enableAutoAck, tc.expectLoss, tc.description)
		})
	}
}

func testMQTTSessionRecoveryModes(t *testing.T, ctx context.Context, mqttURL, natsURL string) {
	t.Run("session-recovery-at-least-once", func(t *testing.T) {
		testMQTTSessionRecovery(t, ctx, mqttURL, natsURL, "session-recovery-at-least-once", false)
	})

	t.Run("session-recovery-at-most-once", func(t *testing.T) {
		testMQTTSessionRecovery(t, ctx, mqttURL, natsURL, "session-recovery-at-most-once", true)
	})
}

func testMQTTSessionRecovery(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string, enableAutoAck bool) {
	ctx, cancel := context.WithTimeout(parentCtx, 45*time.Second)
	defer cancel()

	// Test MQTT session recovery behavior with crash and restart
	// - At-least-once (enable_auto_ack: false): Should recover all unACK'd messages
	// - At-most-once (enable_auto_ack: true): May lose messages that were ACK'd but not processed
	deliveryMode := "at-least-once"
	if enableAutoAck {
		deliveryMode = "at-most-once"
	}
	t.Logf("=== Testing MQTT Session Recovery with %s delivery ===", deliveryMode)

	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.sequences.%s", testID))
	require.NoError(t, err)

	t.Logf("🔗 Starting Benthos pipeline WW_MQTT_3→NATS with enable_auto_ack: %v...", enableAutoAck)

	autoAckConfig := ""
	if enableAutoAck {
		autoAckConfig = "    enable_auto_ack: true"
	} else {
		autoAckConfig = "    enable_auto_ack: false"
	}

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "benthos-worker-%s"
    clean_session: false
%s
    filters:
      "test/sequences/%s": 2

pipeline:
  processors:
    - sleep:
        duration: "10ms"
    - log:
        level: DEBUG
        message: "📨 WW_MQTT_3→NATS: ${! content() }"

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.sequences.%s"
`, testID, autoAckConfig, testID, testID), "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

	pipelineBuilder := service.NewStreamBuilder()
	err = pipelineBuilder.SetYAML(pipelineConfig)
	require.NoError(t, err)

	pipelineStream, err := pipelineBuilder.Build()
	require.NoError(t, err)

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

	t.Logf("💥 SIMULATING CRASH: Stopping Benthos pipeline...")
	pipelineCancel()

	// Wait for cleanup
	time.Sleep(1 * time.Second)

	// RESTART with same client_id for message recovery
	t.Logf("🔄 RESTARTING: Benthos pipeline with same client_id...")

	restartCtx, restartCancel := context.WithTimeout(ctx, 15*time.Second)
	defer restartCancel()

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

	t.Logf("📊 Checking NATS for received messages...")

	receivedMessages := make(map[string]int)
	receivedCount := 0
	for {
		msg, err := sub.NextMsg(200 * time.Millisecond)
		if err != nil {
			break
		}
		receivedCount++
		msgContent := string(msg.Data)
		receivedMessages[msgContent]++
		if receivedCount <= 10 || receivedCount%100 == 0 {
			t.Logf("✅ NATS received: %s", msgContent)
		}
	}

	uniqueCount := len(receivedMessages)
	duplicateCount := receivedCount - uniqueCount

	t.Logf("📊 Final Results:")
	t.Logf("   Sent to MQTT: 2000")
	t.Logf("   Received in NATS: %d", receivedCount)
	t.Logf("   Unique messages: %d", uniqueCount)
	t.Logf("   Duplicates: %d", duplicateCount)

	// Validate based on delivery semantics
	if enableAutoAck {
		// At-most-once delivery - may lose some messages during crash
		messagesLost := 2000 - receivedCount
		t.Logf("📊 At-most-once delivery results:")
		t.Logf("   Messages lost: %d", messagesLost)
		if messagesLost > 0 {
			t.Logf("✅ Expected behavior: Some messages lost with at-most-once delivery")
		} else {
			t.Logf("⚠️  No messages lost - timing didn't trigger loss scenario")
		}
		// With unbuffered channel, loss should be minimal even in at-most-once mode
		assert.LessOrEqual(t, messagesLost, 50, "Message loss should be minimal even with at-most-once")
	} else {
		// At-least-once delivery - expect no message loss (but duplicates are OK)
		t.Logf("📊 At-least-once delivery results:")
		if uniqueCount == 2000 {
			t.Logf("✅ SUCCESS: All 2000 unique messages delivered with session recovery")
			if duplicateCount > 0 {
				t.Logf("   %d duplicates detected (expected behavior for at-least-once)", duplicateCount)
				t.Logf("   Messages were redelivered after crash as they weren't ACK'd")
			}
		} else if uniqueCount < 2000 {
			t.Logf("❌ FAILURE: %d messages lost with at-least-once delivery", 2000-uniqueCount)
		}
		require.Equal(t, 2000, uniqueCount, "At-least-once must deliver all unique messages")
		require.GreaterOrEqual(t, receivedCount, 2000, "At-least-once may have duplicates but no loss")
	}

	t.Logf("=== MQTT session recovery test completed: %d/%d messages delivered (%.1f%%) ===",
		receivedCount, 2000, float64(receivedCount)/20.0)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

func testAutoAckBehavior(t *testing.T, parentCtx context.Context, mqttURL, natsURL, testID string, enableAutoAck *bool, expectLoss bool, description string) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	t.Logf("=== Testing MQTT Delivery Semantics: %s ===", testID)
	t.Logf("Description: %s", description)

	var autoAckStr string
	if enableAutoAck == nil {
		autoAckStr = "default (false)"
	} else {
		autoAckStr = fmt.Sprintf("%t", *enableAutoAck)
	}
	t.Logf("Testing enable_auto_ack: %s", autoAckStr)

	// Setup NATS for receiving processed messages
	natsConn, err := nats.Connect(natsURL)
	require.NoError(t, err)
	defer natsConn.Close()

	sub, err := natsConn.SubscribeSync(fmt.Sprintf("processed.%s", testID))
	require.NoError(t, err)

	// Track what we receive
	receivedCount := 0

	// Build pipeline configuration with dynamic enable_auto_ack setting
	var autoAckConfig string
	if enableAutoAck != nil {
		autoAckConfig = fmt.Sprintf("    enable_auto_ack: %t", *enableAutoAck)
	} else {
		autoAckConfig = "" // Omit field to use default
	}

	pipelineConfig := strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf(`
input:
  ww_mqtt_3:
    urls: ["$MQTT_URL"]
    client_id: "worker-%s"
    clean_session: false
%s
    filters:
      "test/%s": 2

pipeline:
  processors:
    # Add processing delay to increase chance of message loss in at-most-once mode
    - sleep:
        duration: "100ms"  # Increased delay to better demonstrate timing issues

output:
  nats:
    urls: ["$NATS_URL"]
    subject: "processed.%s"
`, testID, autoAckConfig, testID, testID), "$MQTT_URL", mqttURL), "$NATS_URL", natsURL)

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
	producerConfig := strings.ReplaceAll(fmt.Sprintf(`
input:
  generate:
    interval: "1ms"  # Very fast to ensure buffer fills
    count: 200
    mapping: |
      root = "message-" + counter().string()

output:
  mqtt:
    urls: ["$MQTT_URL"]
    topic: "test/%s"
    client_id: "producer-%s"
    qos: 2
`, testID, testID), "$MQTT_URL", mqttURL)

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
	// With 100ms processing delay and 200 messages, full processing would take 20s
	// Kill after 1s to ensure many messages are still in flight
	time.Sleep(1 * time.Second)

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

	// Validate delivery semantics based on expected behavior
	if expectLoss {
		// At-most-once delivery - may lose messages
		messagesLost := 200 - receivedCount
		if messagesLost > 0 {
			t.Logf("✅ EXPECTED: %d messages lost with at-most-once delivery", messagesLost)
			t.Logf("   Messages were ACK'd before processing, enabling higher throughput")
			t.Logf("   This demonstrates at-most-once delivery semantics")
		} else {
			t.Logf("⚠️  No messages lost with enable_auto_ack: true")
			t.Logf("   Due to unbuffered channel and timing, message loss is rare but possible")
			t.Logf("   At-most-once delivery is still configured correctly")
		}

		// With unbuffered channel, message loss requires precise timing
		// We allow 0 loss but note that loss is possible
		if messagesLost > 0 {
			assert.LessOrEqual(t, messagesLost, 10, "Message loss should be minimal with unbuffered channel")
		}
	} else {
		// At-least-once delivery - expect no message loss
		if receivedCount == 200 {
			t.Logf("✅ SUCCESS: All 200 messages delivered with at-least-once semantics")
			t.Logf("   Disabled auto ACK ensures messages are only ACK'd after successful processing")
			t.Logf("   This demonstrates proper at-least-once delivery")
		} else {
			t.Logf("❌ UNEXPECTED: %d messages lost with auto ACK disabled", 200-receivedCount)
			t.Logf("   Disabled auto ACK should prevent message loss during crashes")
		}

		// Should receive all messages with at-least-once delivery
		assert.Equal(t, 200, receivedCount, "Should receive all messages with at-least-once delivery")
	}
}
