#!/bin/bash
set -e

# Deployment validation script
# Tests all critical components of the gethrelay Docker Compose deployment

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_DIR="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

run_test() {
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo ""
    log_info "Test $TESTS_TOTAL: $1"
}

# Change to compose directory
cd "$COMPOSE_DIR"

echo "========================================="
echo " Gethrelay Deployment Validation"
echo "========================================="
echo ""

# Test 1: Check Docker Compose is installed
run_test "Docker Compose installation"
if command -v docker-compose &> /dev/null; then
    VERSION=$(docker-compose --version)
    log_success "Docker Compose installed: $VERSION"
else
    log_error "Docker Compose not found"
    exit 1
fi

# Test 2: Check all containers are running
run_test "All containers running"
EXPECTED_CONTAINERS=7
RUNNING_CONTAINERS=$(docker-compose ps -q | wc -l | tr -d ' ')

if [ "$RUNNING_CONTAINERS" -eq "$EXPECTED_CONTAINERS" ]; then
    log_success "All $EXPECTED_CONTAINERS containers are running"
else
    log_error "Expected $EXPECTED_CONTAINERS containers, found $RUNNING_CONTAINERS"
    docker-compose ps
fi

# Test 3: Check Tor container health
run_test "Tor container health"
TOR_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' tor-proxy 2>/dev/null || echo "unknown")

if [ "$TOR_HEALTH" = "healthy" ]; then
    log_success "Tor container is healthy"
else
    log_error "Tor container health: $TOR_HEALTH"
fi

# Test 4: Tor SOCKS5 proxy is accessible
run_test "Tor SOCKS5 proxy accessibility"
if docker exec tor-proxy nc -z localhost 9050 2>/dev/null; then
    log_success "Tor SOCKS5 proxy is accessible on port 9050"
else
    log_error "Tor SOCKS5 proxy is not accessible"
fi

# Test 5: Tor control port is accessible
run_test "Tor control port accessibility"
if docker exec tor-proxy nc -z localhost 9051 2>/dev/null; then
    log_success "Tor control port is accessible on port 9051"
else
    log_error "Tor control port is not accessible"
fi

# Test 6-8: Check gethrelay nodes are healthy
for i in 1 2 3; do
    run_test "Gethrelay node $i health"
    NODE_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' gethrelay-$i 2>/dev/null || echo "unknown")

    if [ "$NODE_HEALTH" = "healthy" ]; then
        log_success "Gethrelay node $i is healthy"
    else
        log_error "Gethrelay node $i health: $NODE_HEALTH"
    fi
done

# Test 9-11: Check P2P ports are listening
for i in 1 2 3; do
    run_test "Gethrelay node $i P2P port"
    if docker exec gethrelay-$i nc -z localhost 30303 2>/dev/null; then
        log_success "Node $i P2P port 30303 is listening"
    else
        log_error "Node $i P2P port 30303 is not listening"
    fi
done

# Test 12-14: Check RPC endpoints
for i in 1 2 3; do
    run_test "Gethrelay node $i RPC endpoint"
    if docker exec gethrelay-$i nc -z localhost 8545 2>/dev/null; then
        log_success "Node $i RPC endpoint is accessible"
    else
        log_error "Node $i RPC endpoint is not accessible"
    fi
done

