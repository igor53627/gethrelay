#!/bin/sh
set -e

# Healthcheck script for gethrelay containers
# Verifies all critical components are operational

GETH_RPC="${GETH_RPC:-http://127.0.0.1:8545}"
TOR_SOCKS_PORT="${TOR_SOCKS_PORT:-9050}"
P2P_PORT="${P2P_PORT:-30303}"

# Colors for output (if terminal supports it)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo "[healthcheck] $1"
}

log_success() {
    echo "${GREEN}[healthcheck] ✓ $1${NC}"
}

log_error() {
    echo "${RED}[healthcheck] ✗ $1${NC}"
}

log_warning() {
    echo "${YELLOW}[healthcheck] ⚠ $1${NC}"
}

# Check 1: P2P port is listening
check_p2p_port() {
    if nc -z localhost "${P2P_PORT}" 2>/dev/null; then
        log_success "P2P port ${P2P_PORT} is listening"
        return 0
    else
        log_error "P2P port ${P2P_PORT} is not listening"
        return 1
    fi
}

# Check 2: RPC endpoint is accessible (if HTTP is enabled)
check_rpc_endpoint() {
    if nc -z localhost 8545 2>/dev/null; then
        # Try to make an RPC call
        RESPONSE=$(wget -q -O - --timeout=2 --post-data='{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' \
            --header='Content-Type: application/json' \
            "${GETH_RPC}" 2>/dev/null || echo '{}')

        if echo "${RESPONSE}" | grep -q '"result"'; then
            log_success "RPC endpoint is responsive"
            return 0
        else
            log_warning "RPC endpoint is listening but not responding correctly"
            return 0  # Non-fatal, node might still be syncing
        fi
    else
        log_info "RPC endpoint not enabled (this is normal for some configurations)"
        return 0  # Non-fatal
    fi
}

# Check 3: Node has generated .onion address
check_onion_address() {
    if nc -z localhost 8545 2>/dev/null; then
        NODE_INFO=$(wget -q -O - --timeout=2 --post-data='{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
            --header='Content-Type: application/json' \
            "${GETH_RPC}" 2>/dev/null || echo '{}')

        ENODE=$(echo "${NODE_INFO}" | grep -o '"enode":"[^"]*"' | cut -d'"' -f4)

        if echo "${ENODE}" | grep -q '\.onion'; then
            ONION_ADDR=$(echo "${ENODE}" | grep -o '[a-z0-9]\{56\}\.onion')
            log_success "Tor .onion address: ${ONION_ADDR}"
            return 0
        else
            log_warning ".onion address not yet generated (node might still be initializing)"
            return 0  # Non-fatal during startup
        fi
    else
        log_info "RPC not available, skipping .onion address check"
        return 0  # Non-fatal
    fi
}

# Check 4: Can query peer count
check_peer_count() {
    if nc -z localhost 8545 2>/dev/null; then
        PEER_COUNT=$(wget -q -O - --timeout=2 --post-data='{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}' \
            --header='Content-Type: application/json' \
            "${GETH_RPC}" 2>/dev/null | grep -o '"result":"0x[0-9a-f]*"' | cut -d'"' -f4 || echo "")

        if [ -n "${PEER_COUNT}" ]; then
            PEER_COUNT_DEC=$((${PEER_COUNT}))
            log_info "Connected peers: ${PEER_COUNT_DEC}"
            return 0
        else
            log_info "Peer count not available yet"
            return 0  # Non-fatal
        fi
    else
        return 0  # RPC not enabled, skip
    fi
}

# Main healthcheck logic
main() {
    log_info "Running healthcheck..."

    # Critical check: P2P port must be listening
    if ! check_p2p_port; then
        exit 1
    fi

    # Non-critical checks (informational)
    check_rpc_endpoint
    check_onion_address
    check_peer_count

    log_success "Healthcheck passed"
    exit 0
}

# Run main function
main
