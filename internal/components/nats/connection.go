package nats

import (
  "context"
  "crypto/tls"
  "github.com/redpanda-data/benthos/v4/public/service"
  "strings"

  "github.com/nats-io/nats.go"
)

func connectionNameDescription() string {
  return `### Connection Name

When monitoring and managing a production NATS system, it is often useful to
know which connection a message was send/received from. This can be achieved by
setting the connection name option when creating a NATS connection.

Benthos will automatically set the connection name based off the label of the given
NATS component, so that monitoring tools between NATS and benthos can stay in sync.
`
}

// I've split the connection fields into two, which allows us to put tls and
// auth further down the fields stack. This is literally just polish for the
// docs.
func connectionHeadFields() []*service.ConfigField {
  return []*service.ConfigField{
    service.NewStringListField("urls").
      Description("A list of URLs to connect to. If an item of the list contains commas it will be expanded into multiple URLs.").
      Example([]string{"nats://127.0.0.1:4222"}).
      Example([]string{"nats://username:password@127.0.0.1:4222"}),
  }
}

func connectionTailFields() []*service.ConfigField {
  return []*service.ConfigField{
    service.NewTLSToggledField("tls"),
    authFieldSpec(),
  }
}

type connectionDetails struct {
  label    string
  logger   *service.Logger
  tlsConf  *tls.Config
  authConf authConfig
  fs       *service.FS
  urls     string
}

func connectionDetailsFromParsed(conf *service.ParsedConfig, mgr *service.Resources) (c connectionDetails, err error) {
  c.label = mgr.Label()
  c.fs = mgr.FS()
  c.logger = mgr.Logger()

  var urlList []string
  if urlList, err = conf.FieldStringList("urls"); err != nil {
    return
  }
  c.urls = strings.Join(urlList, ",")

  var tlsEnabled bool
  if c.tlsConf, tlsEnabled, err = conf.FieldTLSToggled("tls"); err != nil {
    return
  }
  if !tlsEnabled {
    c.tlsConf = nil
  }

  if c.authConf, err = authFromParsedConfig(conf.Namespace("auth")); err != nil {
    return
  }
  return
}

func (c *connectionDetails) get(_ context.Context, extraOpts ...nats.Option) (*nats.Conn, error) {
  var opts []nats.Option
  if c.tlsConf != nil {
    opts = append(opts, nats.Secure(c.tlsConf))
  }
  opts = append(opts, nats.Name(c.label))
  opts = append(opts, errorHandlerOption(c.logger))
  opts = append(opts, authConfToOptions(c.authConf, c.fs)...)
  opts = append(opts, extraOpts...)
  return nats.Connect(c.urls, opts...)
}

func errorHandlerOption(logger *service.Logger) nats.Option {
  return nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
    if nc != nil {
      logger = logger.With("connection-status", nc.Status())
    }
    if sub != nil {
      logger = logger.With("subject", sub.Subject)
      if c, err := sub.ConsumerInfo(); err == nil {
        logger = logger.With("consumer", c.Name)
      }
    }
    logger.Errorf("nats operation failed: %v\n", err)
  })
}
