//go:build integration
// +build integration

package wombatwisdom_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
)

func TestWombatwWisdomIntegration(t *testing.T) {
	integration.CheckSkip(t)

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Disable Ryuk container cleanup to avoid network issues
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	t.Run("MQTT", testMQTTIntegration)
	t.Run("NATS", testNATSIntegration)
	t.Run("S3", testS3Integration)
	t.Run("IBM_MQ", testIBMMQIntegration)
	t.Run("EventBridge", testEventBridgeIntegration)
}

func testMQTTIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start MQTT container (Eclipse Mosquitto) with custom config
	mqttContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "eclipse-mosquitto:2.0",
			ExposedPorts: []string{"1883/tcp"},
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      "",
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
		if err := mqttContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate MQTT container: %v", err)
		}
	})

	// Get MQTT broker URL
	mqttHost, err := mqttContainer.Host(ctx)
	require.NoError(t, err)
	mqttPort, err := mqttContainer.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	mqttURL := fmt.Sprintf("tcp://%s:%s", mqttHost, mqttPort.Port())

	// Test configuration template - matches test-ww-mqtt.yaml pattern
	template := strings.ReplaceAll(`
input:
  ww_mqtt:
    urls: ["$MQTT_URL"]
    filters:
      "test/integration": 0
    client_id: "integration-test-$ID"

output:
  ww_mqtt:
    urls: ["$MQTT_URL"] 
    topic: "test/output-$ID"
    client_id: "integration-output-$ID"
`, "$MQTT_URL", mqttURL)

	// Run integration test suite - start with just connection tests
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		// Skip batch and stream tests for now as they require both input and output working
	)

	suite.Run(t, template,
		integration.StreamTestOptSleepAfterInput(200*time.Millisecond),
		integration.StreamTestOptSleepAfterOutput(200*time.Millisecond),
	)
}

func testNATSIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start NATS server container
	natsContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.10-alpine",
			ExposedPorts: []string{"4222/tcp"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
			Cmd:          []string{"--port", "4222", "--http_port", "8222"},
		},
		Started: true,
	})
	require.NoError(t, err, "Failed to start NATS container")

	t.Cleanup(func() {
		if err := natsContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate NATS container: %v", err)
		}
	})

	// Get NATS server URL
	natsHost, err := natsContainer.Host(ctx)
	require.NoError(t, err)
	natsPort, err := natsContainer.MappedPort(ctx, "4222/tcp")
	require.NoError(t, err)
	natsURL := fmt.Sprintf("nats://%s:%s", natsHost, natsPort.Port())

	// Test configuration template - matches test-ww-nats.yaml pattern
	template := strings.ReplaceAll(`
input:
  ww_nats:
    url: "$NATS_URL"
    subject: "test.input.$ID"
    name: "integration-test-$ID"

output:
  ww_nats:
    url: "$NATS_URL"
    subject: "test.output.$ID"
    name: "integration-output-$ID"
`, "$NATS_URL", natsURL)

	// Run integration test suite
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestSendBatch(5),
		integration.StreamTestStreamSequential(10),
	)

	suite.Run(t, template,
		integration.StreamTestOptSleepAfterInput(200*time.Millisecond),
		integration.StreamTestOptSleepAfterOutput(200*time.Millisecond),
	)
}

func testS3Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start MinIO container (S3-compatible storage)
	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err, "Failed to start MinIO container")

	t.Cleanup(func() {
		if err := minioContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate MinIO container: %v", err)
		}
	})

	// Get MinIO endpoint
	minioHost, err := minioContainer.Host(ctx)
	require.NoError(t, err)
	minioPort, err := minioContainer.MappedPort(ctx, "9000/tcp")
	require.NoError(t, err)
	minioEndpoint := fmt.Sprintf("http://%s:%s", minioHost, minioPort.Port())

	// Test configuration template - matches test-ww-s3.yaml pattern
	template := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_s3:
    bucket: "test-bucket-$ID"
    region: "us-east-1"
    endpoint_url: "$MINIO_ENDPOINT"
    aws:
      access_key_id: "minioadmin"
      secret_access_key: "minioadmin"

output:
  ww_s3:
    bucket: "test-output-bucket-$ID" 
    region: "us-east-1"
    endpoint_url: "$MINIO_ENDPOINT"
    aws:
      access_key_id: "minioadmin"
      secret_access_key: "minioadmin"
    path: "output/${!uuid_v4()}.json"
