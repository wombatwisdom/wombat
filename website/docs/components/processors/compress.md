---
title: compress
slug: compress
type: processor
status: stable
categories: ["Parsing"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Compresses messages according to the selected algorithm. Supported compression algorithms are: [flate gzip lz4 pgzip snappy zlib]

```yml
# Config fields, showing default values
label: ""
compress:
  algorithm: "" # No default (required)
  level: -1
```

The 'level' field might not apply to all algorithms.

## Fields

### `algorithm`

The compression algorithm to use.


Type: `string`  
Options: `flate`, `gzip`, `lz4`, `pgzip`, `snappy`, `zlib`.

### `level`

The level of compression to use. May not be applicable to all algorithms.


Type: `int`  
Default: `-1`  

