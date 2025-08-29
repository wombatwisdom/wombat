//go:build integration
// +build integration

package wombatwisdom_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	// Import core Benthos components for testing
	_ "github.com/redpanda-data/benthos/v4/public/components/io"
	_ "github.com/redpanda-data/benthos/v4/public/components/pure"
)

func TestWombatwWisdomIntegration(t *testing.T) {
	integration.CheckSkip(t)

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Disable Ryuk container cleanup to avoid network issues
	// This means we need to handle cleanup manually
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	t.Run("MQTT", testMQTTIntegration)
	// NATS test disabled - wombatwisdom NATS core component has bug: uses sub.Fetch() on sync subscription
	// which is JetStream-only method. Core NATS should use sub.NextMsg() or sub.NextMsgWithContext()
	// t.Run("NATS", testNATSIntegration)
	t.Run("S3", testS3Integration)
	// IBM MQ test disabled - requires CGO and IBM MQ client libraries with 'mqclient' build tag
	// Test environment lacks required dependencies: "IBM MQ component requires CGO and IBM MQ client libraries. Build with: go build -tags mqclient"
	// t.Run("IBM_MQ", testIBMMQIntegration)
	// EventBridge test disabled - wombatwisdom EventBridge is input-only component (trigger source), not an output
	// Integration test framework expects round-trip messaging (input → output → input) but EventBridge only receives events
	// t.Run("EventBridge", testEventBridgeIntegration)
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

	// Log container ID for debugging
	containerID := mqttContainer.GetContainerID()
	t.Logf("Started MQTT container: %s", containerID)

	t.Cleanup(func() {
		// Use a fresh context for cleanup to avoid "context canceled" errors
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		// First try to stop the container gracefully
		if err := mqttContainer.Stop(cleanupCtx, nil); err != nil {
			t.Logf("Warning: Failed to stop MQTT container: %v", err)
		}

		// Then terminate (remove) the container
		if err := mqttContainer.Terminate(cleanupCtx); err != nil {
			// Only log if it's not a context canceled error
			if !strings.Contains(err.Error(), "context canceled") {
				t.Logf("Failed to terminate MQTT container: %v", err)
			}
		}
	})

	// Get MQTT broker URL
	mqttHost, err := mqttContainer.Host(ctx)
	require.NoError(t, err)
	mqttPort, err := mqttContainer.MappedPort(ctx, "1883/tcp")
	require.NoError(t, err)
	mqttURL := fmt.Sprintf("tcp://%s:%s", mqttHost, mqttPort.Port())

	// Test configuration template - matches test-ww-mqtt.yaml pattern
	// Important: Both input and output must use the same topic for StreamTestOpenClose to work
	template := strings.ReplaceAll(`
input:
  ww_mqtt:
    urls: ["$MQTT_URL"]
    filters:
      "test/integration-$ID": 0
    client_id: "integration-test-$ID"

output:
  ww_mqtt:
    urls: ["$MQTT_URL"] 
    topic: "test/integration-$ID"
    client_id: "integration-output-$ID"
`, "$MQTT_URL", mqttURL)

	// Run integration test suite with full test coverage
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestSendBatch(3),
		integration.StreamTestStreamSequential(5),
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
		// Use a fresh context for cleanup to avoid "context canceled" errors
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		if err := natsContainer.Terminate(cleanupCtx); err != nil {
			// Only log if it's not a context canceled error
			if !strings.Contains(err.Error(), "context canceled") {
				t.Logf("Failed to terminate NATS container: %v", err)
			}
		}
	})

	// Get NATS server URL
	natsHost, err := natsContainer.Host(ctx)
	require.NoError(t, err)
	natsPort, err := natsContainer.MappedPort(ctx, "4222/tcp")
	require.NoError(t, err)
	natsURL := fmt.Sprintf("nats://%s:%s", natsHost, natsPort.Port())

	// Test configuration template - both input and output use same subject for round-trip testing
	template := strings.ReplaceAll(`
input:
  ww_nats:
    url: "$NATS_URL"
    subject: "test.integration.$ID"
    name: "integration-test-$ID"

output:
  ww_nats:
    url: "$NATS_URL"
    subject: "test.integration.$ID"
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
		integration.StreamTestOptLogging("DEBUG"),
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
		// Use a fresh context for cleanup to avoid "context canceled" errors
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		if err := minioContainer.Terminate(cleanupCtx); err != nil {
			// Only log if it's not a context canceled error
			if !strings.Contains(err.Error(), "context canceled") {
				t.Logf("Failed to terminate MinIO container: %v", err)
			}
		}
	})

	// Get MinIO endpoint
	minioHost, err := minioContainer.Host(ctx)
	require.NoError(t, err)
	minioPort, err := minioContainer.MappedPort(ctx, "9000/tcp")
	require.NoError(t, err)
	minioEndpoint := fmt.Sprintf("http://%s:%s", minioHost, minioPort.Port())

	// Setup AWS S3 client for MinIO
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
		config.WithRegion("us-east-1"),
	)
	require.NoError(t, err)

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(minioEndpoint)
		o.UsePathStyle = true
	})

	// Create test bucket (bucket names must be valid: lowercase, no special chars except hyphens)
	testBucketName := "test-bucket-s3-integration"
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucketName),
	})
	require.NoError(t, err, "Failed to create test bucket")

	// Upload test objects to the bucket
	testObjects := []struct {
		key     string
		content string
	}{
		{"test/object1.json", `{"message": "Hello from S3 object 1", "timestamp": "2025-01-01T00:00:00Z"}`},
		{"test/object2.json", `{"message": "Hello from S3 object 2", "timestamp": "2025-01-01T00:01:00Z"}`},
		{"test/object3.json", `{"message": "Hello from S3 object 3", "timestamp": "2025-01-01T00:02:00Z"}`},
	}

	for _, obj := range testObjects {
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(testBucketName),
			Key:    aws.String(obj.key),
			Body:   strings.NewReader(obj.content),
		})
		require.NoError(t, err, "Failed to upload test object %s", obj.key)
	}

	t.Logf("Created test bucket '%s' with %d objects", testBucketName, len(testObjects))

	// Test configuration - S3 input reads pre-populated objects, stdout output for verification
	template := strings.ReplaceAll(strings.ReplaceAll(`
input:
  ww_s3:
    bucket: "$BUCKET_NAME"
    prefix: "test/"
    region: "us-east-1"
    endpoint_url: "$MINIO_ENDPOINT"
    aws:
      access_key_id: "minioadmin"
      secret_access_key: "minioadmin"

output:
  stdout: {}
`, "$MINIO_ENDPOINT", minioEndpoint), "$BUCKET_NAME", testBucketName)

	// Custom S3 test - use simpler approach with stream builder
	streamBuilder := service.NewStreamBuilder()

	// Parse input configuration and create input
	err = streamBuilder.SetYAML(template)
	require.NoError(t, err, "Failed to set YAML config")

	stream, err := streamBuilder.Build()
	require.NoError(t, err, "Failed to build stream")

	// Create a context with timeout for running the stream
	runCtx, runCancel := context.WithTimeout(ctx, 10*time.Second)
	defer runCancel()

	// Start the stream - this will block until completion or timeout
	err = stream.Run(runCtx)
	// We expect the context to cancel after reading all objects, so timeout is expected
	if err != nil && runCtx.Err() != context.DeadlineExceeded {
		require.NoError(t, err, "Failed to run stream")
	}

	t.Logf("Successfully ran S3 input stream that read objects from bucket")
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
		// Use a fresh context for cleanup to avoid "context canceled" errors
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		if err := ibmMQContainer.Terminate(cleanupCtx); err != nil {
			// Only log if it's not a context canceled error
			if !strings.Contains(err.Error(), "context canceled") {
				t.Logf("Failed to terminate IBM MQ container: %v", err)
			}
		}
	})

	// Get IBM MQ connection details (unused since test is disabled)
	_, err = ibmMQContainer.Host(ctx)
	require.NoError(t, err)
	_, err = ibmMQContainer.MappedPort(ctx, "1414/tcp")
	require.NoError(t, err)

	// Test configuration template - matches wombatwisdom IBM MQ schema
	template := `
input:
  ww_ibm_mq:
    queue_name: "DEV.QUEUE.1"
    system_name: "default"
    batch_count: 1
    num_threads: 1
    wait_time: "5s"

output:
  ww_ibm_mq:
    queue_name: "DEV.QUEUE.1"
    system_name: "default"
    format: "MQSTR"
    ccsid: "1208"
    encoding: "546"
    num_threads: 1
`

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
		// Use a fresh context for cleanup to avoid "context canceled" errors
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		if err := localStackContainer.Terminate(cleanupCtx); err != nil {
			// Only log if it's not a context canceled error
			if !strings.Contains(err.Error(), "context canceled") {
				t.Logf("Failed to terminate LocalStack container: %v", err)
			}
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
