#!/bin/bash
# test-hive.sh - Run Hive tests for gethrelay

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Please start Docker and try again.${NC}"
    echo -e "${YELLOW}Running unit tests instead...${NC}"
    cd "$SCRIPT_DIR"
    go test -v .
    exit 0
fi

echo -e "${GREEN}Building gethrelay Docker image...${NC}"
cd "$REPO_ROOT"
docker build -f "$SCRIPT_DIR/Dockerfile.gethrelay" -t ethereum/gethrelay:local .

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to build Docker image${NC}"
    exit 1
fi

echo -e "${GREEN}Docker image built successfully${NC}"

# Find Hive binary
HIVE_BIN=""
if command -v hive &> /dev/null; then
    HIVE_BIN="hive"
elif [ -f "/tmp/hive-build/hive" ]; then
    HIVE_BIN="/tmp/hive-build/hive"
    echo -e "${BLUE}Using Hive from /tmp/hive-build/hive${NC}"
else
    echo -e "${YELLOW}Hive not found. Attempting to install...${NC}"
    if [ -d "/tmp/hive-build" ]; then
        cd /tmp/hive-build
        go build -o hive . 2>&1 | tail -5
        if [ -f "/tmp/hive-build/hive" ]; then
            HIVE_BIN="/tmp/hive-build/hive"
            echo -e "${GREEN}Hive built successfully${NC}"
        fi
    fi
    
    if [ -z "$HIVE_BIN" ]; then
        echo -e "${YELLOW}Hive installation failed. Running unit tests instead...${NC}"
        cd "$SCRIPT_DIR"
        go test -v .
        exit 0
    fi
fi

echo -e "${GREEN}Running Hive tests...${NC}"

# Run devp2p tests using client override
echo -e "${YELLOW}Running devp2p tests...${NC}"
"$HIVE_BIN" --client=go-ethereum:local \
     --client-override=gethrelay:local=ethereum/gethrelay:local \
     --sim=devp2p \
     --sim=ethereum/rpc-compat \
     --loglevel=5 \
     --clients=gethrelay:local

if [ $? -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Tests failed${NC}"
    exit 1
fi

