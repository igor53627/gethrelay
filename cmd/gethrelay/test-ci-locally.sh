#!/bin/bash
# test-ci-locally.sh - Replicate the exact CI workflow locally for debugging
# This matches the GitHub Actions workflow exactly to test before pushing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${GREEN}=== Testing CI Workflow Locally ===${NC}"
echo "This replicates the exact GitHub Actions workflow steps"
echo ""

# Check Docker
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Please start Docker.${NC}"
    exit 1
fi

# Step 1: Build Docker image (matches CI)
echo -e "${YELLOW}[1/4] Building gethrelay Docker image (matching CI)...${NC}"
cd "$REPO_ROOT"
docker build -f "$SCRIPT_DIR/Dockerfile.gethrelay" \
    --build-arg GO_VERSION=1.24 \
    --build-arg COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "") \
    --build-arg VERSION=$(git describe --tags 2>/dev/null || echo "dev") \
    --build-arg BUILDNUM=1 \
    -t ethereum/gethrelay:latest \
    .

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Docker build failed${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker image built: ethereum/gethrelay:latest${NC}"
docker images | grep gethrelay
echo ""

# Step 2: Set up Hive (matches CI)
echo -e "${YELLOW}[2/4] Setting up Hive (matching CI)...${NC}"
HIVE_DIR="/tmp/hive-local-ci-test"
if [ -d "$HIVE_DIR" ]; then
    echo "Updating existing Hive clone..."
    cd "$HIVE_DIR" && git pull && cd - > /dev/null
else
    echo "Cloning Hive..."
    git clone --depth=1 https://github.com/ethereum/hive.git "$HIVE_DIR"
fi

cd "$HIVE_DIR"
if [ ! -f "./hive" ] && [ ! -f "/usr/local/bin/hive" ]; then
    echo "Building Hive..."
    go build -o ./hive .
fi

HIVE_BIN="./hive"
if command -v hive &> /dev/null; then
    HIVE_BIN="hive"
fi

echo -e "${GREEN}✓ Hive ready at: $HIVE_BIN${NC}"
echo "Hive version:"
$HIVE_BIN --version 2>&1 || echo "Cannot get version"
echo ""

# Step 3: Test the exact CI approach - client-override
echo -e "${YELLOW}[3/4] Testing CI approach: client-override with go-ethereum:local...${NC}"
cd "$HIVE_DIR"

echo "Verifying base image exists:"
docker images | grep "ethereum/gethrelay" || {
    echo -e "${RED}✗ Base image ethereum/gethrelay:latest not found!${NC}"
    exit 1
}
echo ""

echo "Testing if go-ethereum client exists:"
if [ ! -d "clients/go-ethereum" ]; then
    echo -e "${RED}✗ clients/go-ethereum directory not found!${NC}"
    exit 1
fi
echo -e "${GREEN}✓ go-ethereum client exists${NC}"
echo ""

echo "Testing Hive client-override approach:"
echo "Command: hive --sim=ethereum/rpc-compat --client=go-ethereum:local --client-override=gethrelay:local=ethereum/gethrelay:latest --clients=gethrelay:local --loglevel=5"
echo ""

# Test with a short timeout first
echo -e "${BLUE}Running test with 2 minute timeout (short test)...${NC}"
set +e
$HIVE_BIN --sim=ethereum/rpc-compat \
    --client=go-ethereum:local \
    --client-override=gethrelay:local=ethereum/gethrelay:latest \
    --clients=gethrelay:local \
    --loglevel=5 \
    --sim.timelimit=2m \
    --results-root=./test-results-local 2>&1 | tee /tmp/hive-local-test.log
HIVE_EXIT_CODE=$?
set -e

echo ""
echo "Hive exit code: $HIVE_EXIT_CODE"
echo ""

# Step 4: Analyze results
echo -e "${YELLOW}[4/4] Analyzing results...${NC}"

echo "Checking Hive output for errors:"
if grep -qi "unknown client" /tmp/hive-local-test.log; then
    echo -e "${RED}✗ ERROR: 'unknown client' found in output!${NC}"
    echo "This is the same issue we're seeing in CI."
    grep -i "unknown client" /tmp/hive-local-test.log
    echo ""
    echo "Full output (first 50 lines):"
    head -50 /tmp/hive-local-test.log
    exit 1
fi

echo "Checking for simulator/test mentions:"
if grep -qi "simulator.*started\|running.*test" /tmp/hive-local-test.log; then
    echo -e "${GREEN}✓ Simulators appear to have started${NC}"
    grep -i "simulator\|test" /tmp/hive-local-test.log | head -5
else
    echo -e "${YELLOW}⚠ No simulator mentions in output${NC}"
fi

echo ""
echo "Checking for test results:"
if [ -d "test-results-local" ] && [ -n "$(find test-results-local -type f 2>/dev/null)" ]; then
    echo -e "${GREEN}✓ Test results found!${NC}"
    echo "Results directory contents:"
    find test-results-local -type f | head -10
    echo ""
    echo "Looking for simulator.log files:"
    find test-results-local -name "simulator.log" | head -5
    echo ""
    echo -e "${GREEN}✓ SUCCESS: Tests ran and produced results!${NC}"
else
    echo -e "${YELLOW}⚠ No test results found${NC}"
    echo "Results directory: $(test -d test-results-local && echo 'exists but empty' || echo 'does not exist')"
    echo ""
    echo "This matches the CI issue - tests aren't producing results."
    echo ""
    echo "Full Hive output (last 50 lines):"
    tail -50 /tmp/hive-local-test.log
fi

echo ""
echo "Summary:"
if [ $HIVE_EXIT_CODE -eq 0 ] && [ -d "test-results-local" ] && [ -n "$(find test-results-local -type f 2>/dev/null)" ]; then
    echo -e "${GREEN}✓ Local test successful - CI should work too!${NC}"
    exit 0
else
    echo -e "${RED}✗ Local test failed - matches CI issue${NC}"
    echo "Diagnose the issue above before pushing to CI."
    exit 1
fi

