package nats

import (
  "github.com/redpanda-data/benthos/v4/public/service"
)

const (
  kvFieldBucket = "bucket"
)

const (
  tracingVersion = "4.23.0"
)

func inputTracingDocs() *service.ConfigField {
  return service.NewExtractTracingSpanMappingField().Version(tracingVersion)
}
func outputTracingDocs() *service.ConfigField {
  return service.NewInjectTracingSpanMappingField().Version(tracingVersion)
}

func kvDocs(extraFields ...*service.ConfigField) []*service.ConfigField {
  // TODO: Use `slices.Concat()` after switching to Go 1.22
  fields := append(
    connectionHeadFields(),
    []*service.ConfigField{
      service.NewStringField(kvFieldBucket).
        Description("The name of the KV bucket.").Example("my_kv_bucket"),
    }...,
  )
  fields = append(fields, extraFields...)
  fields = append(fields, connectionTailFields()...)

  return fields
}
