# Wombat
Wombat is a stream processing toolkit which allows you to define your stream processors
in a declarative way.

## Why
The goal of wombat is twofold. First, we want to make it easier for people to write stream processors and benefit from
the ecosystem that is already available. Second, we want to make it easier for people to create smaller binaries that
contain only the components they need.

## How it works
Wombat allows you to add binaries based on a spec file. When adding a binary, wombat will download the libraries and
generate a runtime binary that contains all the components defined in the spec file. Except for the binaries subcommand,
the wombat binary will pass all subcommands to the currently selected binary. You may want to use `wombat binary current`
to see which binary is currently selected or select a binary using `wombat binary select <name>`. A list of available
binaries can be retrieved using `wombat binary list`.

## Getting Started
Wombat will compile a binary based on the binary spec you provide. This means that you can decide
what needs to go into your binary and what not, allowing for smaller binaries and faster startup times.

This also means that wombat will need access to the golang compiler. If you don't have it installed
yet, you can download it from the [golang website](https://golang.org/dl/). Make sure your `GO_ROOT` environment
variable is set correctly.

Once all of this is done, grab a binary from the releases and add a binary. You can do this by running the following
command:

```shell
wombat binary add --select preset:full
```

## Adding your own builds
Builds can be added based on a build spec which can be retrieved from a file or from a URL. This spec file is a yaml
file that describes what needs to go into the binary. Here is an example spec file:

```yaml
name: full
benthos_version: v4.35.0
libraries:
  - module: github.com/redpanda-data/benthos/v4
    version: v4.35.0
    packages:
      - public/components/io
      - public/components/pure

  - module: github.com/redpanda-data/connect/v4
    version: v4.33.0
    packages:
      - public/components/community
```
