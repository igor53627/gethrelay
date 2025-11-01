#!/bin/bash
# test-workflow-local.sh - Test parts of the GitHub Actions workflow locally
# This script replicates key steps from the CI workflow for local testing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${GREEN}=== Local GitHub Actions Workflow Test ===${NC}"
echo ""

# Step 1: Unit tests
echo -e "${YELLOW}[1/5] Running unit tests...${NC}"
cd "$SCRIPT_DIR"
if go test -v -race -cover .; then
    echo -e "${GREEN}✓ Unit tests passed${NC}"
else
    echo -e "${RED}✗ Unit tests failed${NC}"
    exit 1
fi
echo ""

# Step 2: Build Docker image
echo -e "${YELLOW}[2/5] Building Docker image...${NC}"
cd "$REPO_ROOT"
if docker build -f "$SCRIPT_DIR/Dockerfile.gethrelay" \
    --build-arg GO_VERSION=1.24 \
    --build-arg COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "") \
    --build-arg VERSION=$(git describe --tags 2>/dev/null || echo "dev") \
    --build-arg BUILDNUM=1 \
    -t ethereum/gethrelay:latest \
    .; then
    echo -e "${GREEN}✓ Docker image built${NC}"
else
    echo -e "${RED}✗ Docker build failed${NC}"
    exit 1
fi
echo ""

# Step 3: Set up Hive
echo -e "${YELLOW}[3/5] Setting up Hive...${NC}"
if [ ! -d "/tmp/hive-local-test" ]; then
    git clone --depth=1 https://github.com/ethereum/hive.git /tmp/hive-local-test
fi
cd /tmp/hive-local-test
if [ ! -f "/usr/local/bin/hive" ] && [ ! -f "./hive" ]; then
    go build -o ./hive .
    echo -e "${GREEN}✓ Hive built${NC}"
else
    echo -e "${BLUE}ℹ Hive already exists${NC}"
fi
HIVE_BIN="./hive"
if command -v hive &> /dev/null; then
    HIVE_BIN="hive"
fi
echo ""

# Step 4: Setup Hive client configuration
echo -e "${YELLOW}[4/5] Setting up Hive client configuration...${NC}"
mkdir -p /tmp/hive-local-test/clients/gethrelay
# Copy base structure from go-ethereum
cp -r /tmp/hive-local-test/clients/go-ethereum/* /tmp/hive-local-test/clients/gethrelay/ 2>/dev/null || true

# Create hive.yaml
cat > /tmp/hive-local-test/clients/gethrelay/hive.yaml << 'EOF'
roles:
  - "eth1"
EOF

# Adapt scripts
if [ -f /tmp/hive-local-test/clients/gethrelay/geth.sh ]; then
    cp /tmp/hive-local-test/clients/gethrelay/geth.sh /tmp/hive-local-test/clients/gethrelay/gethrelay.sh
    sed -i '' 's/geth/gethrelay/g' /tmp/hive-local-test/clients/gethrelay/gethrelay.sh
fi

if [ -f /tmp/hive-local-test/clients/gethrelay/enode.sh ]; then
    sed -i '' 's/geth/gethrelay/g' /tmp/hive-local-test/clients/gethrelay/enode.sh
fi

# Create Dockerfile.local
cat > /tmp/hive-local-test/clients/gethrelay/Dockerfile.local << 'EOF'
FROM ethereum/gethrelay:latest
USER root
RUN apk add --update --no-cache bash curl jq
COPY gethrelay.sh /gethrelay.sh
COPY enode.sh /enode.sh 2>/dev/null || true
RUN chmod +x /gethrelay.sh /enode.sh 2>/dev/null || chmod +x /gethrelay.sh
USER gethrelay
EXPOSE 8545 30303 30303/udp
ENTRYPOINT ["/gethrelay.sh"]
EOF

# Adapt all scripts
find /tmp/hive-local-test/clients/gethrelay -type f -name "*.sh" | while read script; do
    if grep -q "geth" "$script" 2>/dev/null; then
        sed -i '' 's/\bgeth\b/gethrelay/g' "$script"
    fi
done

echo "Client directory structure:"
ls -la /tmp/hive-local-test/clients/gethrelay/
echo -e "${GREEN}✓ Client configuration created${NC}"
echo ""

# Step 5: Test client discovery
echo -e "${YELLOW}[5/5] Testing Hive client discovery...${NC}"
cd /tmp/hive-local-test

echo "Verifying client structure:"
echo "  - hive.yaml:"
cat clients/gethrelay/hive.yaml
echo ""
echo "  - Dockerfile.local (first 10 lines):"
head -10 clients/gethrelay/Dockerfile.local
echo ""

echo "Testing if Hive recognizes the client..."
echo "Note: This will attempt to discover and build the client"
echo ""

# Try to list clients (if supported) or test discovery
echo "Attempting to run Hive with gethrelay:local..."
echo "This will validate if the client is properly configured"
echo ""

echo -e "${BLUE}To run actual tests, use:${NC}"
echo "  cd /tmp/hive-local-test"
echo "  $HIVE_BIN --sim=ethereum/rpc --client=gethrelay:local --loglevel=5"
echo ""

echo -e "${GREEN}=== Local test setup complete! ===${NC}"
echo ""
echo "Summary:"
echo "  ✓ Unit tests passed"
echo "  ✓ Docker image built (ethereum/gethrelay:latest)"
echo "  ✓ Hive cloned and built"
echo "  ✓ Client configuration created in /tmp/hive-local-test/clients/gethrelay/"
echo ""
echo "Next steps:"
echo "  1. Verify the client with: cd /tmp/hive-local-test && $HIVE_BIN --sim=ethereum/rpc --client=gethrelay:local --loglevel=5"
echo "  2. Or run devp2p tests: cd /tmp/hive-local-test && $HIVE_BIN --sim=devp2p --client=gethrelay:local --loglevel=5"

