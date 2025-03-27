package core

import (
    "context"
    "crypto/tls"
    "github.com/nats-io/nats.go"
    "github.com/redpanda-data/benthos/v4/public/service"
    "github.com/wombatwisdom/wombat/docs"
    "strings"
)

const (
    ConnectionUrlField      = "url"
    ConnectionNameField     = "name"
    ConnectionTlsField      = "tls"
    ConnectionAuthField     = "auth"
    ConnectionAuthSeedField = "seed"
    ConnectionAuthJwtField  = "jwt"
)

func ConnectionDescription() (sections []*docs.Section) {
    sections = append(sections, docs.NewSection("Authentication").WithBody(`
In case your NATS server requires authentication, you can provide the necessary credentials through the auth section
of the configuration. This section gives you the ability to provide the NKey seed and JWT token for the user. Since
both of these are sensitive information, it is recommended to expose them through environment variables and not make
them explicit in the configuration.
        `))

    return
}

func ConnectionConfigFields() (commonFields []*service.ConfigField, advancedFields []*service.ConfigField) {
    commonFields = append(commonFields, service.NewStringField(ConnectionUrlField).
        Default(nats.DefaultURL).
        Examples("nats://demo.nats.io:4222", "nats://server-1:4222,nats://server-2:4222", "tls://connect.ngs.global").
        Description(strings.TrimSpace(`
The URL of the NATS server to connect to. 
Multiple URLs can be specified by separating them with commas. If an item of the list contains commas it will be expanded into multiple URLs.
`)))

    advancedFields = append(advancedFields, service.NewStringField(ConnectionNameField).
        Default("Wombat").
        Description("A name for the connection to distinguish it from others."))
    advancedFields = append(advancedFields, service.NewObjectField(ConnectionAuthField,
        service.NewStringField(ConnectionAuthSeedField).
            Description("An plain text user JWT (given along with the corresponding Seed).").
            Secret().
            Optional(),
        service.NewStringField(ConnectionAuthJwtField).
            Description("An plain text user NKey Seed (given along with the corresponding JWT).").
            Secret().
            Optional(),
    ).Description("NATS authentication parameters"))
    advancedFields = append(advancedFields, service.NewTLSToggledField(ConnectionTlsField))

    return
}

func ConnectionFromConfig(cfg *service.ParsedConfig) (*Connection, error) {
    var err error
    result := &Connection{}

    if result.Url, err = cfg.FieldString(ConnectionUrlField); err != nil {
        return nil, err
    }

    if cfg.Contains(ConnectionAuthField) {
        authCfg := cfg.Namespace(ConnectionAuthField)
        jwt, _ := authCfg.FieldString(ConnectionAuthJwtField)
        seed, _ := authCfg.FieldString(ConnectionAuthSeedField)

        if jwt != "" && seed != "" {
            result.Opts = append(result.Opts, nats.UserJWTAndSeed(jwt, seed))
        }
    }

    if cfg.Contains(ConnectionNameField) {
        name, _ := cfg.FieldString(ConnectionNameField)
        result.Opts = append(result.Opts, nats.Name(name))
    }

    var tlsConf *tls.Config
    var tlsEnabled bool
    if tlsConf, tlsEnabled, err = cfg.FieldTLSToggled("tls"); err != nil {
        return nil, err
    }
    if tlsEnabled {
        result.Opts = append(result.Opts, nats.Secure(tlsConf))
    }

    return result, nil
}

type Connection struct {
    Url  string
    Opts []nats.Option

    conn *nats.Conn
}

func (c *Connection) Connect(ctx context.Context) error {
    var err error
    c.conn, err = nats.Connect(c.Url, c.Opts...)
    return err
}

func (c *Connection) Client() *nats.Conn {
    return c.conn
}

func (c *Connection) Close(ctx context.Context) error {
    if c.conn != nil {
        c.conn.Close()
    }

    return nil
}
