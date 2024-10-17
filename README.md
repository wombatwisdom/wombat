
<p align="center">
    <img src="logo.svg" width=50% height=50% alt="Wombat">
</p>

[![godoc for wombatwisdom/wombat][godoc-badge]][godoc-url]
[![Build Status][actions-badge]][actions-url]
[![Docs site][website-badge]][website-url]

Wombat is a high performance and resilient stream processor, able to connect various [sources][inputs] and 
[sinks][outputs] in a range of brokering patterns and perform hydration, enrichments, transformations and filters on payloads.

It comes with a [powerful mapping language][bloblang], is easy to deploy and monitor, and ready to 
drop into your pipeline either as a static binary or docker image, making it cloud native as heck.

Wombat is declarative, with stream pipelines defined in as few as a single config file, allowing you to specify 
connectors and a list of processing stages:

```yaml
input:
  gcp_pubsub:
    project: foo
    subscription: bar

pipeline:
  processors:
    - mapping: |
        root.message = this
        root.meta.link_count = this.links.length()
        root.user.age = this.user.age.number()

output:
  redis_streams:
    url: tcp://TODO:6379
    stream: baz
    max_in_flight: 20
```

## Why Fork?
First of all, this project is not a full fork. We still use the MIT licensed 
[RedPanda Benthos](https://github.com/redpanda-data/benthos) project as our base. We even still use a large part of
the Apache2 Licensed [RedPanda Connect](https://github.com/redpanda-data/connect) project. We did however fork some of
the components RedPanda made proprietary and added some of our own. 

The idea behind this move is to allow anyone to experience wombat without having to rely on a commercial entity 
behind it. This is a community project and always will be.

## Documentation
Take a look at the [documentation](https://wombat.dev) for more information on [how to get started][getting-started]

## Install
We're working on the release process, but you can either compile from source or pull the docker image:

```
docker pull ghcr.io/wombatwisdom/wombat
```

For more information check out the [getting started guide][getting-started].

## Run

```shell
wombat -c ./config.yaml
```

Or, with docker:

```shell
# Using a config file
docker run --rm -v /path/to/your/config.yaml:/wombat.yaml ghcr.io/wombatwisdom/wombat

# Using a series of -s flags
docker run --rm -p 4195:4195 ghcr.io/wombatwisdom/wombat \
  -s "input.type=http_server" \
  -s "output.type=kafka" \
  -s "output.kafka.addresses=kafka-server:9092" \
  -s "output.kafka.topic=wombat_topic"
```

## Honorable Mentions
I can't in all good faith take credit for the enormous amount of work that went into this project. Most of that is on Ash
and the rest of the community behind the old Benthos project. I'm just a guy who forked it and made it worse.

For those of you who miss Ash too much, here are some links to some of the old content still available:
- [Delivery guarantees can be a dodgy subject](https://youtu.be/QmpBOCvY8mY)

## Contributing
Contributions are welcome, please [read the guidelines](CONTRIBUTING.md), and watch your back.

[inputs]: https://wombat.dev/docs/components/inputs/about
[outputs]: https://wombat.dev/docs/components/outputs/about
[processors]: https://wombat.dev/docs/components/processors/about
[general-docs]: https://wombat.dev
[bloblang]: https://wombat.dev/bloblang
[releases]: https://github.com/wombatwisdom/wombat/releases
[getting-started]: https://wombat.dev/getting_started

[godoc-badge]: https://pkg.go.dev/badge/github.com/wombatwisdom/wombat/public
[godoc-url]: https://pkg.go.dev/github.com/wombatwisdom/wombat/public
[actions-badge]: https://github.com/wombatwisdom/wombat/actions/workflows/test.yml/badge.svg
[actions-url]: https://github.com/wombatwisdom/wombat/actions/workflows/test.yml
[website-badge]: https://img.shields.io/badge/Docs-Learn%20more-ffc7c7
[website-url]: https://wombat.dev/

[golangci-lint]: https://golangci-lint.run/
