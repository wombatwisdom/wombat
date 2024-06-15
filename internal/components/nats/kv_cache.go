package nats

import (
  "context"
  "errors"
  "github.com/google/uuid"
  "github.com/nats-io/nats.go/jetstream"
  "github.com/redpanda-data/benthos/v4/public/service"
  "sync"
  "time"

  "github.com/nats-io/nats.go"

  "github.com/Jeffail/shutdown"
)

func natsKVCacheConfig() *service.ConfigSpec {
  return service.NewConfigSpec().
    Categories("Services").
    Version("4.27.0").
    Summary("Cache key/values in a NATS key-value bucket.").
    Description(connectionNameDescription() + authDescription()).
    Fields(kvDocs()...)
}

func init() {
  err := service.RegisterCache(
    "jetstream_kv", natsKVCacheConfig(),
    func(conf *service.ParsedConfig, mgr *service.Resources) (service.Cache, error) {
      return newKVCache(conf, mgr)
    },
  )
  if err != nil {
    panic(err)
  }
}

type kvCache struct {
  connDetails connectionDetails
  bucket      string

  log *service.Logger

  shutSig *shutdown.Signaller

  connMut  sync.RWMutex
  natsConn *nats.Conn
  kv       jetstream.KeyValue

  // The pool caller id. This is a unique identifier we will provide when calling methods on the pool. This is used by
  // the pool to do reference counting and ensure that connections are only closed when they are no longer in use.
  pcid string
}

func newKVCache(conf *service.ParsedConfig, mgr *service.Resources) (*kvCache, error) {
  p := &kvCache{
    log:     mgr.Logger(),
    shutSig: shutdown.NewSignaller(),
    pcid:    uuid.New().String(),
  }

  var err error
  if p.connDetails, err = connectionDetailsFromParsed(conf, mgr); err != nil {
    return nil, err
  }

  if p.bucket, err = conf.FieldString(kvFieldBucket); err != nil {
    return nil, err
  }

  err = p.connect(context.Background())
  return p, err
}

func (p *kvCache) disconnect() {
  p.connMut.Lock()
  defer p.connMut.Unlock()

  if p.natsConn != nil {
    _ = pool.Release(p.pcid, p.connDetails)
    p.natsConn = nil
  }
  p.kv = nil
}

func (p *kvCache) connect(ctx context.Context) error {
  p.connMut.Lock()
  defer p.connMut.Unlock()

  if p.natsConn != nil {
    return nil
  }

  var err error
  if p.natsConn, err = pool.Get(ctx, p.pcid, p.connDetails); err != nil {
    return err
  }

  defer func() {
    if err != nil {
      _ = pool.Release(p.pcid, p.connDetails)
      p.natsConn = nil
    }
  }()

  var js jetstream.JetStream
  if js, err = jetstream.New(p.natsConn); err != nil {
    return err
  }

  if p.kv, err = js.KeyValue(ctx, p.bucket); err != nil {
    return err
  }
  return nil
}

func (p *kvCache) Get(ctx context.Context, key string) ([]byte, error) {
  p.connMut.RLock()
  defer p.connMut.RUnlock()

  entry, err := p.kv.Get(ctx, key)
  if err != nil {
    if errors.Is(err, jetstream.ErrKeyNotFound) {
      err = service.ErrKeyNotFound
    }
    return nil, err
  }
  return entry.Value(), nil
}

func (p *kvCache) Set(ctx context.Context, key string, value []byte, _ *time.Duration) error {
  p.connMut.RLock()
  defer p.connMut.RUnlock()

  _, err := p.kv.Put(ctx, key, value)
  return err
}

func (p *kvCache) Add(ctx context.Context, key string, value []byte, _ *time.Duration) error {
  p.connMut.RLock()
  defer p.connMut.RUnlock()
  _, err := p.kv.Create(ctx, key, value)
  if errors.Is(err, jetstream.ErrKeyExists) {
    return service.ErrKeyAlreadyExists
  }
  return err
}

func (p *kvCache) Delete(ctx context.Context, key string) error {
  p.connMut.RLock()
  defer p.connMut.RUnlock()
  return p.kv.Delete(ctx, key)
}

func (p *kvCache) Close(ctx context.Context) error {
  go func() {
    p.disconnect()
    p.shutSig.TriggerHasStopped()
  }()
  select {
  case <-p.shutSig.HasStoppedChan():
  case <-ctx.Done():
    return ctx.Err()
  }
  return nil
}
