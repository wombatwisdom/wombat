package gcp_bigtable

import (
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/bigtable"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/redpanda-data/benthos/v4/public/service"
	"google.golang.org/api/option"
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
		service.NewStringField("row_key_path").Description("The path to the field in the message containing the row key."),
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

	rk, err := conf.FieldString("row_key_path")
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
		rk:              rk,
	}, nil
}

type GCPBigTableOutput struct {
	project         string
	instance        string
	credentialsJSON string
	table           string
	rk              string

	c *bigtable.Client
	t *bigtable.Table
}

func (g *GCPBigTableOutput) Connect(ctx context.Context) error {
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
	g.t = g.c.Open(g.table)
	return nil
}

func (g *GCPBigTableOutput) WriteBatch(ctx context.Context, batch service.MessageBatch) error {
	rks, err := asRowKeys(batch, g.rk)
	if err != nil {
		return fmt.Errorf("failed to extract row keys: %w", err)
	}

	muts, err := asMutations(batch)
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

func asRowKeys(batch service.MessageBatch, keyPath string) ([]string, error) {
	var result []string
	for _, msg := range batch {
		s, err := expr.TryString(msg)
		if err != nil {
			return nil, err
		}

		result = append(result, s)
	}

	return result, nil
}

func asMutations(batch service.MessageBatch) ([]*bigtable.Mutation, error) {
	var result []*bigtable.Mutation

	for _, msg := range batch {
		mut, err := asMutation(msg)
		if err != nil {
			return nil, err
		}

		if mut != nil {
			result = append(result, mut)
		}
	}

	return result, nil
}

func asMutation(msg *service.Message) (*bigtable.Mutation, error) {
	m, err := msg.AsStructured()
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