# Test 15-17: Query node info and check for .onion address
for i in 1 2 3; do
    run_test "Gethrelay node $i .onion address generation"
    NODE_INFO=$(docker exec gethrelay-$i sh -c '
        wget -q -O - --timeout=5 --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
        --header="Content-Type: application/json" \
        http://127.0.0.1:8545 2>/dev/null || echo "{}"
    ')

    ONION_ADDR=$(echo "$NODE_INFO" | grep -o '[a-z0-9]\{56\}\.onion' || echo "")

    if [ -n "$ONION_ADDR" ]; then
        log_success "Node $i has .onion address: $ONION_ADDR"
    else
        log_warning "Node $i .onion address not generated yet (may still be initializing)"
    fi
done

# Test 18-20: Check peer managers are running
for i in 1 2 3; do
    run_test "Peer manager $i running"
    if docker ps | grep -q "peer-manager-$i"; then
        log_success "Peer manager $i is running"

        # Check if it has logged any activity
        LOGS=$(docker logs peer-manager-$i 2>&1 | tail -5)
        if echo "$LOGS" | grep -q "peer-manager"; then
            log_info "Recent activity detected"
        fi
    else
        log_error "Peer manager $i is not running"
    fi
done

# Test 21-23: Check peer counts
sleep 2  # Give nodes time to connect
for i in 1 2 3; do
    run_test "Gethrelay node $i peer connections"
    PEER_COUNT_HEX=$(docker exec gethrelay-$i sh -c '
        wget -q -O - --timeout=5 --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
        --header="Content-Type: application/json" \
        http://127.0.0.1:8545 2>/dev/null | grep -o "\"result\":\"0x[0-9a-f]*\"" | cut -d"\"" -f4
    ' || echo "")

    if [ -n "$PEER_COUNT_HEX" ]; then
        PEER_COUNT_DEC=$((${PEER_COUNT_HEX}))
        if [ "$PEER_COUNT_DEC" -ge 1 ]; then
            log_success "Node $i has $PEER_COUNT_DEC peer(s)"
        else
            log_warning "Node $i has 0 peers (discovery may still be in progress)"
        fi
    else
        log_warning "Could not query peer count for node $i"
    fi
done

# Test 24: Check volumes exist
run_test "Docker volumes"
EXPECTED_VOLUMES=5
VOLUME_COUNT=$(docker volume ls -q | grep -c "docker-compose_geth-data\|docker-compose_tor" || echo "0")

if [ "$VOLUME_COUNT" -ge "$EXPECTED_VOLUMES" ]; then
    log_success "All expected volumes exist"
else
    log_warning "Expected at least $EXPECTED_VOLUMES volumes, found $VOLUME_COUNT"
fi

# Test 25: Check network exists
run_test "Docker network"
if docker network inspect docker-compose_tor-network &>/dev/null; then
    log_success "Tor network exists"

    # Check network subnet
    SUBNET=$(docker network inspect docker-compose_tor-network | grep -o '172\.28\.0\.0/16' || echo "")
    if [ -n "$SUBNET" ]; then
        log_info "Network subnet: $SUBNET"
    fi
else
    log_error "Tor network does not exist"
fi

# Test 26: Check for errors in logs
run_test "Error log analysis"
ERROR_COUNT=0
for container in gethrelay-1 gethrelay-2 gethrelay-3; do
    ERRORS=$(docker logs $container 2>&1 | grep -i "fatal\|panic" | wc -l)
    ERROR_COUNT=$((ERROR_COUNT + ERRORS))
done

if [ "$ERROR_COUNT" -eq 0 ]; then
    log_success "No fatal errors found in logs"
else
    log_warning "Found $ERROR_COUNT fatal/panic messages in logs"
fi

# Test 27: Check trusted peers
run_test "Trusted peers validation"
TRUSTED_COUNT=0
for i in 1 2 3; do
    TRUSTED=$(docker exec gethrelay-$i sh -c '
        wget -q -O - --timeout=5 --post-data='\''{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}'\'' \
        --header="Content-Type: application/json" \
        http://127.0.0.1:8545 2>/dev/null | grep -o "\"trusted\":true" | wc -l
    ' || echo "0")
    TRUSTED_COUNT=$((TRUSTED_COUNT + TRUSTED))
done

if [ "$TRUSTED_COUNT" -gt 0 ]; then
    log_success "Found $TRUSTED_COUNT trusted peer(s) across all nodes"
else
    log_warning "No trusted peers found yet (peer discovery may still be in progress)"
fi

# Summary
echo ""
echo "========================================="
echo " Validation Summary"
echo "========================================="
echo -e "Total tests: ${BLUE}$TESTS_TOTAL${NC}"
echo -e "Passed:      ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed:      ${RED}$TESTS_FAILED${NC}"
echo "========================================="

if [ "$TESTS_FAILED" -eq 0 ]; then
    echo -e "${GREEN}✓ All critical tests passed!${NC}"
    echo ""
    echo "Deployment is operational. Nodes may still be discovering peers."
    echo "Run 'docker-compose logs -f' to monitor peer discovery."
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    echo "Check the failures above and review logs with:"
    echo "  docker-compose logs -f"
    exit 1
fi
