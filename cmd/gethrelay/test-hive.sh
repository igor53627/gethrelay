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

echo -e "${GREEN}Setting up Hive client configuration for gethrelay...${NC}"

# Find or clone Hive
HIVE_CLIENTS_DIR=""
if [ -d "/tmp/hive-build/clients" ]; then
    HIVE_CLIENTS_DIR="/tmp/hive-build/clients"
elif command -v hive &> /dev/null; then
    # Try to find Hive directory
    HIVE_BIN_DIR=$(dirname $(which hive))
    if [ -d "$HIVE_BIN_DIR/../clients" ]; then
        HIVE_CLIENTS_DIR="$(cd $HIVE_BIN_DIR/../clients && pwd)"
    fi
fi

if [ -z "$HIVE_CLIENTS_DIR" ]; then
    echo -e "${YELLOW}Hive clients directory not found. Cloning Hive...${NC}"
    git clone --depth=1 https://github.com/ethereum/hive.git /tmp/hive-test-setup
    cd /tmp/hive-test-setup
    "$HIVE_BIN" --version 2>&1 || go build -o ./hive .
    HIVE_CLIENTS_DIR="/tmp/hive-test-setup/clients"
fi

# Set up gethrelay client
echo "Creating gethrelay client in $HIVE_CLIENTS_DIR..."
mkdir -p "$HIVE_CLIENTS_DIR/gethrelay"
cp -r "$HIVE_CLIENTS_DIR/go-ethereum/*" "$HIVE_CLIENTS_DIR/gethrelay/" 2>/dev/null || true

# Create hive.yaml
cat > "$HIVE_CLIENTS_DIR/gethrelay/hive.yaml" << 'EOF'
roles:
  - "eth1"
EOF

# Adapt scripts
if [ -f "$HIVE_CLIENTS_DIR/gethrelay/geth.sh" ]; then
    sed 's/geth/gethrelay/g' "$HIVE_CLIENTS_DIR/gethrelay/geth.sh" > "$HIVE_CLIENTS_DIR/gethrelay/gethrelay.sh"
    chmod +x "$HIVE_CLIENTS_DIR/gethrelay/gethrelay.sh"
fi

if [ -f "$HIVE_CLIENTS_DIR/gethrelay/enode.sh" ]; then
    sed -i '' 's/geth/gethrelay/g' "$HIVE_CLIENTS_DIR/gethrelay/enode.sh" 2>/dev/null || \
    sed -i 's/geth/gethrelay/g' "$HIVE_CLIENTS_DIR/gethrelay/enode.sh"
fi

# Create Dockerfile.local
cat > "$HIVE_CLIENTS_DIR/gethrelay/Dockerfile.local" << 'EOF'
FROM ethereum/gethrelay:local
USER root
RUN apk add --update --no-cache bash curl jq
COPY gethrelay.sh /gethrelay.sh
COPY enode.sh /enode.sh
RUN chmod +x /gethrelay.sh /enode.sh
USER gethrelay
EXPOSE 8545 30303 30303/udp
ENTRYPOINT ["/gethrelay.sh"]
EOF

echo -e "${GREEN}Running Hive tests...${NC}"

# Run tests using proper client setup (no --client-override, it doesn't exist!)
# Hive will discover the client from the clients/ directory
if [ -d "/tmp/hive-test-setup" ]; then
    cd /tmp/hive-test-setup
    HIVE_BIN="./hive"
fi

echo -e "${YELLOW}Running devp2p and RPC tests...${NC}"
"$HIVE_BIN" --client=gethrelay:local \
     --sim=devp2p \
     --sim=ethereum/rpc-compat \
     --loglevel=5

if [ $? -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Tests failed${NC}"
    exit 1
fi

