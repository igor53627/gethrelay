#!/bin/sh
set -e

echo "========================================="
echo " Peer Manager Sidecar"
echo "========================================="
echo "Node: ${NODE_NAME:-unknown}"
echo "RPC: ${GETH_RPC}"
echo "Check Interval: ${PEER_CHECK_INTERVAL:-30}s"
echo "========================================="

# Install dependencies
echo "[peer-manager] Installing dependencies..."
apk add --no-cache curl jq netcat-openbsd >/dev/null 2>&1

# Configuration
SEEN_PEERS="/tmp/seen_peers.txt"
PEER_CHECK_INTERVAL="${PEER_CHECK_INTERVAL:-30}"
RETRY_DELAY=5
MAX_RETRIES=12

# Initialize state file
touch "${SEEN_PEERS}"

# Wait for gethrelay RPC to be ready
echo "[peer-manager] Waiting for gethrelay RPC endpoint..."
RETRY_COUNT=0
while ! nc -z 127.0.0.1 8545 2>/dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ ${RETRY_COUNT} -ge ${MAX_RETRIES} ]; then
        echo "[peer-manager] ERROR: RPC endpoint not available after ${MAX_RETRIES} attempts"
        exit 1
    fi
    sleep ${RETRY_DELAY}
done

echo "[peer-manager] RPC endpoint ready"
echo "[peer-manager] Starting continuous peer discovery and management..."

# Helper function to make RPC calls
rpc_call() {
    METHOD="$1"
    PARAMS="${2:-[]}"

    curl -s -X POST "${GETH_RPC}" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"${METHOD}\",\"params\":${PARAMS},\"id\":1}" \
        2>/dev/null || echo '{"result":null,"error":"RPC call failed"}'
}

# Main discovery loop
while true; do
    # Query current peers from gethrelay
    PEERS=$(rpc_call "admin_peers")

    # Check if RPC call was successful
    if echo "${PEERS}" | jq -e '.result' >/dev/null 2>&1; then
        PEER_COUNT=$(echo "${PEERS}" | jq '.result | length' 2>/dev/null || echo 0)
        echo "[peer-manager] Current peer count: ${PEER_COUNT}"

        # Extract .onion peers from connected peers
        ONION_PEERS=$(echo "${PEERS}" | jq -r '
            .result[] |
            select(.network.remoteAddress | contains(".onion")) |
            .enode // empty
        ' 2>/dev/null || echo "")

        # Process each .onion peer found via DHT
        if [ -n "${ONION_PEERS}" ]; then
            echo "${ONION_PEERS}" | while IFS= read -r PEER_ENODE; do
                if [ -n "${PEER_ENODE}" ]; then
                    # Check if we've already promoted this peer
                    if ! grep -Fxq "${PEER_ENODE}" "${SEEN_PEERS}"; then
                        echo "[peer-manager] New .onion peer discovered via DHT:"
                        echo "[peer-manager]   ${PEER_ENODE}"

                        # Add as trusted peer (static-like persistence)
                        # This ensures the peer persists across restarts
                        RESULT=$(rpc_call "admin_addTrustedPeer" "[\"${PEER_ENODE}\"]")

                        if echo "${RESULT}" | jq -e '.result == true' >/dev/null 2>&1; then
                            echo "${PEER_ENODE}" >> "${SEEN_PEERS}"
                            echo "[peer-manager] Successfully promoted to trusted peer"
                        else
                            ERROR=$(echo "${RESULT}" | jq -r '.error.message // "Unknown error"')
                            echo "[peer-manager] Failed to add trusted peer: ${ERROR}"
                        fi
                    fi
                fi
            done
        fi

        # Also check for any peers that might have been removed
        # Query trusted peers list
        TRUSTED_PEERS=$(rpc_call "admin_peers")
        TRUSTED_COUNT=$(echo "${TRUSTED_PEERS}" | jq '[.result[] | select(.trusted == true)] | length' 2>/dev/null || echo 0)

        if [ "${TRUSTED_COUNT}" -gt 0 ]; then
            echo "[peer-manager] Trusted peer count: ${TRUSTED_COUNT}"
        fi

    else
        echo "[peer-manager] WARNING: Failed to query peers from RPC"
    fi

    # Wait before next check
    sleep "${PEER_CHECK_INTERVAL}"
done
