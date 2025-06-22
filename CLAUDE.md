# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Wombat is a high-performance stream processor built on top of RedPanda Benthos. It's a Go application that connects various data sources and sinks, performing transformations and filtering on data streams using a declarative YAML configuration.

## Common Development Commands

### Build and Installation
```bash
task build          # Build the main binary
task install        # Build and install to $GOPATH/bin
task docker         # Build Docker image
task deps           # Install dependencies
```

### Testing
```bash
task test           # Run all unit tests
task test-race      # Run tests with race detection
task test-integration  # Run integration tests (resource intensive)

# Run a single test
go test -v -run TestName ./path/to/package

# Run integration tests for specific components
go test -run 'TestIntegration/kafka' ./...
```

### Code Quality
```bash
task fmt            # Format code with gofmt and goimports
task lint           # Run golangci-lint
task docs           # Generate documentation
```

### Running the Application
```bash
# From source
go run ./cmd/wombat -c ./config.yaml

# After building
./wombat -c ./config.yaml
```

## Architecture

### Component System
Wombat uses a plugin-based architecture where components are registered at compile time:

1. **Component Location**: `/public/components/`
   - Each subdirectory contains a specific integration (mongodb, nats, snowflake, etc.)
   - Components implement interfaces from the Benthos framework

2. **Component Registration**: `/public/components/all/package.go`
   - All components must be imported here to be available at runtime
   - Uses blank imports to trigger init() functions

3. **Adding New Components**:
   - Create a new package under `/public/components/your_component/`
   - Implement the required interfaces (Input, Output, or Processor)
   - Add import to `/public/components/all/package.go`

### Key Directories
- `/cmd/wombat/` - Main application entry point
- `/internal/` - Internal packages (docs generation, implementations)
- `/public/` - Public API and components
- `/website/` - Documentation site (Astro + Starlight)
- `/examples/` - Example configurations

### Testing Patterns
- Unit tests: Standard Go testing with `testify` assertions
- Integration tests: Named with `TestIntegration` prefix
- Test files are colocated with source files (`foo.go` → `foo_test.go`)
- Some components use Ginkgo/Gomega for BDD-style tests

### Configuration
Wombat uses YAML configuration files defining:
- Input sources (Kafka, HTTP, files, databases, etc.)
- Pipeline processors (transformations, filters, enrichments)
- Output sinks (databases, message queues, HTTP endpoints)
- Uses Bloblang mapping language for data transformations

## Development Workflow

1. Make changes in a feature branch
2. Ensure tests pass: `task test`
3. Format code: `task fmt`
4. Run linting: `task lint`
5. Update documentation if needed: `task docs`
6. Create pull request against `main` branch

## Important Notes

- Uses Go 1.23.4 or later
- Task runner (taskfile.dev) is used instead of Make
- Version information is injected at build time via ldflags
- The project maintains compatibility with both MIT-licensed Benthos core and custom community components
- Docker images are published to `ghcr.io/wombatwisdom/wombat`

## Public Project Considerations

- This is a super public project with lots of people using it. So please be cautious