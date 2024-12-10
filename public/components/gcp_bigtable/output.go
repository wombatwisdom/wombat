package gcp_bigtable

import (
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/bigtable"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	err := service.RegisterBatchOutput(
		"gcp_bigtable", GCPBigTableConfig,
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchOutput, service.BatchPolicy, int, error) {
			bPol, err := conf.FieldBatchPolicy("batching")
			if err != nil {
				return nil, service.BatchPolicy{}, 0, err
			}

			maxInFlight, err := conf.FieldInt("max_in_flight")
			if err != nil {
				return nil, service.BatchPolicy{}, 0, err
			}
			w, err := NewGCPBigTableOutput(conf, mgr)
			if err != nil {
				return nil, service.BatchPolicy{}, 0, err
			}
			spanOutput, err := conf.WrapBatchOutputExtractTracingSpanMapping("gcp_bigtable", w)
			return spanOutput, bPol, maxInFlight, err
		})
	if err != nil {
		panic(err)
	}
}

var GCPBigTableConfig = service.NewConfigSpec().
	Summary("Write data to a Google Cloud Bigtable table.").
	Description(`

`).
	Beta().
	Categories("Google Cloud").
	Fields(
		service.NewOutputMaxInFlightField().Default(1024),
		service.NewBatchPolicyField("batching"),
	).
	Fields(
		service.NewStringField("project").Description("The Google Cloud project to write to."),
		service.NewStringField("instance").Description("The Bigtable instance to write to."),
		service.NewStringField("credentials_json").
			Description("The json credentials used for connecting to the Bigtable instance.").Optional(),
	).
	Fields(
		service.NewStringField("table").Description("The Bigtable table to write to."),
		service.NewStringField("key").Description("The expression that results in the row key.").Default("this.key"),
		service.NewStringField("data").Description("The expression that results in the row data.").Default("this.without(\"key\")"),
		service.NewStringField("emulated_host_port").Advanced().Description("Connect to an emulated Bigtable instance.").Default(""),
	)

func NewGCPBigTableOutput(conf *service.ParsedConfig, mgr *service.Resources) (*GCPBigTableOutput, error) {
	project, err := conf.FieldString("project")
	if err != nil {
		return nil, err
	}

	instance, err := conf.FieldString("instance")
	if err != nil {
		return nil, err
	}

	table, err := conf.FieldString("table")
	if err != nil {
		return nil, err
	}

	rkp, err := conf.FieldString("key")
	if err != nil {
		return nil, err
	}

	rke, err := bloblang.Parse(rkp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse row key bloblang query: %w", err)
	}

	rdp, err := conf.FieldString("data")
	if err != nil {
		return nil, err
	}

	rde, err := bloblang.Parse(rdp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse row data bloblang query: %w", err)
	}

	emulated, err := conf.FieldString("emulated_host_port")
	if err != nil {
		return nil, err
	}

	credentialsJSON := ""
	if conf.Contains("credentials") {
		credentialsJSON, err = conf.FieldString("credentials_json")
		if err != nil {
			return nil, err
		}
	}

	return &GCPBigTableOutput{
		project:         project,
		instance:        instance,
		credentialsJSON: credentialsJSON,
		table:           table,
		rke:             rke,
		rde:             rde,
		emulated:        emulated,
	}, nil
}

type GCPBigTableOutput struct {
	project         string
	instance        string
	credentialsJSON string
	table           string
	rke             *bloblang.Executor
	rde             *bloblang.Executor
	emulated        string

	c *bigtable.Client
	t *bigtable.Table
}

func (g *GCPBigTableOutput) Connect(ctx context.Context) error {
	if g.emulated != "" {
		conn, err := grpc.NewClient(
			g.emulated,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}

		g.c, err = bigtable.NewClient(context.Background(),
			"fake-project", "fake-instance", option.WithGRPCConn(conn))
		if err != nil {
			return err
		}
	} else {
		opts := &credentials.DetectOptions{
			Scopes: []string{
				bigtable.Scope,
			},
		}

		if g.credentialsJSON != "" {
			opts.CredentialsJSON = []byte(g.credentialsJSON)
		}

		creds, err := credentials.DetectDefault(opts)
		if err != nil {
			return err
		}

		client, err := bigtable.NewClient(ctx, g.project, g.instance, option.WithAuthCredentials(creds))
		if err != nil {
			return err
		}

		g.c = client
	}

	g.t = g.c.Open(g.table)
	return nil
}

func (g *GCPBigTableOutput) WriteBatch(ctx context.Context, batch service.MessageBatch) error {
	rks, err := asRowKeys(batch, g.rke)
	if err != nil {
		return fmt.Errorf("failed to extract row keys: %w", err)
	}

	muts, err := asMutations(batch, g.rde)
	if err != nil {
		return fmt.Errorf("failed to extract mutations: %w", err)
	}

	errs, err := g.t.ApplyBulk(ctx, rks, muts)
	if err != nil {
		return err
	}

	if len(errs) > 0 {
		return &multierror.Error{Errors: errs}
	}

	return nil
}

func (g *GCPBigTableOutput) Close(ctx context.Context) error {
	if g.c != nil {
		return g.c.Close()
	}

	return nil
}

func asRowKeys(batch service.MessageBatch, rke *bloblang.Executor) ([]string, error) {
	var result []string
	for _, msg := range batch {
		st, err := msg.AsStructured()
		if err != nil {
			return nil, err
		}

		s, err := rke.Query(st)
		if err != nil {
			return nil, err
		}

		if s == nil {
			continue
		}

		ss, ok := s.(string)
		if !ok {
			return nil, fmt.Errorf("failed to extract row key: expected string but got %T", s)
		}

		result = append(result, ss)
	}

	return result, nil
}

func asMutations(batch service.MessageBatch, rde *bloblang.Executor) ([]*bigtable.Mutation, error) {
	var result []*bigtable.Mutation

	for _, msg := range batch {
		mut, err := asMutation(msg, rde)
		if err != nil {
			return nil, err
		}

		if mut != nil {
			result = append(result, mut)
		}
	}

	return result, nil
}

func asMutation(msg *service.Message, rde *bloblang.Executor) (*bigtable.Mutation, error) {
	st, err := msg.AsStructured()
	if err != nil {
		return nil, err
	}

	m, err := rde.Query(st)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, nil
	}

	data, ok := m.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid message format: expected the message root to be map[string]interface{} but got %T", m)
	}

	mut := bigtable.NewMutation()
	for fk, fv := range data {
		fvm, ok := fv.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid message format: expected family %q to be a map[string]interface{} but got %T", fk, fv)
		}

		for ck, cv := range fvm {
			b, err := json.Marshal(cv)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal value for column %q: %w", ck, err)
			}

			mut.Set(fk, ck, bigtable.Now(), b)
		}
	}

	return mut, nil
}
