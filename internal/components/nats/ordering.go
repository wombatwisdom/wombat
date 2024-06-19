package nats

import (
	"context"
	"github.com/Jeffail/checkpoint"
	"github.com/Jeffail/shutdown"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redpanda-data/benthos/v4/public/service"
	"sync"
	"time"
)

type batchWithAckFn struct {
	onAck func()
	batch service.MessageBatch
}

type msgWithRecord struct {
	msg *service.Message
	r   jetstream.Msg
}

type subjectTracker struct {
	batcherLock    sync.Mutex
	topBatchRecord jetstream.Msg
	batcher        *service.Batcher

	checkpointerLock sync.Mutex
	checkpointer     *checkpoint.Uncapped[jetstream.Msg]

	outBatchChan chan<- batchWithAckFn
	commitFn     func(r jetstream.Msg)

	shutSig *shutdown.Signaller
}

func newSubjectTracker(batcher *service.Batcher, batchChan chan<- batchWithAckFn, commitFn func(r jetstream.Msg)) *subjectTracker {
	pt := &subjectTracker{
		batcher:      batcher,
		checkpointer: checkpoint.NewUncapped[jetstream.Msg](),
		outBatchChan: batchChan,
		commitFn:     commitFn,
		shutSig:      shutdown.NewSignaller(),
	}
	go pt.loop()
	return pt
}

func (p *subjectTracker) loop() {
	defer func() {
		if p.batcher != nil {
			p.batcher.Close(context.Background())
		}
		p.shutSig.TriggerHasStopped()
	}()

	// No need to loop when there's no batcher for async writes.
	if p.batcher == nil {
		return
	}

	var flushBatch <-chan time.Time
	var flushBatchTicker *time.Ticker
	adjustTimedFlush := func() {
		if flushBatch != nil || p.batcher == nil {
			return
		}

		tNext, exists := p.batcher.UntilNext()
		if !exists {
			if flushBatchTicker != nil {
				flushBatchTicker.Stop()
				flushBatchTicker = nil
			}
			return
		}

		if flushBatchTicker != nil {
			flushBatchTicker.Reset(tNext)
		} else {
			flushBatchTicker = time.NewTicker(tNext)
		}
		flushBatch = flushBatchTicker.C
	}

	closeAtLeisureCtx, done := p.shutSig.SoftStopCtx(context.Background())
	defer done()

	for {
		adjustTimedFlush()
		select {
		case <-flushBatch:
			var sendBatch service.MessageBatch
			var sendRecord jetstream.Msg

			// Wrap this in a closure to make locking/unlocking easier.
			func() {
				p.batcherLock.Lock()
				defer p.batcherLock.Unlock()

				flushBatch = nil
				if tNext, exists := p.batcher.UntilNext(); !exists || tNext > 1 {
					// This can happen if a pushed message triggered a batch before
					// the last known flush period. In this case we simply enter the
					// loop again which readjusts our flush batch timer.
					return
				}

				if sendBatch, _ = p.batcher.Flush(closeAtLeisureCtx); len(sendBatch) == 0 {
					return
				}
				sendRecord = p.topBatchRecord
				p.topBatchRecord = nil
			}()

			if len(sendBatch) > 0 {
				if err := p.sendBatch(closeAtLeisureCtx, sendBatch, sendRecord); err != nil {
					return
				}
			}
		case <-p.shutSig.SoftStopChan():
			return
		}
	}
}

func (p *subjectTracker) sendBatch(ctx context.Context, b service.MessageBatch, r jetstream.Msg) error {
	p.checkpointerLock.Lock()
	releaseFn := p.checkpointer.Track(r, int64(len(b)))
	p.checkpointerLock.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.outBatchChan <- batchWithAckFn{
		batch: b,
		onAck: func() {
			p.checkpointerLock.Lock()
			releaseRecord := releaseFn()
			p.checkpointerLock.Unlock()

			if releaseRecord != nil && *releaseRecord != nil {
				p.commitFn(*releaseRecord)
			}
		},
	}:
	}
	return nil
}

func (p *subjectTracker) add(ctx context.Context, m *msgWithRecord, limit int) (pauseFetch bool) {
	var sendBatch service.MessageBatch
	if p.batcher != nil {
		// Wrap this in a closure to make locking/unlocking easier.
		func() {
			p.batcherLock.Lock()
			defer p.batcherLock.Unlock()

			if p.batcher.Add(m.msg) {
				// Batch triggered, we flush it here synchronously.
				sendBatch, _ = p.batcher.Flush(ctx)
			} else {
				// Otherwise store the latest record as the representative of the
				// pending batch offset. This will be used by the timer based
				// flushing mechanism within loop() if applicable.
				p.topBatchRecord = m.r
			}
		}()
	} else {
		sendBatch = service.MessageBatch{m.msg}
	}

	if len(sendBatch) > 0 {
		// Ignoring in the error here is fine, it implies shut down has been
		// triggered and we would only acknowledge the message by committing it
		// if it were successfully delivered.
		_ = p.sendBatch(ctx, sendBatch, m.r)
	}

	p.checkpointerLock.Lock()
	pauseFetch = p.checkpointer.Pending() >= int64(limit)
	p.checkpointerLock.Unlock()
	return
}

func (p *subjectTracker) pauseFetch(limit int) (pauseFetch bool) {
	p.checkpointerLock.Lock()
	pauseFetch = p.checkpointer.Pending() >= int64(limit)
	p.checkpointerLock.Unlock()
	return
}

func (p *subjectTracker) close(ctx context.Context) error {
	p.shutSig.TriggerSoftStop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.shutSig.HasStoppedChan():
	}
	return nil
}

//------------------------------------------------------------------------------

type checkpointTracker struct {
	mut      sync.Mutex
	subjects map[string]*subjectTracker

	res       *service.Resources
	batchChan chan<- batchWithAckFn
	commitFn  func(r jetstream.Msg)
	batchPol  service.BatchPolicy
}

func newCheckpointTracker(
	res *service.Resources,
	batchChan chan<- batchWithAckFn,
	releaseFn func(r jetstream.Msg),
	batchPol service.BatchPolicy,
) *checkpointTracker {
	return &checkpointTracker{
		subjects:  map[string]*subjectTracker{},
		res:       res,
		batchChan: batchChan,
		commitFn:  releaseFn,
		batchPol:  batchPol,
	}
}

func (c *checkpointTracker) close() {
	c.mut.Lock()
	defer c.mut.Unlock()

	for _, tracker := range c.subjects {
		_ = tracker.close(context.Background())
	}
}

func (c *checkpointTracker) addRecord(ctx context.Context, m *msgWithRecord, limit int) (pauseFetch bool) {
	c.mut.Lock()
	defer c.mut.Unlock()

	st := c.subjects[m.r.Subject()]
	if st == nil {
		var batcher *service.Batcher
		if !c.batchPol.IsNoop() {
			var err error
			if batcher, err = c.batchPol.NewBatcher(c.res); err != nil {
				c.res.Logger().Errorf("Failed to initialise batch policy: %v, falling back to individual message delivery", err)
				batcher = nil
			}
		}
		st = newSubjectTracker(batcher, c.batchChan, c.commitFn)
		c.subjects[m.r.Subject()] = st
	}

	return st.add(ctx, m, limit)
}

func (c *checkpointTracker) pauseFetch(subject string, limit int) bool {
	c.mut.Lock()
	defer c.mut.Unlock()

	st := c.subjects[subject]
	if st == nil {
		return false
	}

	return st.pauseFetch(limit)
}

//------------------------------------------------------------------------------
