---
title: gcp_pubsub
slug: gcp_pubsub
type: input
status: stable
categories: ["Services","GCP"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Consumes messages from a GCP Cloud Pub/Sub subscription.


<Tabs defaultValue="common" values={[
  { label: 'Common', value: 'common', },
  { label: 'Advanced', value: 'advanced', },
]}>

<TabItem value="common">

```yml
# Common config fields, showing default values
input:
  label: ""
  gcp_pubsub:
    project: "" # No default (required)
    subscription: "" # No default (required)
    endpoint: ""
    sync: false
    max_outstanding_messages: 1000
    max_outstanding_bytes: 1e+09
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
input:
  label: ""
  gcp_pubsub:
    project: "" # No default (required)
    subscription: "" # No default (required)
    endpoint: ""
    sync: false
    max_outstanding_messages: 1000
    max_outstanding_bytes: 1e+09
    create_subscription:
      enabled: false
      topic: ""
```

</TabItem>
</Tabs>

For information on how to set up credentials check out [this guide](https://cloud.google.com/docs/authentication/production).

### Metadata

This input adds the following metadata fields to each message:

``` text
- gcp_pubsub_publish_time_unix - The time at which the message was published to the topic.
- gcp_pubsub_delivery_attempt - When dead lettering is enabled, this is set to the number of times PubSub has attempted to deliver a message.
- All message attributes
```

You can access these metadata fields using [function interpolation](/docs/configuration/interpolation#bloblang-queries).


## Fields

### `project`

The project ID of the target subscription.


Type: `string`  

### `subscription`

The target subscription ID.


Type: `string`  

### `endpoint`

An optional endpoint to override the default of `pubsub.googleapis.com:443`. This can be used to connect to a region specific pubsub endpoint. For a list of valid values check out [this document.](https://cloud.google.com/pubsub/docs/reference/service_apis_overview#list_of_regional_endpoints)


Type: `string`  
Default: `""`  

```yml
# Examples

endpoint: us-central1-pubsub.googleapis.com:443

endpoint: us-west3-pubsub.googleapis.com:443
```

### `sync`

Enable synchronous pull mode.


Type: `bool`  
Default: `false`  

### `max_outstanding_messages`

The maximum number of outstanding pending messages to be consumed at a given time.


Type: `int`  
Default: `1000`  

### `max_outstanding_bytes`

The maximum number of outstanding pending messages to be consumed measured in bytes.


Type: `int`  
Default: `1000000000`  

### `create_subscription`

Allows you to configure the input subscription and creates if it doesn't exist.


Type: `object`  

### `create_subscription.enabled`

Whether to configure subscription or not.


Type: `bool`  
Default: `false`  

### `create_subscription.topic`

Defines the topic that the subscription should be vinculated to.


Type: `string`  
Default: `""`  

