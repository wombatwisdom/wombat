package mongodb

import (
	"context"
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/service"
	mdb "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Fields = []*service.ConfigField{
	service.NewStringField("uri").Description("MongoDB connection URI"),
}

func ConfigFromParsed(conf *service.ParsedConfig) (*Config, error) {
	uri, err := conf.FieldString("uri")
	if err != nil {
		return nil, err
	}

	return &Config{
		Uri: uri,
	}, nil
}

type Config struct {
	Uri string `json:"uri"`
}

func (c *Config) NewClient(ctx context.Context) (*mdb.Client, error) {
	if c.Uri == "" {
		return nil, fmt.Errorf("uri is required")
	}

	opts := options.Client().ApplyURI(c.Uri)

	return mdb.Connect(ctx, opts)
}
