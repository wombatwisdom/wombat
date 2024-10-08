---
title: Inputs
---

An input is a source of data piped through an array of optional [processors](cmp-processors.md):

```yaml
input:
  label: my_redis_input

  redis_streams:
    url: tcp://localhost:6379
    streams:
      - wombat_stream
    body_key: body
    consumer_group: wombat_group

  # Optional list of processing steps
  processors:
    - mapping: |
        root.document = this.without("links")
        root.link_count = this.links.length()
```

Some inputs have a logical end, for example a `csv` input ends once the last row is consumed, when this
happens the input gracefully terminates and Wombat will shut itself down once all messages have been processed fully.

It's also possible to specify a logical end for an input that otherwise doesn't have one with the
`read_until` input, which checks a condition against each consumed message in order to determine
whether it should be the last.

## Brokering

Only one input is configured at the root of a Wombat config. However, the root input can be a `broker`
which combines multiple inputs and merges the streams:

```yaml
input:
  broker:
    inputs:
      - kafka:
          addresses: [ TODO ]
          topics: [ foo, bar ]
          consumer_group: foogroup

      - redis_streams:
          url: tcp://localhost:6379
          streams:
            - wombat_stream
          body_key: body
          consumer_group: wombat_group
```

## Labels

Inputs have an optional field `label` that can uniquely identify them in observability data such as metrics and logs.
This can be useful when running configs with multiple inputs, otherwise their metrics labels will be generated based on
their composition. For more information check out the [metrics documentation](cmp-metrics.md).

### Sequential Reads

Sometimes it's useful to consume a sequence of inputs, where an input is only consumed once its predecessor is drained
fully, you can achieve this with the `sequence` input.

## Generating Messages

It's possible to generate data with Wombat using the `generate` input, which is also a convenient way
to trigger scheduled pipelines.