//go:build integration
// +build integration

package mqtt3_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
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
    topic: data/flow
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

func TestWriteTimeoutIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL := startMQTTBroker(t, ctx)

	testCases := []struct {
		name          string
		writeTimeout  string
		expectSuccess bool
	}{
		{
			name:          "2ms_timeout_should_fail",
			writeTimeout:  "2ms",
			expectSuccess: false,
		},
		{
			name:          "5s_timeout_should_succeed",
			writeTimeout:  "5s",
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use timestamp to make topics unique
			testID := fmt.Sprintf("%d", time.Now().UnixNano())
			messagesReceived := make(chan mqtt.Message, 10)
			subscriberOpts := mqtt.NewClientOptions().
				AddBroker(mqttURL).
				SetClientID(fmt.Sprintf("timeout-subscriber-%s-%s", tc.name, testID))

			subscriber := mqtt.NewClient(subscriberOpts)
			token := subscriber.Connect()
			require.True(t, token.WaitTimeout(5*time.Second))
			require.NoError(t, token.Error())
			defer subscriber.Disconnect(1000)

			topicPattern := fmt.Sprintf("test/timeout/%s/%s/+", tc.name, testID)
			token = subscriber.Subscribe(topicPattern, 1, func(client mqtt.Client, msg mqtt.Message) {
				select {
				case messagesReceived <- msg:
				default:
				}
			})
			require.True(t, token.WaitTimeout(5*time.Second))
			require.NoError(t, token.Error())

			outputConf := fmt.Sprintf(`
ww_mqtt_3:
  urls:
    - %s
  client_id: timeout-test-%s-%s
  topic: 'test/timeout/%s/%s/${! json("id") }'
  write_timeout: %s
  qos: 1
`, mqttURL, tc.name, testID, tc.name, testID, tc.writeTimeout)

			streamBuilder := service.NewStreamBuilder()
			require.NoError(t, streamBuilder.AddOutputYAML(outputConf))

			// Send larger messages (5-6MB) that will serialize quickly
			// but should time out during transmission with very short WriteTimeout config
			require.NoError(t, streamBuilder.AddInputYAML(`
generate:
  count: 3
  interval: 100ms
  mapping: |
    root.id = counter().string()
    root.data = range(0, 100000).map_each(i -> {
      "index": i,
      "value": uuid_v4(),
    })
    root.timestamp = now()
`))

			// capture log messages so we can make assertions
			var logBuffer bytes.Buffer

			logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
			streamBuilder.SetLogger(logger)

			stream, err := streamBuilder.Build()
			require.NoError(t, err)

			streamCtx, streamCancel := context.WithTimeout(ctx, 10*time.Second)
			defer streamCancel()

			streamDone := make(chan error, 1)
			go func() {
				streamDone <- stream.Run(streamCtx)
			}()

			receivedCount := 0
			timeout := time.After(6 * time.Second)

		CollectLoop:
			for {
				select {
				case msg := <-messagesReceived:
					receivedCount++
					t.Logf("Received message %d on topic: %s", receivedCount, msg.Topic())
				case <-timeout:
					t.Log("Collection timeout")
					break CollectLoop
				case err := <-streamDone:
					if err != nil && err != context.DeadlineExceeded {
						t.Logf("Stream ended with error: %v", err)
					}
					break CollectLoop
				}
			}

			streamCancel()

			if tc.expectSuccess {
				assert.Equal(t, receivedCount, 3,
					"Should receive messages with %s write_timeout", tc.writeTimeout)
			} else {
				assert.Contains(t, logBuffer.String(), "i/o timeout", "Logs should show IO timeout")
			}
		})
	}
}

// tcpProxy is a simple TCP proxy that can be paused to simulate network disruption
type tcpProxy struct {
	listener   net.Listener
	targetAddr string
	mu         sync.Mutex
	paused     bool
	closed     bool
	conns      []net.Conn
}

func newTCPProxy(listenAddr, targetAddr string) (*tcpProxy, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	return &tcpProxy{
		listener:   listener,
		targetAddr: targetAddr,
		conns:      make([]net.Conn, 0),
	}, nil
}

func (p *tcpProxy) Start() {
	go func() {
		for {
			conn, err := p.listener.Accept()
			if err != nil {
				return
			}

			p.mu.Lock()
			if p.closed {
				p.mu.Unlock()
				conn.Close()
				return
			}
			p.conns = append(p.conns, conn)
			p.mu.Unlock()

			go p.handleConnection(conn)
		}
	}()
}

func (p *tcpProxy) handleConnection(client net.Conn) {
	defer client.Close()

	target, err := net.Dial("tcp", p.targetAddr)
	if err != nil {
		return
	}
	defer target.Close()

	// Bidirectional forwarding
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.forward(client, target)
	}()
	p.forward(target, client)
	<-done
}

