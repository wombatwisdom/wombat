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

This project uses [task](https://taskfile.dev/) as a build tool.
Please always make sure a pull request has been:

- Unit tested with `task test` (or `go test ./...`)
- Test coverage checked with `task test-coverage`
- Formatted with `task fmt`
- Linted with `task lint`. This uses [golangci-lint](https://golangci-lint.run/) which you need to have installed.

If your change impacts inputs, outputs or other connectors then try to test them with `task test-integration`. If the integration tests aren't working on your machine then don't panic, just mention it in your PR.

If your change has an impact on documentation then make sure it is generated with `task docs`. You can test out the documentation site locally by running `yarn && yarn start` in the `./website` directory.

Make sure all your commits are [signed](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).

## Testing Guidelines

We take testing seriously at Wombat. Good tests help us maintain reliability for the many users who depend on this project. Here's how to write effective tests for Wombat:

### Test Organization

#### Suite-Based Testing
For complex components, we use Ginkgo/Gomega for BDD-style testing:

```go
package mongodb_test

import (
    "testing"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestMongoDB(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "MongoDB Suite")
}
```

#### Standard Go Testing
For simpler components, standard Go testing with testify is preferred:

```go
func TestChangeStreamReader_Connect(t *testing.T) {
    tests := []struct {
        name        string
        options     ChangeStreamReaderOptions
        wantErr     bool
        errContains string
    }{
        // test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### What to Test

#### Critical Paths
Focus on testing the most critical functionality:
- **Connection lifecycle**: Connect, reconnect, and disconnect
- **Error handling**: Invalid configs, network errors, resource failures
- **Data flow**: Message creation, transformation, and acknowledgment
- **Resource management**: Proper cleanup, no leaks

#### Configuration Validation
Always test configuration parsing and validation:
```go
func TestConfig(t *testing.T) {
    // Test empty/invalid configurations
    // Test valid configurations with various options
    // Test edge cases (special characters, limits)
}
```

#### Error Scenarios
Don't just test the happy path:
```go
// Good: Tests specific error scenarios
func TestReader_Errors(t *testing.T) {
    t.Run("returns error when not connected", func(t *testing.T) {
        reader := NewReader(opts)
        _, err := reader.Read(ctx)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not connected")
    })
}
```

### Test Patterns

#### Table-Driven Tests
Use table-driven tests for comprehensive coverage:
```go
tests := []struct {
    name    string
    input   interface{}
    want    interface{}
    wantErr bool
}{
    {
        name:    "valid input",
        input:   "test",
        want:    "expected",
        wantErr: false,
    },
    // more cases...
}
```

#### Resource Cleanup
Always clean up test resources:
```go
t.Cleanup(func() {
    _ = client.Disconnect(ctx)
    _ = os.RemoveAll(tempDir)
})
```

#### Concurrent Access
Test concurrent operations when relevant:
```go
t.Run("handles concurrent closes", func(t *testing.T) {
    // Test that multiple goroutines can safely close
})
```

### Running Tests

```bash
# Run all tests
task test

# Run tests with race detection
task test-race

# Run specific package tests
go test -v ./public/components/mongodb/...

# Run a single test
go test -v -run TestConfig ./public/components/mongodb/

# Generate coverage report
task test-coverage
```

### Integration Tests

Integration tests use real services and are resource-intensive:

```bash
# Run all integration tests (not recommended locally)
task test-integration

# Run specific integration tests
go test -run 'TestIntegration/kafka' ./...
```

### Test Coverage

We aim for:
- **>70% coverage** for critical components (inputs, outputs, processors)
- **>50% coverage** for supporting code
- **100% coverage** for configuration parsing and validation

Check coverage with:
```bash
task test-coverage
# Open coverage.html in your browser
```

### Best Practices

1. **Test behavior, not implementation**: Focus on what the code does, not how
2. **Use meaningful test names**: `TestReader_Connect_FailsWithInvalidURI` not `TestReader1`
3. **Keep tests independent**: Each test should run in isolation
4. **Use test fixtures sparingly**: Prefer generating test data in the test
5. **Mock external dependencies**: Don't rely on external services for unit tests
6. **Document complex test scenarios**: Add comments explaining why a test exists
7. **Fail fast with clear messages**: Use `require` for critical assertions

### Example Test Structure

Here's a complete example following our patterns:

```go
package example_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestComponent_Lifecycle(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    t.Run("connects successfully with valid config", func(t *testing.T) {
        config := Config{URI: "valid://localhost"}
        component := NewComponent(config)
        
        err := component.Connect(ctx)
        require.NoError(t, err)
        
        t.Cleanup(func() {
            _ = component.Close(ctx)
        })
        
        // Test component is usable
        msg, err := component.Read(ctx)
        assert.NoError(t, err)
        assert.NotNil(t, msg)
    })
    
    t.Run("fails with invalid config", func(t *testing.T) {
        config := Config{URI: ""}
        component := NewComponent(config)
        
        err := component.Connect(ctx)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "URI required")
    })
}
```

Remember: Well-tested code is a gift to your future self and to the Wombat community!