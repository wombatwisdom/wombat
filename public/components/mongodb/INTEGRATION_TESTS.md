# MongoDB Integration Tests

This directory contains integration tests for MongoDB components using testcontainers-go.

## Prerequisites

- Docker must be installed and running
- Go 1.23.4 or higher
- Sufficient system resources for running MongoDB containers

## Running Integration Tests

Integration tests are tagged with `//go:build integration` and are not run by default.

### Run all MongoDB integration tests:
```bash
go test -tags=integration -v -timeout=10m ./public/components/mongodb/...
```

### Run specific test files:
```bash
# MongoDB config tests
go test -tags=integration -v ./public/components/mongodb -run TestMongoDBConfigIntegration

# Change stream tests  
go test -tags=integration -v ./public/components/mongodb/change_stream -run TestChangeStreamIntegration
```

### Using the test script:
```bash
./scripts/run-integration-tests.sh
```

## Test Coverage

Integration tests provide real-world validation for:

1. **MongoDB Config (`mongodb_integration_test.go`)**
   - Connection establishment with real MongoDB
   - Authentication handling
   - CRUD operations
   - Connection pooling
   - Timeout handling
   - MongoDB features (bulk operations, aggregation, indexes)

2. **Change Streams (`change_stream_integration_test.go`)**
   - Reading insert/update/delete operations
   - Collection-level change streams
   - Database-level change streams
   - Error handling and recovery
   - Resource cleanup
   - Service framework integration

## How It Works

1. **Testcontainers**: Automatically starts MongoDB containers for each test suite
2. **Isolation**: Each test gets a fresh MongoDB instance
3. **Cleanup**: Containers are automatically removed after tests complete
4. **Realistic**: Tests run against actual MongoDB, not mocks

## Benefits

- **Confidence**: Validates actual MongoDB behavior
- **Compatibility**: Ensures code works with specific MongoDB versions
- **Feature Coverage**: Tests advanced features like change streams, aggregations
- **Error Scenarios**: Validates real error conditions and edge cases

## Adding New Integration Tests

When adding new MongoDB features:

1. Add integration tests in a `*_integration_test.go` file
2. Use the `//go:build integration` build tag
3. Follow the existing patterns for container setup
4. Ensure proper cleanup with `t.Cleanup()`
5. Test both success and failure scenarios

Example structure:
```go
//go:build integration
// +build integration

package mypackage_test

func TestMyFeatureIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration tests in short mode")
    }
    
    ctx := context.Background()
    mongoURI := setupMongoDBContainer(t, ctx)
    
    // Your test code here
}
```

## Troubleshooting

- **Docker not running**: Ensure Docker Desktop or Docker daemon is running
- **Timeouts**: Increase timeout with `-timeout=20m` flag
- **Resource limits**: Check Docker resource settings if tests fail to start containers
- **Port conflicts**: Testcontainers automatically handles port allocation

## CI/CD Integration

For CI pipelines:
1. Ensure Docker or Docker-in-Docker is available
2. Run with appropriate timeouts
3. Consider using `testing.Short()` to skip in certain environments
4. Monitor resource usage for parallel test execution