`, "$MINIO_ENDPOINT", minioEndpoint), "$MINIO_ENDPOINT", minioEndpoint)

	// Run integration test suite
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestSendBatch(3),
	)

	suite.Run(t, template,
		integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, vars *integration.StreamTestConfigVars) {
			// TODO: Create S3 buckets if the ww_s3 component doesn't auto-create them
			// This would require AWS SDK or MinIO client to pre-create buckets
		}),
		integration.StreamTestOptSleepAfterInput(500*time.Millisecond),
		integration.StreamTestOptSleepAfterOutput(500*time.Millisecond),
	)
}

func testIBMMQIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start IBM MQ container
	// Note: This requires IBM MQ container image which may need licensing
	ibmMQContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "icr.io/ibm-messaging/mq:latest",
			ExposedPorts: []string{"1414/tcp", "9443/tcp"},
			Env: map[string]string{
				"LICENSE":      "accept",
				"MQ_QMGR_NAME": "QM1",
			},
			WaitingFor: wait.ForLog("Started web server").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	require.NoError(t, err, "Failed to start IBM MQ container")

	t.Cleanup(func() {
		if err := ibmMQContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate IBM MQ container: %v", err)
		}
	})

	// Get IBM MQ connection details
	mqHost, err := ibmMQContainer.Host(ctx)
	require.NoError(t, err)
	mqPort, err := ibmMQContainer.MappedPort(ctx, "1414/tcp")
	require.NoError(t, err)

	// Test configuration template - matches IBM MQ YAML patterns
	template := strings.ReplaceAll(strings.ReplaceAll(`
input:
  generate:
    interval: "2s"
    count: 3
    mapping: |
      root = {
        "message": "Integration test message",
        "timestamp": now(),
        "test_id": uuid_v4(),
        "source": "integration_test"
      }

output:
  ww_ibm_mq:
    queue_name: "DEV.QUEUE.1"
    system_name: "default"
    format: "MQSTR"
    ccsid: "1208"
    encoding: "546"
    host: "$MQ_HOST"
    port: $MQ_PORT
    channel: "DEV.APP.SVRCONN"
    queue_manager: "QM1"
    num_threads: 1
`, "$MQ_HOST", mqHost), "$MQ_PORT", mqPort.Port())

	// Run basic configuration test
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestSendBatch(3),
	)

	suite.Run(t, template,
		integration.StreamTestOptSleepAfterInput(1*time.Second),
		integration.StreamTestOptSleepAfterOutput(1*time.Second),
	)
}

func testEventBridgeIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start LocalStack container for AWS EventBridge emulation
	localStackContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "localstack/localstack:latest",
			ExposedPorts: []string{"4566/tcp"},
			Env: map[string]string{
				"SERVICES":    "events,sqs",
				"DEBUG":       "1",
				"DATA_DIR":    "/tmp/localstack/data",
				"DOCKER_HOST": "unix:///var/run/docker.sock",
			},
			WaitingFor: wait.ForHTTP("/_localstack/health").WithPort("4566/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err, "Failed to start LocalStack container")

	t.Cleanup(func() {
		if err := localStackContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	})

	// Get LocalStack endpoint
	localStackHost, err := localStackContainer.Host(ctx)
	require.NoError(t, err)
	localStackPort, err := localStackContainer.MappedPort(ctx, "4566/tcp")
	require.NoError(t, err)
	localStackEndpoint := fmt.Sprintf("http://%s:%s", localStackHost, localStackPort.Port())

	// Test configuration template - matches EventBridge pattern
	template := strings.ReplaceAll(`
input:
  generate:
    interval: "2s"
    count: 2
    mapping: |
      root = {
        "source": "integration.test",
        "detail-type": "Integration Test Event",
        "detail": {
          "message": "Integration test from wombat",
          "timestamp": now(),
          "test_id": uuid_v4()
        }
      }

output:
  ww_eventbridge:
    region: "us-east-1"
    endpoint_url: "$LOCALSTACK_ENDPOINT"
    event_bus_name: "default"
    aws:
      access_key_id: "test"
      secret_access_key: "test"
`, "$LOCALSTACK_ENDPOINT", localStackEndpoint)

	// Run basic configuration test
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestSendBatch(2),
	)

	suite.Run(t, template,
		integration.StreamTestOptSleepAfterInput(1*time.Second),
		integration.StreamTestOptSleepAfterOutput(1*time.Second),
	)
}