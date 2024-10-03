---
title: nanomsg
slug: nanomsg
type: input
status: stable
categories: ["Network"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Consumes messages via Nanomsg sockets (scalability protocols).


<Tabs defaultValue="common" values={[
  { label: 'Common', value: 'common', },
  { label: 'Advanced', value: 'advanced', },
]}>

<TabItem value="common">

```yml
# Common config fields, showing default values
input:
  label: ""
  nanomsg:
    urls: [] # No default (required)
    bind: true
    socket_type: PULL
    auto_replay_nacks: true
    sub_filters: []
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
input:
  label: ""
  nanomsg:
    urls: [] # No default (required)
    bind: true
    socket_type: PULL
    auto_replay_nacks: true
    sub_filters: []
    poll_timeout: 5s
```

</TabItem>
</Tabs>

Currently only PULL and SUB sockets are supported.

## Fields

### `urls`

A list of URLs to connect to (or as). If an item of the list contains commas it will be expanded into multiple URLs.


Type: `array`  

### `bind`

Whether the URLs provided should be connected to, or bound as.


Type: `bool`  
Default: `true`  

### `socket_type`

The socket type to use.


Type: `string`  
Default: `"PULL"`  
Options: `PULL`, `SUB`.

### `auto_replay_nacks`

Whether messages that are rejected (nacked) at the output level should be automatically replayed indefinitely, eventually resulting in back pressure if the cause of the rejections is persistent. If set to `false` these messages will instead be deleted. Disabling auto replays can greatly improve memory efficiency of high throughput streams as the original shape of the data can be discarded immediately upon consumption and mutation.


Type: `bool`  
Default: `true`  

### `sub_filters`

A list of subscription topic filters to use when consuming from a SUB socket. Specifying a single sub_filter of `''` will subscribe to everything.


Type: `array`  
Default: `[]`  

### `poll_timeout`

The period to wait until a poll is abandoned and reattempted.


Type: `string`  
Default: `"5s"`  