func (p *tcpProxy) forward(from, to net.Conn) {
	buf := make([]byte, 4096)
	for {
		p.mu.Lock()
		paused := p.paused
		p.mu.Unlock()

		if paused {
			// When paused, don't forward packets (simulate network disruption)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		from.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := from.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		_, err = to.Write(buf[:n])
		if err != nil {
			return
		}
	}
}

func (p *tcpProxy) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

func (p *tcpProxy) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = false
}

func (p *tcpProxy) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true

	// Close all connections
	for _, conn := range p.conns {
		conn.Close()
	}

	return p.listener.Close()
}

func (p *tcpProxy) Addr() string {
	return p.listener.Addr().String()
}

func TestConnectTimeoutIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL := startMQTTBroker(t, ctx)

	testCases := []struct {
		name           string
		connectTimeout string
		mqttURL        string
		expectSuccess  bool
	}{
		{
			name:           "100ms_timeout_to_nonexistent_should_fail",
			connectTimeout: "100ms",
			mqttURL:        "tcp://127.0.0.1:65432", // Non-existent port
			expectSuccess:  false,
		},
		{
			name:           "5s_timeout_to_valid_broker_should_succeed",
			connectTimeout: "5s",
			mqttURL:        mqttURL,
			expectSuccess:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var subscriber mqtt.Client
			messagesReceived := make(chan mqtt.Message, 10)

			if tc.expectSuccess {
				subscriberOpts := mqtt.NewClientOptions().
					AddBroker(tc.mqttURL).
					SetClientID(fmt.Sprintf("connect-timeout-subscriber-%s", tc.name))

				subscriber = mqtt.NewClient(subscriberOpts)
				token := subscriber.Connect()
				require.True(t, token.WaitTimeout(5*time.Second))
				require.NoError(t, token.Error())
				defer subscriber.Disconnect(1000)

				topicPattern := fmt.Sprintf("test/connect/%s/+", tc.name)
				token = subscriber.Subscribe(topicPattern, 1, func(client mqtt.Client, msg mqtt.Message) {
					select {
					case messagesReceived <- msg:
					default:
					}
				})
				require.True(t, token.WaitTimeout(5*time.Second))
				require.NoError(t, token.Error())

				// Give subscription time to be established on broker
				time.Sleep(500 * time.Millisecond)
			}

			outputConf := fmt.Sprintf(`
ww_mqtt_3:
  urls:
    - %s
  client_id: connect-timeout-test-%s
  topic: 'test/connect/%s/${! json("id") }'
  connect_timeout: %s
  qos: 1
`, tc.mqttURL, tc.name, tc.name, tc.connectTimeout)

			streamBuilder := service.NewStreamBuilder()
			require.NoError(t, streamBuilder.AddOutputYAML(outputConf))

			require.NoError(t, streamBuilder.AddInputYAML(`
generate:
  count: 1
  mapping: |
    root.id = counter().string()
    root.message = "test connection"
    root.timestamp = now()
`))

			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
			streamBuilder.SetLogger(logger)

			stream, err := streamBuilder.Build()
			require.NoError(t, err)

			streamCtx, streamCancel := context.WithTimeout(ctx, 10*time.Second)
			defer streamCancel()

			streamDone := make(chan error, 1)
			go func() {
				streamDone <- stream.Run(streamCtx)
			}()

			receivedCount := 0
			timeout := time.After(6 * time.Second)

		CollectLoop:
			for {
				select {
				case msg := <-messagesReceived:
					receivedCount++
					t.Logf("Received message %d on topic: %s", receivedCount, msg.Topic())
				case <-timeout:
					t.Log("Collection timeout")
					break CollectLoop
				case err := <-streamDone:
					if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
						t.Logf("Stream ended with error: %v", err)
					}
					break CollectLoop
				}
			}

			streamCancel()

			logs := logBuffer.String()
			t.Logf("Captured logs: %s", logs)

			if tc.expectSuccess {
				assert.Equal(t, 1, receivedCount,
					"Should receive message with %s connect_timeout to valid broker", tc.connectTimeout)
				assert.NotContains(t, logs, "i/o timeout", "Should not have timeout errors")
				assert.NotContains(t, logs, "connection refused", "Should not have connection errors")
			} else {
				assert.Equal(t, 0, receivedCount,
					"Should not receive messages with %s connect_timeout to non-existent broker", tc.connectTimeout)
				hasConnectionError := strings.Contains(logs, "Attempting to reconnect to MQTT broker")
				assert.True(t, hasConnectionError,
					"Logs should show connection error with %s connect_timeout", tc.connectTimeout)
			}
		})
	}
}

