---
title: Lambda
description: Deploying Wombat as an AWS Lambda function
---

The `wombat-lambda` distribution is a version of Wombat specifically tailored
for deployment as an AWS Lambda function on the `go1.x` runtime,
which runs Amazon Linux on the `x86_64` architecture.
The `wombat-lambda-al2` distribution supports the `provided.al2` runtime,
which runs Amazon Linux 2 on either the `x86_64` or `arm64` architecture.

:::info Looking for something less manual?
Rather than bundle the distribution and configs yourself,
check out [makenew/serverless-wombat], which makes quick work of deploying 
a Wombat serverless project on AWS Lambda.
For building and deploying distributions with custom plugins,
look at [makenew/wombat-plugin].
:::

It uses the same configuration format as a regular Wombat instance, which can be
provided in 1 of 2 ways:

1. Inline via the `BENTO_CONFIG` environment variable (YAML format).
2. Via the filesystem using a layer, extension, or container image. By default,
   the `wombat-lambda` distribution will look for a valid configuration file in
   the locations listed below. Alternatively, the configuration file path can be
   set explicity by passing a `BENTO_CONFIG_PATH` environment variable.
  - `./wombat.yaml`
  - `./config.yaml`
  - `/wombat.yaml`
  - `/etc/wombat/config.yaml`
  - `/etc/wombat.yaml`

Also, the `http`, `input` and `buffer` sections are ignored as the service wide
HTTP server is not used, and messages are inserted via function invocations.

If the `output` section is omitted in your config then the result of the
processing pipeline is returned back to the caller, otherwise the resulting data
is sent to the output destination.

### Running with an output

The flow of a Wombat lambda function with an output configured looks like this:

```text
                    wombat-lambda
           +------------------------------+
           |                              |
       -------> Processors ----> Output -----> Somewhere
invoke     |                              |        |
       <-------------------------------------------/
           |         <Ack/Noack>          |
           |                              |
           +------------------------------+
```

Where the call will block until the output target has confirmed receipt of the
resulting payload. When the message is successfully propagated a JSON payload is
returned of the form `{"message":"request successful"}`, otherwise an error is
returned containing the reason for the failure.

### Running without an output

The flow when an output is not configured looks like this:

```text
               wombat-lambda
           +--------------------+
           |                    |
       -------> Processors --\  |
invoke     |                 |  |
       <---------------------/  |
           |     <Result>       |
           |                    |
           +--------------------+
```

Where the function returns the result of processing directly back to the caller.
The format of the result differs depending on the number of batches and messages
of a batch that resulted from the invocation:

- Single message of a single batch: `{}` (JSON object)
- Multiple messages of a single batch: `[{},{}]` (Array of JSON objects)
- Multiple batches: `[[{},{}],[{}]]` (Array of arrays of JSON objects, batches
  of size one are a single object array in this case)

#### Processing Errors

The default behaviour of a Wombat lambda is that the handler will not return an
error unless the output fails. This means that errors that occur within your
processors will not result in the handler failing, which will instead return the
final state of the message.

The handler will fail if messages have encountered an uncaught error during execution.
However, it is also possible to configure your output to use the [`reject` output][output.reject] 
in order to trigger a handler error on processor errors:

```yaml
output:
  switch:
    retry_until_success: false
    cases:
      - check: '!errored()'
        output:
          sync_response: {}
      - output:
          reject: "processing failed due to: ${! error() }"
```

