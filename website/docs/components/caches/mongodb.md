---
title: mongodb
slug: mongodb
type: cache
status: experimental
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

:::caution EXPERIMENTAL
This component is experimental and therefore subject to change or removal outside of major version releases.
:::
Use a MongoDB instance as a cache.

Introduced in version 1.0.0.

```yml
# Config fields, showing default values
label: ""
mongodb:
  url: mongodb://localhost:27017 # No default (required)
  database: "" # No default (required)
  username: ""
  password: ""
  collection: "" # No default (required)
  key_field: "" # No default (required)
  value_field: "" # No default (required)
```

## Fields

### `url`

The URL of the target MongoDB server.


Type: `string`  

```yml
# Examples

url: mongodb://localhost:27017
```

### `database`

The name of the target MongoDB database.


Type: `string`  

### `username`

The username to connect to the database.


Type: `string`  
Default: `""`  

### `password`

The password to connect to the database.
:::warning Secret
This field contains sensitive information that usually shouldn't be added to a config directly, read our [secrets page for more info](/docs/configuration/secrets).
:::


Type: `string`  
Default: `""`  

### `collection`

The name of the target collection.


Type: `string`  

### `key_field`

The field in the document that is used as the key.


Type: `string`  

### `value_field`

The field in the document that is used as the value.


Type: `string`  

