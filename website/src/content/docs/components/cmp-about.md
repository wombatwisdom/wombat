---
title: Components
---

A good ninja gets clued up on its gear.

## Core Components

Every Wombat pipeline has at least one [input](cmp-inputs.md), an optional [buffer](cmp-buffers.md), an [output](cmp-outputs.md) and any
number of [processors](cmp-processors.md):

```yaml
input:
  kafka:
    addresses: [ TODO ]
    topics: [ foo, bar ]
    consumer_group: foogroup

buffer:
  type: none

pipeline:
  processors:
    - mapping: |
        message = this
        meta.link_count = links.length()

output:
  aws_s3:
    bucket: TODO
    path: '${! metadata("kafka_topic") }/${! json("message.id") }.json'
```

These are the main components within Wombat and they provide the majority of useful behaviour.

## Observability Components

There are also the observability components [http](cmp-http.md), [logger](cmp-logger.md), [metrics](cmp-metrics.md), and [tracing](cmp-tracers.md),
which allow you to specify how Wombat exposes observability data:

```yaml
http:
  address: 0.0.0.0:4195
  enabled: true
  debug_endpoints: false

logger:
  format: json
  level: WARN

metrics:
  statsd:
    address: localhost:8125
    flush_period: 100ms

tracer:
  jaeger:
    agent_address: localhost:6831
```

## Resource Components

Finally, there are [caches](cmp-caches.md) and [rate limits](cmp-rate_limits.md). These are components that are referenced by core
components and can be shared.

```yaml
input:
  http_client: # This is an input
    url: TODO
    rate_limit: foo_ratelimit # This is a reference to a rate limit

pipeline:
  processors:
    - cache: # This is a processor
        resource: baz_cache # This is a reference to a cache
        operator: add
        key: '${! json("id") }'
        value: "x"
    - mapping: root = if errored() { deleted() }

rate_limit_resources:
  - label: foo_ratelimit
    local:
      count: 500
      interval: 1s

cache_resources:
  - label: baz_cache
    memcached:
      addresses: [ localhost:11211 ]
```

It's also possible to configure inputs, outputs and processors as resources which allows them to be reused throughout a
configuration with the `resource` input, `resource` output and `resource` processor respectively.

For more information about any of these component types check out their sections:

- [inputs](cmp-inputs.md)
- [processors](cmp-processors.md)
- [outputs](cmp-outputs.md)
- [buffers](cmp-buffers.md)
- [metrics](cmp-metrics.md)
- [tracers](cmp-tracers.md)
- [logger](cmp-logger.md)
- [caches](cmp-caches.md)
- [rate limits](cmp-rate_limits.md)