---
title: batched
slug: batched
type: input
status: stable
categories: ["Utility"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Consumes data from a child input and applies a batching policy to the stream.

Introduced in version 1.0.0.


<Tabs defaultValue="common" values={[
  { label: 'Common', value: 'common', },
  { label: 'Advanced', value: 'advanced', },
]}>

<TabItem value="common">

```yml
# Common config fields, showing default values
input:
  label: ""
  batched:
    child: null # No default (required)
    policy:
      count: 0
      byte_size: 0
      period: ""
      check: ""
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
input:
  label: ""
  batched:
    child: null # No default (required)
    policy:
      count: 0
      byte_size: 0
      period: ""
      check: ""
      processors: [] # No default (optional)
```

</TabItem>
</Tabs>

Batching at the input level is sometimes useful for processing across micro-batches, and can also sometimes be a useful performance trick. However, most inputs are fine without it so unless you have a specific plan for batching this component is not worth using.

## Fields

### `child`

The child input.


Type: `input`  

### `policy`

Allows you to configure a [batching policy](/docs/configuration/batching).


Type: `object`  

```yml
# Examples

policy:
  byte_size: 5000
  count: 0
  period: 1s

policy:
  count: 10
  period: 1s

policy:
  check: this.contains("END BATCH")
  count: 0
  period: 1m
```

### `policy.count`

A number of messages at which the batch should be flushed. If `0` disables count based batching.


Type: `int`  
Default: `0`  

### `policy.byte_size`

An amount of bytes at which the batch should be flushed. If `0` disables size based batching.


Type: `int`  
Default: `0`  

### `policy.period`

A period in which an incomplete batch should be flushed regardless of its size.


Type: `string`  
Default: `""`  

```yml
# Examples

period: 1s

period: 1m

period: 500ms
```

### `policy.check`

A [Bloblang query](/docs/guides/bloblang/about/) that should return a boolean value indicating whether a message should end a batch.


Type: `string`  
Default: `""`  

```yml
# Examples

check: this.type == "end_of_transaction"
```

### `policy.processors`

A list of [processors](/docs/components/processors/about) to apply to a batch as it is flushed. This allows you to aggregate and archive the batch however you see fit. Please note that all resulting messages are flushed as a single batch, therefore splitting the batch into smaller batches using these processors is a no-op.


Type: `array`  

```yml
# Examples

processors:
  - archive:
      format: concatenate

processors:
  - archive:
      format: lines

processors:
  - archive:
      format: json_array
```

