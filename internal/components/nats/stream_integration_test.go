package nats

import (
	"context"
	"fmt"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationNatsJetstream(t *testing.T) {
	integration.CheckSkip(t)
	t.Parallel()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	pool.MaxWait = time.Second * 30
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "nats",
		Tag:        "latest",
		Cmd:        []string{"--js"},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, pool.Purge(resource))
	})

	var natsConn *nats.Conn
	_ = resource.Expire(900)
	require.NoError(t, pool.Retry(func() error {
		natsConn, err = nats.Connect(fmt.Sprintf("tcp://localhost:%v", resource.GetPort("4222/tcp")))
		return err
	}))
	t.Cleanup(func() {
		natsConn.Close()
	})

	template := `
output:
  jetstream_stream:
    urls: [ nats://localhost:$PORT ]
    subject: subject-$ID

input:
  jetstream_stream:
    urls: [ nats://localhost:$PORT ]
    stream: stream-$ID
    msg_wait: 5s
    subjects:
      - subject-$ID
`
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		//integration.StreamTestMetadata(),
		integration.StreamTestSendBatch(10),
		// integration.StreamTestAtLeastOnceDelivery(), // TODO: SubscribeSync doesn't seem to honor durable setting
		integration.StreamTestStreamParallel(1000),
		integration.StreamTestStreamSequential(1000),
		integration.StreamTestStreamParallelLossy(1000),
		integration.StreamTestStreamParallelLossyThroughReconnect(1000),
	)
	suite.Run(
		t, template,
		//integration.StreamTestOptLogging("TRACE"),
		integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, vars *integration.StreamTestConfigVars) {
			js, err := natsConn.JetStream()
			require.NoError(t, err)

			streamName := "stream-" + vars.ID

			_, err = js.AddStream(&nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{"subject-" + vars.ID},
			})
			require.NoError(t, err)
		}),
		integration.StreamTestOptSleepAfterInput(100*time.Millisecond),
		integration.StreamTestOptSleepAfterOutput(100*time.Millisecond),
		integration.StreamTestOptPort(resource.GetPort("4222/tcp")),
	)
}

func TestIntegrationNatsJetstreamWithClaims(t *testing.T) {
	integration.CheckSkip(t)
	t.Parallel()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	pool.MaxWait = time.Second * 30
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "nats",
		Tag:        "latest",
		Cmd:        []string{"--js"},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, pool.Purge(resource))
	})

	var natsConn *nats.Conn
	_ = resource.Expire(900)
	require.NoError(t, pool.Retry(func() error {
		natsConn, err = nats.Connect(fmt.Sprintf("tcp://localhost:%v", resource.GetPort("4222/tcp")))
		return err
	}))
	t.Cleanup(func() {
		natsConn.Close()
	})

	template := `
output:
  jetstream_stream:
    urls: [ nats://localhost:$PORT ]
    subject: subject-$ID

input:
  jetstream_stream:
    urls: [ nats://localhost:$PORT ]
    stream: stream-$ID
    msg_wait: 5s
    claim_header: "nats_subject"
    subjects:
      - subject-$ID
`
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		//integration.StreamTestMetadata(),
		integration.StreamTestSendBatch(10),
		// integration.StreamTestAtLeastOnceDelivery(), // TODO: SubscribeSync doesn't seem to honor durable setting
		integration.StreamTestStreamParallel(1000),
		integration.StreamTestStreamSequential(1000),
		integration.StreamTestStreamParallelLossy(1000),
		integration.StreamTestStreamParallelLossyThroughReconnect(1000),
	)
	suite.Run(
		t, template,
		//integration.StreamTestOptLogging("TRACE"),
		integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, vars *integration.StreamTestConfigVars) {
			js, err := natsConn.JetStream()
			require.NoError(t, err)

			streamName := "stream-" + vars.ID

			_, err = js.AddStream(&nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{"subject-" + vars.ID},
			})
			require.NoError(t, err)
		}),
		integration.StreamTestOptSleepAfterInput(100*time.Millisecond),
		integration.StreamTestOptSleepAfterOutput(100*time.Millisecond),
		integration.StreamTestOptPort(resource.GetPort("4222/tcp")),
	)
}
