#!/bin/bash
# RED PHASE: Tor Hidden Service Infrastructure Tests
# These tests MUST FAIL initially, then pass after implementation

set -e

SERVER="geth-onion-dev"
COMPOSE_DIR="/root/gethrelay-docker"

echo "========================================="
echo "TOR HIDDEN SERVICE INFRASTRUCTURE TESTS"
echo "========================================="
echo ""

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local test_name="$1"
    local test_command="$2"

    TESTS_RUN=$((TESTS_RUN + 1))
    echo "TEST $TESTS_RUN: $test_name"

    if eval "$test_command"; then
        echo "✓ PASS"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo "✗ FAIL"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    echo ""
}

# TEST 1: torrc configuration file exists
run_test "torrc configuration file exists" \
    "ssh $SERVER 'test -f $COMPOSE_DIR/torrc'"

# TEST 2: torrc contains 3 hidden service configurations
run_test "torrc configures 3 hidden services" \
    "ssh $SERVER 'grep -c \"HiddenServiceDir\" $COMPOSE_DIR/torrc | grep -q 3'"

# TEST 3: torrc forwards to gethrelay P2P ports (30303)
run_test "torrc forwards to port 30303 for all 3 services" \
    "ssh $SERVER 'grep -c \"HiddenServicePort.*30303\" $COMPOSE_DIR/torrc | grep -q 3'"

# TEST 4: Hidden service directories exist in tor-data volume
run_test "Hidden service directories created in tor container" \
    "ssh $SERVER 'docker exec tor-proxy test -d /var/lib/tor/hidden_service_1 && docker exec tor-proxy test -d /var/lib/tor/hidden_service_2 && docker exec tor-proxy test -d /var/lib/tor/hidden_service_3'"

# TEST 5: .onion hostnames are generated
run_test ".onion hostnames exist for all 3 services" \
    "ssh $SERVER 'docker exec tor-proxy test -f /var/lib/tor/hidden_service_1/hostname && docker exec tor-proxy test -f /var/lib/tor/hidden_service_2/hostname && docker exec tor-proxy test -f /var/lib/tor/hidden_service_3/hostname'"

# TEST 6: docker-compose mounts torrc
run_test "docker-compose mounts torrc into tor container" \
    "ssh $SERVER 'grep -q \"./torrc:/etc/tor/torrc:ro\" $COMPOSE_DIR/docker-compose.yml'"

# TEST 7: docker-compose defines hidden service volume
run_test "docker-compose defines tor-hs-data volume" \
    "ssh $SERVER 'grep -q \"tor-hs-data:\" $COMPOSE_DIR/docker-compose.yml'"

# TEST 8: Bootstrap script exists for connecting nodes
run_test "bootstrap-tor-connections.sh script exists" \
    "ssh $SERVER 'test -f $COMPOSE_DIR/scripts/bootstrap-tor-connections.sh'"

# TEST 9: Extract and verify .onion addresses script exists
run_test "extract-onion-addresses.sh script exists" \
    "ssh $SERVER 'test -f $COMPOSE_DIR/scripts/extract-onion-addresses.sh'"

# TEST 10: Nodes actually connect via .onion addresses
run_test "At least one node has .onion peer connections" \
    "ssh $SERVER \"docker exec gethrelay-1 wget -q -O- --header='Content-Type: application/json' --post-data='{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"method\\\":\\\"admin_peers\\\",\\\"params\\\":[],\\\"id\\\":1}' http://127.0.0.1:8546 | grep -q onion\""

# TEST 11: Peer managers detect .onion peers
run_test "Peer manager logs show .onion peer detection" \
    "ssh $SERVER 'docker logs peer-manager-1 2>&1 | grep -q \"New .onion peer discovered\"'"

# TEST 12: Admin peers returns .onion addresses (count >= 1)
run_test "admin_peers API returns .onion addresses" \
    "ssh $SERVER \"docker exec gethrelay-1 wget -q -O- --header='Content-Type: application/json' --post-data='{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"method\\\":\\\"admin_peers\\\",\\\"params\\\":[],\\\"id\\\":1}' http://127.0.0.1:8546 | grep -c onion\" | grep -q '[1-9]'"

# Print summary
echo "========================================="
echo "TEST SUMMARY"
echo "========================================="
echo "Tests run:    $TESTS_RUN"
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo "✓ ALL TESTS PASSED - Infrastructure is working!"
    exit 0
else
    echo "✗ TESTS FAILED - Infrastructure needs implementation"
    exit 1
fi
