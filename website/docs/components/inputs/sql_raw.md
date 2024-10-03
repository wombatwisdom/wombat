---
title: sql_raw
slug: sql_raw
type: input
status: beta
categories: ["Services"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the corresponding source file under internal/impl/<provider>.
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

:::caution BETA
This component is mostly stable but breaking changes could still be made outside of major version releases if a fundamental problem with the component is found.
:::
Executes a select query and creates a message for each row received.

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
  sql_raw:
    driver: "" # No default (required)
    dsn: clickhouse://username:password@host1:9000,host2:9000/database?dial_timeout=200ms&max_execution_time=60 # No default (required)
    query: SELECT * FROM footable WHERE user_id = $1; # No default (required)
    args_mapping: root = [ this.cat.meow, this.doc.woofs[0] ] # No default (optional)
    auto_replay_nacks: true
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
input:
  label: ""
  sql_raw:
    driver: "" # No default (required)
    dsn: clickhouse://username:password@host1:9000,host2:9000/database?dial_timeout=200ms&max_execution_time=60 # No default (required)
    query: SELECT * FROM footable WHERE user_id = $1; # No default (required)
    args_mapping: root = [ this.cat.meow, this.doc.woofs[0] ] # No default (optional)
    auto_replay_nacks: true
    init_files: [] # No default (optional)
    init_statement: | # No default (optional)
      CREATE TABLE IF NOT EXISTS some_table (
        foo varchar(50) not null,
        bar integer,
        baz varchar(50),
        primary key (foo)
      ) WITHOUT ROWID;
    init_verify_conn: false
    conn_max_idle_time: "" # No default (optional)
    conn_max_life_time: "" # No default (optional)
    conn_max_idle: 2
    conn_max_open: 0 # No default (optional)
```

</TabItem>
</Tabs>

Once the rows from the query are exhausted this input shuts down, allowing the pipeline to gracefully terminate (or the next input in a [sequence](/docs/components/inputs/sequence) to execute).

## Examples

<Tabs defaultValue="Consumes an SQL table using a query as an input." values={[
{ label: 'Consumes an SQL table using a query as an input.', value: 'Consumes an SQL table using a query as an input.', },
]}>

<TabItem value="Consumes an SQL table using a query as an input.">


Here we preform an aggregate over a list of names in a table that are less than 3600 seconds old.

```yaml
input:
  sql_raw:
    driver: postgres
    dsn: postgres://foouser:foopass@localhost:5432/testdb?sslmode=disable
    query: "SELECT name, count(*) FROM person WHERE last_updated < $1 GROUP BY name;"
    args_mapping: |
      root = [
        now().ts_unix() - 3600
      ]
```

</TabItem>
</Tabs>

## Fields

### `driver`

A database [driver](#drivers) to use.


Type: `string`  
Options: `mysql`, `postgres`, `clickhouse`, `mssql`, `sqlite`, `oracle`, `snowflake`, `trino`, `gocosmos`.

### `dsn`

A Data Source Name to identify the target database.

#### Drivers

The following is a list of supported drivers, their placeholder style, and their respective DSN formats:

| Driver | Data Source Name Format |
|---|---|
| `clickhouse` | [`clickhouse://[username[:password]@][netloc][:port]/dbname[?param1=value1&...&paramN=valueN]`](https://github.com/ClickHouse/clickhouse-go#dsn) |
| `mysql` | `[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]` |
| `postgres` | `postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]` |
| `mssql` | `sqlserver://[user[:password]@][netloc][:port][?database=dbname&param1=value1&...]` |
| `sqlite` | `file:/path/to/filename.db[?param&=value1&...]` |
| `oracle` | `oracle://[username[:password]@][netloc][:port]/service_name?server=server2&server=server3` |
| `snowflake` | `username[:password]@account_identifier/dbname/schemaname[?param1=value&...&paramN=valueN]` |
| `trino` | [`http[s]://user[:pass]@host[:port][?parameters]`](https://github.com/trinodb/trino-go-client#dsn-data-source-name) |
| `gocosmos` | [`AccountEndpoint=<cosmosdb-endpoint>;AccountKey=<cosmosdb-account-key>[;TimeoutMs=<timeout-in-ms>][;Version=<cosmosdb-api-version>][;DefaultDb/Db=<db-name>][;AutoId=<true/false>][;InsecureSkipVerify=<true/false>]`](https://pkg.go.dev/github.com/microsoft/gocosmos#readme-example-usage) |

Please note that the `postgres` driver enforces SSL by default, you can override this with the parameter `sslmode=disable` if required.

The `snowflake` driver supports multiple DSN formats. Please consult [the docs](https://pkg.go.dev/github.com/snowflakedb/gosnowflake#hdr-Connection_String) for more details. For [key pair authentication](https://docs.snowflake.com/en/user-guide/key-pair-auth.html#configuring-key-pair-authentication), the DSN has the following format: `<snowflake_user>@<snowflake_account>/<db_name>/<schema_name>?warehouse=<warehouse>&role=<role>&authenticator=snowflake_jwt&privateKey=<base64_url_encoded_private_key>`, where the value for the `privateKey` parameter can be constructed from an unencrypted RSA private key file `rsa_key.p8` using `openssl enc -d -base64 -in rsa_key.p8 | basenc --base64url -w0` (you can use `gbasenc` insted of `basenc` on OSX if you install `coreutils` via Homebrew). If you have a password-encrypted private key, you can decrypt it using `openssl pkcs8 -in rsa_key_encrypted.p8 -out rsa_key.p8`. Also, make sure fields such as the username are URL-encoded.

The [`gocosmos`](https://pkg.go.dev/github.com/microsoft/gocosmos) driver is still experimental, but it has support for [hierarchical partition keys](https://learn.microsoft.com/en-us/azure/cosmos-db/hierarchical-partition-keys) as well as [cross-partition queries](https://learn.microsoft.com/en-us/azure/cosmos-db/nosql/how-to-query-container#cross-partition-query). Please refer to the [SQL notes](https://github.com/microsoft/gocosmos/blob/main/SQL.md) for details.


Type: `string`  

```yml
# Examples

dsn: clickhouse://username:password@host1:9000,host2:9000/database?dial_timeout=200ms&max_execution_time=60

dsn: foouser:foopassword@tcp(localhost:3306)/foodb

dsn: postgres://foouser:foopass@localhost:5432/foodb?sslmode=disable

dsn: oracle://foouser:foopass@localhost:1521/service_name
```

### `query`

The query to execute. The style of placeholder to use depends on the driver, some drivers require question marks (`?`) whereas others expect incrementing dollar signs (`$1`, `$2`, and so on) or colons (`:1`, `:2` and so on). The style to use is outlined in this table:

| Driver | Placeholder Style |
|---|---|
| `clickhouse` | Dollar sign |
| `mysql` | Question mark |
| `postgres` | Dollar sign |
| `mssql` | Question mark |
| `sqlite` | Question mark |
| `oracle` | Colon |
| `snowflake` | Question mark |
| `trino` | Question mark |
| `gocosmos` | Colon |


Type: `string`  

```yml
# Examples

query: SELECT * FROM footable WHERE user_id = $1;
```

### `args_mapping`

A [Bloblang mapping](/docs/guides/bloblang/about) which should evaluate to an array of values matching in size to the number of columns specified.


Type: `string`  

```yml
# Examples

args_mapping: root = [ this.cat.meow, this.doc.woofs[0] ]

args_mapping: root = [ meta("user.id") ]
```

### `auto_replay_nacks`

Whether messages that are rejected (nacked) at the output level should be automatically replayed indefinitely, eventually resulting in back pressure if the cause of the rejections is persistent. If set to `false` these messages will instead be deleted. Disabling auto replays can greatly improve memory efficiency of high throughput streams as the original shape of the data can be discarded immediately upon consumption and mutation.


Type: `bool`  
Default: `true`  

### `init_files`

An optional list of file paths containing SQL statements to execute immediately upon the first connection to the target database. This is a useful way to initialise tables before processing data. Glob patterns are supported, including super globs (double star).

Care should be taken to ensure that the statements are idempotent, and therefore would not cause issues when run multiple times after service restarts. If both `init_statement` and `init_files` are specified the `init_statement` is executed _after_ the `init_files`.

If a statement fails for any reason a warning log will be emitted but the operation of this component will not be stopped.


Type: `array`  
Requires version 1.0.0 or newer  

```yml
# Examples

init_files:
  - ./init/*.sql

init_files:
  - ./foo.sql
  - ./bar.sql
```

### `init_statement`

An optional SQL statement to execute immediately upon the first connection to the target database. This is a useful way to initialise tables before processing data. Care should be taken to ensure that the statement is idempotent, and therefore would not cause issues when run multiple times after service restarts.

If both `init_statement` and `init_files` are specified the `init_statement` is executed _after_ the `init_files`.

If the statement fails for any reason a warning log will be emitted but the operation of this component will not be stopped.


Type: `string`  
Requires version 1.0.0 or newer  

```yml
# Examples

init_statement: |2
  CREATE TABLE IF NOT EXISTS some_table (
    foo varchar(50) not null,
    bar integer,
    baz varchar(50),
    primary key (foo)
  ) WITHOUT ROWID;
```

### `init_verify_conn`

Whether to verify the database connection on startup by performing a simple ping, by default this is disabled.


Type: `bool`  
Default: `false`  
Requires version 1.2.0 or newer  

### `conn_max_idle_time`

An optional maximum amount of time a connection may be idle. Expired connections may be closed lazily before reuse. If `value <= 0`, connections are not closed due to a connections idle time.


Type: `string`  

### `conn_max_life_time`

An optional maximum amount of time a connection may be reused. Expired connections may be closed lazily before reuse. If `value <= 0`, connections are not closed due to a connections age.


Type: `string`  

### `conn_max_idle`

An optional maximum number of connections in the idle connection pool. If conn_max_open is greater than 0 but less than the new conn_max_idle, then the new conn_max_idle will be reduced to match the conn_max_open limit. If `value <= 0`, no idle connections are retained. The default max idle connections is currently 2. This may change in a future release.


Type: `int`  
Default: `2`  

### `conn_max_open`

An optional maximum number of open connections to the database. If conn_max_idle is greater than 0 and the new conn_max_open is less than conn_max_idle, then conn_max_idle will be reduced to match the new conn_max_open limit. If `value <= 0`, then there is no limit on the number of open connections. The default is 0 (unlimited).


Type: `int`  