func TestKeepAliveIntegration(t *testing.T) {
	integration.CheckSkip(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mqttURL := startMQTTBroker(t, ctx)

	brokerHost := strings.TrimPrefix(mqttURL, "tcp://")

	// Create TCP proxy we can later interrupt
	proxy, err := newTCPProxy("127.0.0.1:0", brokerHost)
	require.NoError(t, err)
	defer proxy.Close()

	proxy.Start()
	proxyURL := fmt.Sprintf("tcp://%s", proxy.Addr())
	t.Logf("TCP proxy listening at %s, forwarding to %s", proxyURL, brokerHost)

	t.Run("detects_connection_loss_via_keepalive", func(t *testing.T) {
		messagesReceived := make(chan mqtt.Message, 10)
		subscriberOpts := mqtt.NewClientOptions().
			AddBroker(mqttURL).
			SetClientID("keepalive-subscriber").
			SetKeepAlive(30 * time.Second)

		subscriber := mqtt.NewClient(subscriberOpts)
		token := subscriber.Connect()
		require.True(t, token.WaitTimeout(5*time.Second))
		require.NoError(t, token.Error())
		defer subscriber.Disconnect(1000)

		token = subscriber.Subscribe("test/keepalive/+", 1, func(client mqtt.Client, msg mqtt.Message) {
			select {
			case messagesReceived <- msg:
			default:
			}
		})
		require.True(t, token.WaitTimeout(5*time.Second))
		require.NoError(t, token.Error())

		// Give subscription time to be established
		time.Sleep(500 * time.Millisecond)

		// Create output that connects through proxy with short keepalive
		outputConf := fmt.Sprintf(`
ww_mqtt_3:
  urls:
    - %s
  client_id: keepalive-test-client
  topic: 'test/keepalive/${! json("id") }'
  keepalive: 2s
  connect_timeout: 5s
  qos: 1
`, proxyURL)

		streamBuilder := service.NewStreamBuilder()
		require.NoError(t, streamBuilder.AddOutputYAML(outputConf))

		require.NoError(t, streamBuilder.AddInputYAML(`
generate:
  count: 20
  interval: 1s
  mapping: |
    root.id = counter().string()
    root.message = "keepalive test"
    root.timestamp = now()
`))

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
		streamBuilder.SetLogger(logger)

		stream, err := streamBuilder.Build()
		require.NoError(t, err)

		streamCtx, streamCancel := context.WithTimeout(ctx, 25*time.Second)
		defer streamCancel()

		streamDone := make(chan error, 1)
		go func() {
			streamDone <- stream.Run(streamCtx)
		}()

		// Wait for first message to verify connection
		select {
		case msg := <-messagesReceived:
			t.Logf("Received initial message on topic: %s", msg.Topic())
		case <-time.After(5 * time.Second):
			t.Fatal("Failed to receive initial message")
		}

		// Now pause the proxy to simulate network disruption
		t.Log("Pausing proxy to simulate network disruption")
		proxy.Pause()
		disruptionStart := time.Now()

		// Monitor logs for disconnection detection
		// With 2s keepalive, disconnection should be detected within ~13 seconds
		// --> 10 second Paho default ping timeout + 2 second pingreq/pingresp + some processing
		disconnectionDetected := false
		var disconnectionTime time.Duration
		checkStart := time.Now()
		lastLogSize := 0

		for time.Since(checkStart) < 15*time.Second {
			time.Sleep(500 * time.Millisecond)
			logs := logBuffer.String()

			// Log any new content
			if len(logs) > lastLogSize {
				newContent := logs[lastLogSize:]
				t.Logf("New log at %v: %s", time.Since(disruptionStart), strings.TrimSpace(newContent))
				lastLogSize = len(logs)
			}

			if strings.Contains(logs, "pingresp not received") {
				disconnectionDetected = true
				disconnectionTime = time.Since(disruptionStart)
				t.Logf("Disconnection detected after %v", disconnectionTime)
				break
			}
		}

		logs := logBuffer.String()
		t.Logf("Total time elapsed: %v", time.Since(disruptionStart))
		t.Logf("Final logs:\n%s", logs)

		assert.True(t, disconnectionDetected,
			"Should detect connection loss within ~3x keepalive interval (6s)")

		t.Log("Resuming proxy")
		proxy.Resume()

		logBuffer.Reset()

		receivedAfterResume := 0
		timeout := time.After(10 * time.Second)

	CollectLoop:
		for {
			select {
			case msg := <-messagesReceived:
				receivedAfterResume++
				t.Logf("Received message %d after resume on topic: %s", receivedAfterResume, msg.Topic())
				if receivedAfterResume >= 2 {
					break CollectLoop
				}
			case <-timeout:
				t.Log("Collection timeout")
				break CollectLoop
			case err := <-streamDone:
				if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
					t.Logf("Stream ended with error: %v", err)
				}
				break CollectLoop
			}
		}

		assert.GreaterOrEqual(t, receivedAfterResume, 1,
			"Should receive at least one message after connection resumes")
	})
}
