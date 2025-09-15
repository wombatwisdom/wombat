package change_stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/service"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	mdb "go.mongodb.org/mongo-driver/mongo"

	"github.com/wombatwisdom/wombat/public/components/mongodb"
)

const (
	IdHeader = "mongodb_change_stream_id"
)

type ChangeStreamReaderOptions struct {
	mongodb.Config

	Database   string `json:"database"`
	Collection string `json:"collection"`
}

func (o ChangeStreamReaderOptions) ChangeStream(ctx context.Context, c *mdb.Client) (*mdb.ChangeStream, error) {
	if o.Database != "" {
		db := c.Database(o.Database)
		if db == nil {
			return nil, fmt.Errorf("database not found: %s", o.Database)
		}

		if o.Collection != "" {
			// -- watch collection
			col := db.Collection(o.Collection)
			if col == nil {
				return nil, fmt.Errorf("collection not found: %s", o.Collection)
			}
			return col.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
		} else {
			// -- watch database
			return db.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
		}
	} else {
		// -- watch client
		return c.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	}
}

type ChangeStreamReader struct {
	cfg ChangeStreamReaderOptions

	c  *mdb.Client
	cs *mdb.ChangeStream
}

func NewChangeStreamReader(options ChangeStreamReaderOptions) *ChangeStreamReader {
	return &ChangeStreamReader{
		cfg: options,
	}
}

func (r *ChangeStreamReader) Connect(ctx context.Context) error {
	var err error

	r.c, err = r.cfg.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	r.cs, err = r.cfg.ChangeStream(ctx, r.c)
	if err != nil {
		return fmt.Errorf("failed to create change stream: %w", err)
	}

	return nil
}

func (r *ChangeStreamReader) Close(ctx context.Context) error {
	var err error
	if r.cs != nil {
		err = errors.Join(r.cs.Close(ctx))
		r.cs = nil
	}

	if r.c != nil {
		err = errors.Join(r.c.Disconnect(ctx))
		r.c = nil
	}

	return err
}

func (r *ChangeStreamReader) Read(ctx context.Context) (*service.Message, error) {
	if r.cs == nil {
		return nil, service.ErrNotConnected
	}

	avail := r.cs.Next(ctx)
	if !avail {
		return nil, service.ErrEndOfInput
	}

	// -- read next change
	b, err := bson.MarshalExtJSON(r.cs.Current, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal change: %w", err)
	}

	msg := service.NewMessage(b)
	msg.MetaSet(IdHeader, fmt.Sprintf("%d", r.cs.ID()))

	return msg, nil
}
