---
title: aws_dynamodb
slug: aws_dynamodb
type: cache
status: stable
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Stores key/value pairs as a single document in a DynamoDB table. The key is stored as a string value and used as the table hash key. The value is stored as
a binary value using the `data_key` field name.

Introduced in version 1.0.0.


<Tabs defaultValue="common" values={[
  { label: 'Common', value: 'common', },
  { label: 'Advanced', value: 'advanced', },
]}>

<TabItem value="common">

```yml
# Common config fields, showing default values
label: ""
aws_dynamodb:
  table: "" # No default (required)
  hash_key: "" # No default (required)
  data_key: "" # No default (required)
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
label: ""
aws_dynamodb:
  table: "" # No default (required)
  hash_key: "" # No default (required)
  data_key: "" # No default (required)
  consistent_read: false
  default_ttl: "" # No default (optional)
  ttl_key: "" # No default (optional)
  retries:
    initial_interval: 1s
    max_interval: 5s
    max_elapsed_time: 30s
  region: ""
  endpoint: ""
  credentials:
    profile: ""
    id: ""
    secret: ""
    token: ""
    from_ec2_role: false
    role: ""
    role_external_id: ""
```

</TabItem>
</Tabs>

A prefix can be specified to allow multiple cache types to share a single DynamoDB table. An optional TTL duration (`ttl`) and field
(`ttl_key`) can be specified if the backing table has TTL enabled.

Strong read consistency can be enabled using the `consistent_read` configuration field.

## Fields

### `table`

The table to store items in.


Type: `string`  

### `hash_key`

The key of the table column to store item keys within.


Type: `string`  

### `data_key`

The key of the table column to store item values within.


Type: `string`  

### `consistent_read`

Whether to use strongly consistent reads on Get commands.


Type: `bool`  
Default: `false`  

### `default_ttl`

An optional default TTL to set for items, calculated from the moment the item is cached. A `ttl_key` must be specified in order to set item TTLs.


Type: `string`  

### `ttl_key`

The column key to place the TTL value within.


Type: `string`  

### `retries`

Determine time intervals and cut offs for retry attempts.


Type: `object`  

### `retries.initial_interval`

The initial period to wait between retry attempts.


Type: `string`  
Default: `"1s"`  

```yml
# Examples

initial_interval: 50ms

initial_interval: 1s
```

### `retries.max_interval`

The maximum period to wait between retry attempts


Type: `string`  
Default: `"5s"`  

```yml
# Examples

max_interval: 5s

max_interval: 1m
```

### `retries.max_elapsed_time`

The maximum overall period of time to spend on retry attempts before the request is aborted.


Type: `string`  
Default: `"30s"`  

```yml
# Examples

max_elapsed_time: 1m

max_elapsed_time: 1h
```

### `region`

The AWS region to target.


Type: `string`  
Default: `""`  

### `endpoint`

Allows you to specify a custom endpoint for the AWS API.


Type: `string`  
Default: `""`  

### `credentials`

Optional manual configuration of AWS credentials to use. More information can be found [in this document](/docs/guides/cloud/aws).


Type: `object`  

### `credentials.profile`

A profile from `~/.aws/credentials` to use.


Type: `string`  
Default: `""`  

### `credentials.id`

The ID of credentials to use.


Type: `string`  
Default: `""`  

### `credentials.secret`

The secret for the credentials being used.
:::warning Secret
This field contains sensitive information that usually shouldn't be added to a config directly, read our [secrets page for more info](/docs/configuration/secrets).
:::


Type: `string`  
Default: `""`  

### `credentials.token`

The token for the credentials being used, required when using short term credentials.


Type: `string`  
Default: `""`  

### `credentials.from_ec2_role`

Use the credentials of a host EC2 machine configured to assume [an IAM role associated with the instance](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html).


Type: `bool`  
Default: `false`  
Requires version 1.0.0 or newer  

### `credentials.role`

A role ARN to assume.


Type: `string`  
Default: `""`  

### `credentials.role_external_id`

An external ID to provide when assuming a role.


Type: `string`  
Default: `""`  

