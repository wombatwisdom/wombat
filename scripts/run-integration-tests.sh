#!/bin/bash
# Script to run integration tests with proper tags

set -e

echo "Running MongoDB integration tests..."
echo "================================"

# Set timeout for integration tests (10 minutes)
TIMEOUT="10m"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running. Please start Docker first.${NC}"
    exit 1
fi

echo -e "${YELLOW}Note: Integration tests require Docker to run MongoDB containers${NC}"
echo ""

# Run MongoDB integration tests
echo "Testing MongoDB components..."
if go test -tags=integration -v -timeout=$TIMEOUT ./public/components/mongodb/... -run Integration; then
    echo -e "${GREEN}✓ MongoDB integration tests passed${NC}"
else
    echo -e "${RED}✗ MongoDB integration tests failed${NC}"
    exit 1
fi

echo ""
echo "Testing MongoDB change stream components..."
if go test -tags=integration -v -timeout=$TIMEOUT ./public/components/mongodb/change_stream/... -run Integration; then
    echo -e "${GREEN}✓ MongoDB change stream integration tests passed${NC}"
else
    echo -e "${RED}✗ MongoDB change stream integration tests failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}All integration tests passed!${NC}"
echo ""
echo "To run integration tests for a specific package:"
echo "  go test -tags=integration -v ./path/to/package/..."
echo ""
echo "To run all tests including integration:"
echo "  go test -tags=integration -v ./..."