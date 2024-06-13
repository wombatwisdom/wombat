# Wombat

Wombat is a stream processing toolkit which allows you to define your stream processors
in a declarative way. It is designed to be simple and easy to use while still harboring
immense power and flexibility.

## Getting Started
The easiest way at this point to get started with wombat is to download one of the binaries
from the releases.

The first thing to do is to create a wombat config file. This is a yaml file that describes
where data needs to be read from, what processors need to be applied and where the result 
should go to. Here is an example config file:

```yaml
input:
  generate:
    count: 1
    mapping: |-
      root.message = "Hello, World!"

pipeline:
  processors:
    - log:
        message: "Look mom, a message: ${! this }"
    - mapping: |-
        root.result = root.message.uppercase()

output:
  stdout: {}
```

This config file defines a rather trivial pipeline which:
- generates a single message
- logs it
- converts it to uppercase
- prints it to stdout

To run this pipeline, you can use the following command:

```shell
wombat -c path/to/config.yaml
```

Obviously, there are a lot more things you can do with wombat. 
We are still working hard to make the online documentation available.

To get a list of all components that are available in wombat, you can 
use the following command:

```shell
wombat list
```

## Disclaimer
Wombat builds on top of the [RedPanda Benthos](https://github.com/redpanda-data/benthos) and the 
free (Apache 2) parts of the [RedPanda Connect](https://github.com/redpanda-data/connect) project.
However, there are additional components being added solely within wombat and we will keep on 
developing and extending wombat beyond the components in the RedPanda projects.

So why wombat you may ask? Well, we like to be independent from the RedPanda project and still 
be able to use the tool we love like we did before. We also want to extend it in ways that are
out of the interest zone of RedPanda.