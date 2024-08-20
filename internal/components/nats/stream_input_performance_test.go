package nats

import (
	"context"
	"github.com/redpanda-data/benthos/v4/public/service"
	_ "github.com/redpanda-data/connect/public/bundle/free/v4"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStreamInputPerformance(t *testing.T) {
	sb := service.NewStreamBuilder()
	err := sb.SetYAML(`
input:
  read_until:
    check: count("messages") >= 1000
    input:
      jetstream_stream:
        durable: "wombat"
        urls: ['tls://connect.ngs.global']
        auth:
          user_credentials_file: '/Users/calmera/Downloads/NGS-Experiment-brain.creds'
        stream: "benchstream"
        subjects:
          - ">"
        deliver: "all"
        max_ack_pending: 1

output:
  jetstream_stream:
    urls: ['tls://connect.ngs.global']
    auth:
      user_credentials_file: '/Users/calmera/Downloads/NGS-Experiment-brain.creds'
    metadata:
      include_patterns:
        - ".*"
    subject: "received.wombat.${!@nats_subject}"
    max_in_flight: 1
`)
	assert.NoError(t, err)

	str, err := sb.Build()
	if err != nil {
		assert.NoError(t, err)
	}

	if err := str.Run(context.Background()); err != nil {
		assert.Error(t, err)
	}
}

func TestLocalStreamInputPerformance(t *testing.T) {
	sb := service.NewStreamBuilder()
	err := sb.SetYAML(`
input:
  read_until:
    check: count("messages") >= 1000
    input:
      jetstream_stream:
        durable: "wombat"
        urls: ['nats://localhost:4222']
        stream: "benchstream"
        subjects:
          - ">"
        deliver: "all"
        max_ack_pending: 1

output:
  jetstream_stream:
    urls: ['nats://localhost:4222']
    metadata:
      include_patterns:
        - ".*"
    subject: "received.wombat.${!@nats_subject}"
    max_in_flight: 1
`)
	assert.NoError(t, err)

	str, err := sb.Build()
	if err != nil {
		assert.NoError(t, err)
	}

	if err := str.Run(context.Background()); err != nil {
		assert.Error(t, err)
	}
}

func TestStreamInputOnlyPerformance(t *testing.T) {
	sb := service.NewStreamBuilder()
	err := sb.SetYAML(`
input:
  read_until:
    check: count("messages") >= 1000
    input:
      jetstream_stream:
        durable: "wombat"
        urls: ['tls://connect.ngs.global']
        auth:
#          user_credentials_file: '${HOME}/NGS-Experiment-brain.creds'
          user_credentials_file: '/Users/calmera/Downloads/NGS-Experiment-brain.creds'
        stream: "benchstream"
        subjects:
          - ">"
        deliver: "all"
        max_ack_pending: 1

output:
  drop: {}
`)
	assert.NoError(t, err)

	str, err := sb.Build()
	if err != nil {
		assert.NoError(t, err)
	}

	if err := str.Run(context.Background()); err != nil {
		assert.Error(t, err)
	}
}

func TestLocalStreamInputOnlyPerformance(t *testing.T) {
	sb := service.NewStreamBuilder()
	err := sb.SetYAML(`
input:
  read_until:
    check: count("messages") >= 1000
    input:
      jetstream_stream:
        durable: "wombat"
        urls: ['nats://localhost:4222']
        stream: "benchstream"
        subjects:
          - ">"
        deliver: "all"
        max_ack_pending: 1

output:
  drop: {}
`)
	assert.NoError(t, err)

	str, err := sb.Build()
	if err != nil {
		assert.NoError(t, err)
	}

	if err := str.Run(context.Background()); err != nil {
		assert.Error(t, err)
	}
}