:::caution
If you are using [partial batch responses](https://docs.aws.amazon.com/lambda/latest/dg/services-sqs-errorhandling.html#services-sqs-batchfailurereporting) then 
throwing an exception will cause the entire batch to be considered a failure.
:::

### Running a combination

It's possible to configure pipelines that send messages to third party
destinations and also return a result back to the caller. This is done by
configuring an output block and including an output of the type
`sync_response`.

For example, if we wished for our lambda function to send a payload to Kafka
and also return the same payload back to the caller we could use a
[broker][output-broker]:

```yml
output:
  broker:
    pattern: fan_out
    outputs:
    - kafka:
        addresses:
        - todo:9092
        client_id: wombat_serverless
        topic: example_topic
    - sync_response: {}
```

## Upload to AWS

### go1.x on x86_64

Grab an archive labelled `wombat-lambda` from the [releases page][releases]
page and then create your function:

```sh
LAMBDA_ENV=`cat yourconfig.yaml | jq -csR {Variables:{BENTO_CONFIG:.}}`
aws lambda create-function \
  --runtime go1.x \
  --handler wombat-lambda \
  --role wombat-example-role \
  --zip-file fileb://wombat-lambda.zip \
  --environment "$LAMBDA_ENV" \
  --function-name wombat-example
```

There is also an example [SAM template][sam-template] and
[Terraform resource][tf-example] in the repo to copy from.

### provided.al2 on arm64

Grab an archive labelled `wombat-lambda-al2` for `arm64` from the [releases page][releases]
page and then create your function (AWS CLI v2 only):

```sh
LAMBDA_ENV=`cat yourconfig.yaml | jq -csR {Variables:{BENTO_CONFIG:.}}`
aws lambda create-function \
  --runtime provided.al2 \
  --architectures arm64 \
  --handler not.used.for.provided.al2.runtime \
  --role wombat-example-role \
  --zip-file fileb://wombat-lambda.zip \
  --environment "$LAMBDA_ENV" \
  --function-name wombat-example
```

There is also an example [SAM template][sam-template-al2] and
[Terraform resource][tf-example-al2] in the repo to copy from.

Note that you can also run `wombat-lambda-al2` on x86_64, just use the `amd64` zip instead.

## Invoke

```sh
aws lambda invoke \
  --function-name wombat-example \
  --payload '{"your":"document"}' \
  out.txt && cat out.txt && rm out.txt
```

## Build

You can build and archive the function yourself with:

```sh
go build github.com/wombatwisdom/wombat/cmd/serverless/wombat-lambda
zip wombat-lambda.zip wombat-lambda
```

## Local testing
A quick guide on using the [LocalStack AWS emulator](https://docs.localstack.cloud/overview/) to test your `wombat-lambda` locally.

### Installation
The quickest way to get up-and-running is using the [LocalStack Docker image](https://docs.localstack.cloud/getting-started/installation/#docker):
```sh
docker run \
  --rm -it \
  -p 127.0.0.1:4566:4566 \
  -p 127.0.0.1:4510-4559:4510-4559 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  localstack/localstack
```

### Using the AWS CLI v2

First, configure AWS CLI to connect to LocalStack:
```sh
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_DEFAULT_REGION="us-east-1"
export AWS_ENDPOINT_URL=http://localhost:4566 # Unset this to create resource in AWS
```

### Creation and Invocation Examples

With the container running and CLI configured, we can go ahead with creating and invoking our Wombat Lambda using [Lambda on LocalStack](https://docs.localstack.cloud/user-guide/aws/lambda/).

Running the example in [go1.x on x86_64](#go1x-on-x86_64) can be easily achieved by including the specified endpoint flag.

```sh
LAMBDA_ENV=`cat yourconfig.yaml | jq -csR {Variables:{BENTO_CONFIG:.}}`
aws lambda create-function \
  --runtime go1.x \
  --handler wombat-lambda \
  --role 'arn:aws:iam::000000000000:role/wombat-example-role' \
  --zip-file fileb://wombat-lambda.zip \
  --environment "$LAMBDA_ENV" \
  --function-name wombat-example
```

Invocation can be done the same way:
```sh
aws lambda invoke \                                                       
  --function-name wombat-example \
  --cli-binary-format raw-in-base64-out \
  --payload '{"your":"document"}' \
  out.txt && cat out.txt && rm out.txt
```

The LocalStack logs should then include:
```sh
localstack.request.aws     : AWS lambda.CreateFunction => 201
...
localstack.request.aws     : AWS lambda.Invoke => 200
```

[releases]: https://github.com/wombatwisdom/wombat/releases
[sam-template]: https://github.com/wombatwisdom/wombat/tree/main/resources/serverless/lambda/wombat-lambda-sam.yaml
[tf-example]: https://github.com/wombatwisdom/wombat/tree/main/resources/serverless/lambda/wombat-lambda.tf
[sam-template-al2]: https://github.com/wombatwisdom/wombat/tree/main/resources/serverless/lambda/wombat-lambda-al2-sam.yaml
[tf-example-al2]: https://github.com/wombatwisdom/wombat/tree/main/resources/serverless/lambda/wombat-lambda-al2.tf
[output-broker]: /docs/components/outputs/broker
[output.reject]: /docs/components/outputs/reject
[makenew/serverless-wombat]: https://github.com/makenew/serverless-wombat
[makenew/wombat-plugin]: https://github.com/makenew/wombat-plugin
