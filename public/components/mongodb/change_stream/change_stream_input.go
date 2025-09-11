package change_stream

import (
	"context"

	"github.com/redpanda-data/benthos/v4/public/service"

	"github.com/wombatwisdom/wombat/public/components/mongodb"
)

// ChangeStreamReaderInterface defines the interface for change stream readers
type ChangeStreamReaderInterface interface {
	Connect(ctx context.Context) error
	Read(ctx context.Context) (*service.Message, error)
	Close(ctx context.Context) error
}

func inputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Consume the MongoDB ChangeStream.").
		Description(`This input is capable of reading change events from a MongoDB client, database, or collection.
When a database as well as a collection is provided, only changes to that collection will be read.
When only a database is provided, changes to all collections in that database will be read.
When neither a database nor a collection is provided, changes to all databases and collections will be read.
`).
		Fields(mongodb.Fields...).
		Field(service.NewStringField("database").Description("The database to watch.").Optional().Default("")).
		Field(service.NewStringField("collection").Description("The collection to watch.").Optional().Default("")).
		Field(service.NewAutoRetryNacksToggleField())
}

func init() {
	err := service.RegisterInput(
		"mongodb_change_stream", inputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			input, err := newChangeStreamInput(conf, mgr)
			if err != nil {
				return nil, err
			}

			return service.AutoRetryNacksToggled(conf, input)
		})
	if err != nil {
		panic(err)
	}
}

// NewChangeStreamInput creates a new change stream input from configuration
func NewChangeStreamInput(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	return newChangeStreamInput(conf, mgr)
}

func newChangeStreamInput(conf *service.ParsedConfig, mgr *service.Resources) (*changeStreamInput, error) {
	mcfg, err := mongodb.ConfigFromParsed(conf)
	if err != nil {
		return nil, err
	}

	opts := ChangeStreamReaderOptions{
		Config: *mcfg,
	}

	opts.Database, err = conf.FieldString("database")
	if err != nil {
		return nil, err
	}

	opts.Collection, err = conf.FieldString("collection")
	if err != nil {
		return nil, err
	}

	return &changeStreamInput{
		reader: NewChangeStreamReader(opts),
	}, nil
}

type changeStreamInput struct {
	reader ChangeStreamReaderInterface
}

func (c *changeStreamInput) Connect(ctx context.Context) error {
	return c.reader.Connect(ctx)
}

func (c *changeStreamInput) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	if c.reader == nil {
		return nil, nil, service.ErrNotConnected
	}

	msg, err := c.reader.Read(ctx)
	if err != nil {
		return nil, nil, err
	}

	return msg, func(ctx context.Context, err error) error {
		return err
	}, nil
}

func (c *changeStreamInput) Close(ctx context.Context) error {
	return c.reader.Close(ctx)
}
