---
title: rate_limit
slug: rate_limit
type: processor
status: stable
categories: ["Utility"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Throttles the throughput of a pipeline according to a specified [`rate_limit`](/docs/components/rate_limits/about) resource. Rate limits are shared across components and therefore apply globally to all processing pipelines.

```yml
# Config fields, showing default values
label: ""
rate_limit:
  resource: "" # No default (required)
```

## Fields

### `resource`

The target [`rate_limit` resource](/docs/components/rate_limits/about).


Type: `string`  

