package nats

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"
)

// Mock types for testing
type mockMsg struct {
	data         []byte
	subject      string
	headers      nats.Header
	acked        bool
	naked        bool
	nakDelay     time.Duration
	metadataFunc func() (*jetstream.MsgMetadata, error)
}

func (m *mockMsg) Data() []byte         { return m.data }
func (m *mockMsg) Subject() string      { return m.subject }
func (m *mockMsg) Headers() nats.Header { return m.headers }
func (m *mockMsg) Ack() error {
	m.acked = true
	return nil
}
func (m *mockMsg) Nak() error {
	m.naked = true
	return nil
}
func (m *mockMsg) NakWithDelay(delay time.Duration) error {
	m.naked = true
	m.nakDelay = delay
	return nil
}
func (m *mockMsg) Metadata() (*jetstream.MsgMetadata, error) {
	if m.metadataFunc != nil {
		return m.metadataFunc()
	}
	return &jetstream.MsgMetadata{
		Sequence: jetstream.SequencePair{
			Stream:   123,
			Consumer: 456,
		},
		NumDelivered: 1,
		NumPending:   10,
		Domain:       "test-domain",
		Timestamp:    time.Unix(1234567890, 0),
	}, nil
}
func (m *mockMsg) Term() error                         { return nil }
func (m *mockMsg) InProgress() error                   { return nil }
func (m *mockMsg) Ack2() jetstream.PubAck              { return jetstream.PubAck{} }
func (m *mockMsg) Reply() string                       { return "" }
func (m *mockMsg) Size() int                           { return len(m.data) }
func (m *mockMsg) DoubleAck(ctx context.Context) error { return nil }
func (m *mockMsg) TermWithReason(reason string) error  { return nil }

type mockMessageResult struct {
	messages []jetstream.Msg
	err      error
	idx      int
	mu       sync.Mutex
}

func (m *mockMessageResult) Messages() <-chan jetstream.Msg {
	ch := make(chan jetstream.Msg)
	go func() {
		defer close(ch)
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, msg := range m.messages {
			ch <- msg
		}
	}()
	return ch
}

func (m *mockMessageResult) Error() error {
	return m.err
}

type mockConsumer struct {
	fetchHandler func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error)
}

func (m *mockConsumer) Fetch(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	if m.fetchHandler != nil {
		return m.fetchHandler(batch, opts...)
	}
	return &mockMessageResult{}, nil
}

func (m *mockConsumer) Messages(opts ...jetstream.PullMessagesOpt) (jetstream.MessagesContext, error) {
	return nil, errors.New("not implemented")
}

func (m *mockConsumer) Next(opts ...jetstream.FetchOpt) (jetstream.Msg, error) {
	return nil, errors.New("not implemented")
}

func (m *mockConsumer) Info(ctx context.Context) (*jetstream.ConsumerInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *mockConsumer) CachedInfo() *jetstream.ConsumerInfo { return nil }

// Add missing Consume method to satisfy the Consumer interface
func (m *mockConsumer) Consume(handler jetstream.MessageHandler, opts ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
	return nil, errors.New("not implemented")
}

func (m *mockConsumer) FetchBytes(maxBytes int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	return nil, errors.New("not implemented")
}

func (m *mockConsumer) FetchNoWait(batch int) (jetstream.MessageBatch, error) {
	return nil, errors.New("not implemented")
}

type mockJetStream struct {
	consumerHandler       func(ctx context.Context, stream, consumer string) (jetstream.Consumer, error)
	createConsumerHandler func(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error)
}

func (m *mockJetStream) Consumer(ctx context.Context, stream, consumer string) (jetstream.Consumer, error) {
	if m.consumerHandler != nil {
		return m.consumerHandler(ctx, stream, consumer)
	}
	return &mockConsumer{}, nil
}

func (m *mockJetStream) CreateOrUpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	if m.createConsumerHandler != nil {
		return m.createConsumerHandler(ctx, stream, cfg)
	}
	return &mockConsumer{}, nil
}

