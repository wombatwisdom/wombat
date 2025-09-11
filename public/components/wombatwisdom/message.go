package wombatwisdom

import (
	"fmt"
	"iter"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/framework/spec"
)

type MessageFactory struct {
}

func (m *MessageFactory) NewBatch(msg ...spec.Message) spec.Batch {
	return &BenthosBatch{
		msgs: msg,
	}
}

func (m *MessageFactory) NewMessage() spec.Message {
	return &BenthosMessage{
		Message: service.NewMessage(nil),
	}
}

type BenthosBatch struct {
	msgs []spec.Message
}

func (b *BenthosBatch) Messages() iter.Seq2[int, spec.Message] {
	return func(yield func(int, spec.Message) bool) {
		for i, m := range b.msgs {
			if !yield(i, m) {
				return
			}
		}
	}
}

func (b *BenthosBatch) Append(msg spec.Message) {
	b.msgs = append(b.msgs, msg)
}

type BenthosMessage struct {
	Message *service.Message
}

func (b *BenthosMessage) SetMetadata(key string, value any) {
	if b.Message == nil {
		return
	}

	b.Message.MetaSetMut(key, value)
}

func (b *BenthosMessage) SetRaw(bytes []byte) {
	if b.Message == nil {
		return
	}

	b.Message.SetBytes(bytes)
}

func (b *BenthosMessage) Raw() ([]byte, error) {
	if b.Message == nil {
		return nil, fmt.Errorf("message is nil")
	}

	return b.Message.AsBytes()
}

func (b *BenthosMessage) Metadata() iter.Seq2[string, any] {
	if b.Message == nil {
		return func(yield func(string, any) bool) {}
	}

	return func(yield func(string, any) bool) {
		_ = b.Message.MetaWalkMut(func(key string, value any) error {
			yield(key, value)
			return nil
		})
	}
}
