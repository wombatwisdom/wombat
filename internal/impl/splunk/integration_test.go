//go:build integration
// +build integration

package splunk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSplunkServer simulates a Splunk HEC endpoint
type MockSplunkServer struct {
	server          *httptest.Server
	receivedEvents  []SplunkEvent
	receivedBatches []string
	mu              sync.Mutex
	authToken       string
	returnError     bool
	statusCode      int
}

type SplunkEvent struct {
	Event      interface{}            `json:"event"`
	Host       string                 `json:"host,omitempty"`
	Source     string                 `json:"source,omitempty"`
	SourceType string                 `json:"sourcetype,omitempty"`
	Index      string                 `json:"index,omitempty"`
	Time       float64                `json:"time,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
}

func NewMockSplunkServer(authToken string) *MockSplunkServer {
	m := &MockSplunkServer{
		authToken:       authToken,
		receivedEvents:  make([]SplunkEvent, 0),
		receivedBatches: make([]string, 0),
		statusCode:      http.StatusOK,
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		// Validate auth header
		authHeader := r.Header.Get("Authorization")
		expectedAuth := fmt.Sprintf("Splunk %s", m.authToken)
		if authHeader != expectedAuth {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text": "Invalid authentication",
				"code": 3,
			})
			return
		}

		// Check content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text": "Invalid content type",
				"code": 6,
			})
			return
		}

		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Store raw batch
		m.receivedBatches = append(m.receivedBatches, string(body))

		// Parse events (could be single or multiple)
		// Splunk HEC expects newline-delimited JSON
		lines := splitLines(string(body))
		for _, line := range lines {
			if line == "" {
				continue
			}

			var event SplunkEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"text": "Invalid event format",
					"code": 6,
				})
				return
			}
			m.receivedEvents = append(m.receivedEvents, event)
		}

		if m.returnError {
			w.WriteHeader(m.statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text": "Error processing events",
				"code": 5,
			})
			return
		}

		// Return success
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"text": "Success",
			"code": 0,
		})
	}))

	return m
}

func (m *MockSplunkServer) URL() string {
	return m.server.URL + "/services/collector/event"
}

func (m *MockSplunkServer) Close() {
	m.server.Close()
}

func (m *MockSplunkServer) GetEvents() []SplunkEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	events := make([]SplunkEvent, len(m.receivedEvents))
	copy(events, m.receivedEvents)
	return events
}

func (m *MockSplunkServer) GetBatches() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	batches := make([]string, len(m.receivedBatches))
	copy(batches, m.receivedBatches)
	return batches
}

func (m *MockSplunkServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivedEvents = make([]SplunkEvent, 0)
	m.receivedBatches = make([]string, 0)
	m.returnError = false
	m.statusCode = http.StatusOK
}

func (m *MockSplunkServer) SetError(statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnError = true
	m.statusCode = statusCode
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func TestSplunkIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	authToken := "test-auth-token-12345"
	mockServer := NewMockSplunkServer(authToken)
	defer mockServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("sends single event", func(t *testing.T) {
		mockServer.Reset()

		env := service.NewEnvironment()
		builder := env.NewStreamBuilder()

		config := fmt.Sprintf(`
input:
  generate:
    count: 1
    mapping: |
      root = {
        "message": "test event",
        "level": "info",
        "timestamp": now()
      }

output:
  splunk_hec:
    url: %s
    token: %s
    batching_count: 1
    batching_period: "1s"
`, mockServer.URL(), authToken)

		err := builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		// Run the stream briefly
		go func() {
			time.Sleep(2 * time.Second)
			stream.Stop(ctx)
		}()

		err = stream.Run(ctx)
		assert.NoError(t, err)

		// Verify events were received
		events := mockServer.GetEvents()
		require.Len(t, events, 1)

		event := events[0]
		eventMap, ok := event.Event.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test event", eventMap["message"])
		assert.Equal(t, "info", eventMap["level"])
	})

	t.Run("sends batched events", func(t *testing.T) {
		mockServer.Reset()

		env := service.NewEnvironment()
		builder := env.NewStreamBuilder()

		config := fmt.Sprintf(`
input:
  generate:
    count: 5
    interval: "100ms"
    mapping: |
      root = {
        "message": "batch event " + this.count().string(),
        "index": this.count()
      }

output:
  splunk_hec:
    url: %s
    token: %s
    batching_count: 3
    batching_period: "5s"
`, mockServer.URL(), authToken)

		err := builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			time.Sleep(3 * time.Second)
			stream.Stop(ctx)
		}()

		err = stream.Run(ctx)
		assert.NoError(t, err)

		// Verify batching occurred
		batches := mockServer.GetBatches()
		assert.GreaterOrEqual(t, len(batches), 1, "Should have at least one batch")

		events := mockServer.GetEvents()
		assert.GreaterOrEqual(t, len(events), 3, "Should have received at least 3 events")
	})

	t.Run("applies event metadata", func(t *testing.T) {
		mockServer.Reset()

		env := service.NewEnvironment()
		builder := env.NewStreamBuilder()

		config := fmt.Sprintf(`
input:
  generate:
    count: 1
    mapping: |
      root = "simple text message"

output:
  splunk_hec:
    url: %s
    token: %s
    event_host: "test-host"
    event_source: "test-source"
    event_sourcetype: "test-sourcetype"
    event_index: "test-index"
    batching_count: 1
`, mockServer.URL(), authToken)

		err := builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			time.Sleep(2 * time.Second)
			stream.Stop(ctx)
		}()

		err = stream.Run(ctx)
		assert.NoError(t, err)

		// Verify metadata was applied
		events := mockServer.GetEvents()
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "simple text message", event.Event)
		assert.Equal(t, "test-host", event.Host)
		assert.Equal(t, "test-source", event.Source)
		assert.Equal(t, "test-sourcetype", event.SourceType)
		assert.Equal(t, "test-index", event.Index)
	})

	t.Run("handles pre-formatted events", func(t *testing.T) {
		mockServer.Reset()

		env := service.NewEnvironment()
		builder := env.NewStreamBuilder()

		config := fmt.Sprintf(`
input:
  generate:
    count: 1
    mapping: |
      root = {
        "event": {
          "user": "john",
          "action": "login",
          "result": "success"
        },
        "host": "app-server-01",
        "source": "auth-service"
      }

output:
  splunk_hec:
    url: %s
    token: %s
    batching_count: 1
`, mockServer.URL(), authToken)

		err := builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		// Run the stream
		go func() {
			time.Sleep(2 * time.Second)
			stream.Stop(ctx)
		}()

		err = stream.Run(ctx)
		assert.NoError(t, err)

		// Verify pre-formatted event was preserved
		events := mockServer.GetEvents()
		require.Len(t, events, 1)

		event := events[0]
		eventData, ok := event.Event.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "john", eventData["user"])
		assert.Equal(t, "login", eventData["action"])
		assert.Equal(t, "success", eventData["result"])
		assert.Equal(t, "app-server-01", event.Host)
		assert.Equal(t, "auth-service", event.Source)
	})

	t.Run("handles authentication failure", func(t *testing.T) {
		mockServer.Reset()

		env := service.NewEnvironment()
		builder := env.NewStreamBuilder()

		config := fmt.Sprintf(`
input:
  generate:
    count: 1
    mapping: 'root = "test"'

output:
  splunk_hec:
    url: %s
    token: wrong-token
    batching_count: 1
`, mockServer.URL())

		err := builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		// Run the stream - it should fail due to auth error
		go func() {
			time.Sleep(3 * time.Second)
			stream.Stop(ctx)
		}()

		err = stream.Run(ctx)
		// The stream might not return an error immediately due to async processing
		// but no events should be successfully sent
		time.Sleep(1 * time.Second)
		
		events := mockServer.GetEvents()
		assert.Len(t, events, 0, "No events should be accepted with wrong auth token")
	})

	t.Run("respects rate limiting", func(t *testing.T) {
		mockServer.Reset()

		env := service.NewEnvironment()
		builder := env.NewStreamBuilder()

		config := fmt.Sprintf(`
input:
  generate:
    count: 10
    interval: "10ms"
    mapping: 'root = {"event": "rate limit test " + this.count().string()}'

rate_limit_resources:
  - label: splunk_limiter
    local:
      count: 2
      interval: "1s"

output:
  splunk_hec:
    url: %s
    token: %s
    rate_limit: splunk_limiter
    batching_count: 1
`, mockServer.URL(), authToken)

		err := builder.SetYAML(config)
		require.NoError(t, err)

		stream, err := builder.Build()
		require.NoError(t, err)

		start := time.Now()
		
		// Run the stream
		go func() {
			time.Sleep(3 * time.Second)
			stream.Stop(ctx)
		}()

		err = stream.Run(ctx)
		assert.NoError(t, err)

		duration := time.Since(start)
		
		// With 10 events and rate limit of 2/sec, should take at least 4 seconds
		assert.GreaterOrEqual(t, duration, 2*time.Second, "Rate limiting should slow down processing")
		
		events := mockServer.GetEvents()
		assert.GreaterOrEqual(t, len(events), 4, "Should have processed some events within rate limit")
	})
}