var _ = Describe("JetStream Input", func() {
	var (
		mgr *service.Resources
	)

	BeforeEach(func() {
		mgr = service.MockResources()
	})

	Describe("newJetStreamReaderFromConfig", func() {
		Context("with valid configuration", func() {
			It("should create reader with all delivery policy", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: all
ack_wait: 30s
max_ack_pending: 1024
batch_size: 10
flush_wait: 100ms
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader).NotTo(BeNil())
				Expect(reader.cfg.Name).To(Equal("test-consumer"))
				Expect(reader.stream).To(Equal("test-stream"))
				Expect(reader.cfg.DeliverPolicy).To(Equal(jetstream.DeliverAllPolicy))
				Expect(reader.batchSize).To(Equal(10))
				Expect(reader.flushWait).To(Equal(100 * time.Millisecond))
			})

			It("should create reader with last delivery policy", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: last
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.cfg.DeliverPolicy).To(Equal(jetstream.DeliverLastPolicy))
			})

			It("should create reader with by_start_time delivery policy", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: by_start_time
start_time: "2023-01-01T00:00:00Z"
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.cfg.DeliverPolicy).To(Equal(jetstream.DeliverByStartTimePolicy))
				Expect(reader.cfg.OptStartTime).NotTo(BeNil())
				Expect(reader.cfg.OptStartTime.Format(time.RFC3339)).To(Equal("2023-01-01T00:00:00Z"))
			})

			It("should create reader with by_start_sequence delivery policy", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: by_start_sequence
start_sequence: 1000
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.cfg.DeliverPolicy).To(Equal(jetstream.DeliverByStartSequencePolicy))
				Expect(reader.cfg.OptStartSeq).To(Equal(uint64(1000)))
			})

			It("should handle single filter subject", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: all
filter_subjects: ["foo.bar.baz"]
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.cfg.FilterSubject).To(Equal("foo.bar.baz"))
				Expect(reader.cfg.FilterSubjects).To(BeEmpty())
			})

			It("should handle multiple filter subjects", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: all
filter_subjects: ["foo.bar", "foo.baz", "foo.qux"]
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.cfg.FilterSubject).To(BeEmpty())
				Expect(reader.cfg.FilterSubjects).To(ConsistOf("foo.bar", "foo.baz", "foo.qux"))
			})

			It("should handle bind mode", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
bind: true
deliver: all
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.bind).To(BeTrue())
			})

			It("should handle nak_delay configuration", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: all
nak_delay: 5s
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				reader, err := newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.nakDelay).NotTo(BeNil())
				Expect(*reader.nakDelay).To(Equal(5 * time.Second))
			})
		})

		Context("with invalid configuration", func() {
			It("should fail with invalid deliver policy", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: invalid_policy
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				_, err = newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("deliver option invalid_policy was not recognised"))
			})

			It("should fail with by_start_time but no start_time", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: by_start_time
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				_, err = newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("start_time"))
			})

			It("should fail with invalid start_time format", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: by_start_time
start_time: "invalid-time"
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				_, err = newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse start time"))
			})

			It("should fail with invalid ack_wait duration", func() {
				confStr := `
urls: ["nats://localhost:4222"]
name: test-consumer
stream: test-stream
deliver: all
ack_wait: "invalid-duration"
`
				spec := natsJetStreamInputConfig()
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				_, err = newJetStreamReaderFromConfig(conf, mgr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse ack wait duration"))
			})
		})
	})

	Describe("ReadBatch", func() {
		var reader *jetStreamReader

		BeforeEach(func() {
			reader = &jetStreamReader{
				batchSize: 5,
				flushWait: 100 * time.Millisecond,
				log:       mgr.Logger(),
			}
		})

		It("should return error when not connected", func() {
			reader.consumer = nil
			_, _, err := reader.ReadBatch(context.Background())
			Expect(err).To(Equal(service.ErrNotConnected))
		})

		It("should successfully read batch of messages", func() {
			msg1 := &mockMsg{
				data:    []byte("message 1"),
				subject: "test.subject",
				headers: nats.Header{"X-Test": []string{"header1"}},
			}
			msg2 := &mockMsg{
				data:    []byte("message 2"),
				subject: "test.subject",
				headers: nats.Header{"X-Test": []string{"header2"}},
			}
			messages := []jetstream.Msg{msg1, msg2}

			mockConsumer := &mockConsumer{
				fetchHandler: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					return &mockMessageResult{messages: messages}, nil
				},
			}
			reader.consumer = mockConsumer

			batch, ackFunc, err := reader.ReadBatch(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(batch).To(HaveLen(2))
			Expect(ackFunc).NotTo(BeNil())

			// Test message content
			msgBytes1, _ := batch[0].AsBytes()
			msgBytes2, _ := batch[1].AsBytes()
			Expect(string(msgBytes1)).To(Equal("message 1"))
			Expect(string(msgBytes2)).To(Equal("message 2"))

			// Test ACK functionality
			err = ackFunc(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(msg1.acked).To(BeTrue())
			Expect(msg2.acked).To(BeTrue())
		})

		It("should NAK messages on error", func() {
			msg := &mockMsg{data: []byte("message 1")}
			messages := []jetstream.Msg{msg}

			mockConsumer := &mockConsumer{
				fetchHandler: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					return &mockMessageResult{messages: messages}, nil
				},
			}
			reader.consumer = mockConsumer

			batch, ackFunc, err := reader.ReadBatch(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(batch).To(HaveLen(1))

			// Test NAK functionality
			err = ackFunc(context.Background(), errors.New("processing error"))
			Expect(err).NotTo(HaveOccurred())
			Expect(msg.naked).To(BeTrue())
		})

		It("should NAK messages with delay when configured", func() {
			reader.nakDelay = new(time.Duration)
			*reader.nakDelay = 5 * time.Second

			msg := &mockMsg{data: []byte("message 1")}
			messages := []jetstream.Msg{msg}

			mockConsumer := &mockConsumer{
				fetchHandler: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					return &mockMessageResult{messages: messages}, nil
				},
			}
			reader.consumer = mockConsumer

			_, ackFunc, err := reader.ReadBatch(context.Background())
			Expect(err).NotTo(HaveOccurred())

			err = ackFunc(context.Background(), errors.New("processing error"))
			Expect(err).NotTo(HaveOccurred())
			Expect(msg.naked).To(BeTrue())
			Expect(msg.nakDelay).To(Equal(5 * time.Second))
		})

		It("should handle fetch error", func() {
			mockConsumer := &mockConsumer{
				fetchHandler: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					return nil, errors.New("fetch error")
				},
			}
			reader.consumer = mockConsumer

			_, _, err := reader.ReadBatch(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fetch error"))
		})

		It("should handle message result error", func() {
			mockConsumer := &mockConsumer{
				fetchHandler: func(batch int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
					return &mockMessageResult{err: errors.New("result error")}, nil
				},
			}
			reader.consumer = mockConsumer

			_, _, err := reader.ReadBatch(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("result error"))
		})
	})

	Describe("convertMessage", func() {
		It("should convert message with all metadata", func() {
			mockMsg := &mockMsg{
				data:    []byte("test message"),
				subject: "test.subject",
				headers: nats.Header{
					"X-Custom-Header": []string{"value1"},
					"X-Another":       []string{"value2"},
				},
			}

			msg := convertMessage(mockMsg)
			Expect(msg).NotTo(BeNil())

			// Check message data
			msgBytes, _ := msg.AsBytes()
			Expect(string(msgBytes)).To(Equal("test message"))

			// Check metadata
			subject, _ := msg.MetaGet("nats_subject")
			Expect(subject).To(Equal("test.subject"))

			seqStream, _ := msg.MetaGet("nats_sequence_stream")
			Expect(seqStream).To(Equal("123"))

			seqConsumer, _ := msg.MetaGet("nats_sequence_consumer")
			Expect(seqConsumer).To(Equal("456"))

			numDelivered, _ := msg.MetaGet("nats_num_delivered")
			Expect(numDelivered).To(Equal("1"))

			numPending, _ := msg.MetaGet("nats_num_pending")
			Expect(numPending).To(Equal("10"))

			domain, _ := msg.MetaGet("nats_domain")
			Expect(domain).To(Equal("test-domain"))

			timestamp, _ := msg.MetaGet("nats_timestamp_unix_nano")
			Expect(timestamp).To(Equal("1234567890000000000"))

			// Check headers
			customHeader, _ := msg.MetaGet("X-Custom-Header")
			Expect(customHeader).To(Equal("value1"))

			anotherHeader, _ := msg.MetaGet("X-Another")
			Expect(anotherHeader).To(Equal("value2"))
		})

		It("should handle message without metadata", func() {
			mockMsg := &mockMsg{
				data:    []byte("test message"),
				subject: "test.subject",
				metadataFunc: func() (*jetstream.MsgMetadata, error) {
					return nil, errors.New("no metadata")
				},
			}

			msg := convertMessage(mockMsg)
			Expect(msg).NotTo(BeNil())

			// Subject should still be set
			subject, _ := msg.MetaGet("nats_subject")
			Expect(subject).To(Equal("test.subject"))

			// Other metadata should not be set
			_, exists := msg.MetaGet("nats_sequence_stream")
			Expect(exists).To(BeFalse())
		})

		It("should handle empty headers", func() {
			mockMsg := &mockMsg{
				data:    []byte("test message"),
				subject: "test.subject",
				headers: nats.Header{
					"Empty-Header": []string{""},
				},
			}

			msg := convertMessage(mockMsg)
			Expect(msg).NotTo(BeNil())

			// Empty header should not be set
			_, exists := msg.MetaGet("Empty-Header")
			Expect(exists).To(BeFalse())
		})
	})
})

// Tests are run via the main nats_suite_test.go
