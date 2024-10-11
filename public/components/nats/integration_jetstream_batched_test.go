package nats

import (
	"context"
	"github.com/nats-io/nats.go/jetstream"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nats-io/nats-server/v2/test"
	"github.com/redpanda-data/benthos/v4/public/service/integration"
)

func TestIntegrationNatsPullConsumer(t *testing.T) {
	integration.CheckSkip(t)
	//t.Parallel()

	var err error
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	opts.StoreDir, err = os.MkdirTemp("", "wombat-nats-test-")
	assert.NoError(t, err)

	srv := test.RunServer(&opts)
	t.Cleanup(func() {
		srv.Shutdown()
		os.RemoveAll(opts.StoreDir)
	})

	natsConn, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() {
		natsConn.Close()
	})
	js, err := jetstream.New(natsConn)
	require.NoError(t, err)

	template := strings.ReplaceAll(`
output:
  nats_jetstream_batched:
    urls: 
      - $URL
    subject: subject-$ID

input:
  nats_jetstream_batched:
    name: $ID
    urls: [ $URL ]
    stream: stream-$ID
    flush_wait: 2s
    filter_subjects:
      - subject-$ID
`, "$URL", srv.ClientURL())
	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		// integration.StreamTestMetadata(), TODO
		integration.StreamTestSendBatch(10),
		// integration.StreamTestAtLeastOnceDelivery(), // TODO: SubscribeSync doesn't seem to honor durable setting
		integration.StreamTestStreamParallel(1000),
		integration.StreamTestStreamSequential(1000),
		integration.StreamTestStreamParallelLossy(1000),
		integration.StreamTestStreamParallelLossyThroughReconnect(1000),
	)
	suite.Run(
		t, template,
		integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, vars *integration.StreamTestConfigVars) {
			streamName := "stream-" + vars.ID

			_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
				Name:     streamName,
				Subjects: []string{"subject-" + vars.ID},
			})
			require.NoError(t, err)
		}),
		integration.StreamTestOptSleepAfterInput(100*time.Millisecond),
		integration.StreamTestOptSleepAfterOutput(100*time.Millisecond),
	)
}
