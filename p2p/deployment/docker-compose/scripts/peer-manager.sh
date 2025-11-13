#!/bin/sh
set -e

echo "========================================="
echo " Peer Manager Sidecar"
echo "========================================="
echo "Node: $NODE_NAME"
echo "RPC: $GETH_RPC"
echo "========================================="

# Install dependencies
apk add --no-cache curl jq 2>/dev/null || echo "Dependencies already installed"

SEEN_PEERS="/tmp/seen_peers.txt"
PEER_CHECK_INTERVAL=${PEER_CHECK_INTERVAL:-30}

touch $SEEN_PEERS

echo "[peer-manager] Waiting for gethrelay to be ready..."
sleep 15

echo "[peer-manager] Starting continuous peer discovery..."

while true; do
    # Query current peers
    PEERS=$(curl -s -X POST ${GETH_RPC} \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}' \
        2>/dev/null || echo '{"result":[]}')
    
    PEER_COUNT=$(echo "$PEERS" | jq '.result | length' 2>/dev/null || echo 0)
    echo "[peer-manager] Current peer count: $PEER_COUNT"
    
    # Extract .onion peers - look for .onion in the enode URL
    echo "$PEERS" | jq -r '.result[]? | .enode // empty' 2>/dev/null | grep '\.onion' | while read PEER_ENODE; do
        # Check if already promoted to trusted
        if ! grep -q "$PEER_ENODE" $SEEN_PEERS 2>/dev/null; then
            echo "[peer-manager] New .onion peer discovered via DHT: $PEER_ENODE"
            
            # Add as trusted peer (static-like persistence)
            RESULT=$(curl -s -X POST ${GETH_RPC} \
                -H "Content-Type: application/json" \
                -d "{\"jsonrpc\":\"2.0\",\"method\":\"admin_addTrustedPeer\",\"params\":[\"$PEER_ENODE\"],\"id\":1}" \
                2>/dev/null || echo '{"error":"failed"}')
            
            if echo "$RESULT" | grep -q '"result":true'; then
                echo "$PEER_ENODE" >> $SEEN_PEERS
                echo "[peer-manager] ✓ Promoted to trusted peer"
            else
                echo "[peer-manager] ✗ addTrustedPeer failed"
            fi
        fi
    done
    
    sleep $PEER_CHECK_INTERVAL
done
