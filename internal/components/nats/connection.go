package nats

import (
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
    service.NewStringField("name").
      Description("An optional name to assign to the connection. If not set, will default to the label").
      Default(""),
  }
}

func connectionTailFields() []*service.ConfigField {
  return []*service.ConfigField{
    service.NewTLSToggledField("tls"),
    authFieldSpec(),
    service.NewStringField("pool_key").
      Description("The connection pool key to use. Components using the same poolKey will share their connection").
      Default("default").
      Advanced(),
  }
}

type connectionDetails struct {
  label    string
  logger   *service.Logger
  tlsConf  *tls.Config
  fs       *service.FS
  poolKey  string
  urls     string
  opts     []nats.Option
  authConf authConfig
}

func connectionDetailsFromParsed(conf *service.ParsedConfig, mgr *service.Resources, extraOpts ...nats.Option) (c connectionDetails, err error) {
  var urlList []string
  if urlList, err = conf.FieldStringList("urls"); err != nil {
    return
  }
  c.urls = strings.Join(urlList, ",")

  if c.poolKey, err = conf.FieldString("pool_key"); err != nil {
    return
  }

  var name string
  if name, err = conf.FieldString("name"); err != nil {
    return
  }
  if name == "" {
    name = mgr.Label()
  }
  c.opts = append(c.opts, nats.Name(name))

  var tlsEnabled bool
  var tlsConf *tls.Config
  if tlsConf, tlsEnabled, err = conf.FieldTLSToggled("tls"); err != nil {
    return
  }
  if tlsEnabled && tlsConf != nil {
    c.opts = append(c.opts, nats.Secure(tlsConf))
  }

  if c.authConf, err = authFromParsedConfig(conf.Namespace("auth")); err != nil {
    return
  }

  c.opts = append(c.opts, authConfToOptions(c.authConf, mgr.FS())...)
  c.opts = append(c.opts, errorHandlerOption(mgr.Logger()))
  c.opts = append(c.opts, extraOpts...)
  return
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
