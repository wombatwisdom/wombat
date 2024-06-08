Contributing to Wombat
=======================

Joining our Wisdom (yeah, the home of a wombat is called a wisdom) by contributing to the Wombat project is a selfless, boring and occasionally painful act. As such any contributors to this project will be treated with the respect and compassion that they deserve.

Please be dull, please be respecting of others and their efforts, please do not take criticism or rejection of your ideas personally.

## Reporting Bugs

If you find a bug then please let the project know by opening an issue after doing the following:

- Do a quick search of the existing issues to make sure the bug isn't already reported
- Try and make a minimal list of steps that can reliably reproduce the bug you are experiencing
- Collect as much information as you can to help identify what the issue is (project version, configuration files, etc)

## Suggesting Enhancements

Having even the most casual interest in Wombat gives you honorary membership of the Wisdom, entitling you to give a reserved (and hypothetical) tickle of the projects' toes in order to steer it in the direction of your whim.

Please don't abuse this entitlement, the poor wombats can only gobble so many features before they turn depressed and demoralized beyond repair. Enhancements should roughly follow the general goals of Wombat and be:

- Common use cases
- Simple to understand
- Simple to monitor

You can help us out by doing the following before raising a new issue:

- Check that the feature hasn't been requested already by searching existing issues
- Try and reduce your enhancement into a single, concise and deliverable request, rather than a general idea
- Explain your own use cases as the basis of the request

## Adding Features

Pull requests are always welcome. However, before going through the trouble of implementing a change it's worth creating an issue. This allows us to discuss the changes and make sure they are a good fit for the project.

Please always make sure a pull request has been:

- Unit tested with `make test`
- Linted with `make lint`
- Formatted with `make fmt`

If your change impacts inputs, outputs or other connectors then try to test them with `make test-integration`. If the integration tests aren't working on your machine then don't panic, just mention it in your PR.

If your change has an impact on documentation then make sure it is generated with `make docs`. You can test out the documentation site locally by running `yarn && yarn start` in the `./website` directory.