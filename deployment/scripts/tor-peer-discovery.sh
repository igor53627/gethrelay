#!/bin/bash
# Tor Peer Discovery for Kubernetes Cluster
# This script extracts .onion addresses from the local pod and shares them
# with other pods via a ConfigMap for cluster-local peer discovery

set -e

NAMESPACE="${NAMESPACE:-gethrelay}"
CONFIGMAP_NAME="${CONFIGMAP_NAME:-tor-peer-addresses}"
POD_NAME="${POD_NAME:-$(hostname)}"
TOR_HOSTNAME_FILE="${TOR_HOSTNAME_FILE:-/var/lib/tor/hidden_service/hostname}"
ENR_FILE="${ENR_FILE:-/data/geth/geth/nodekey}"
STATIC_NODES_FILE="${STATIC_NODES_FILE:-/data/geth/geth/static-nodes.json}"
MAX_RETRIES="${MAX_RETRIES:-30}"
RETRY_DELAY="${RETRY_DELAY:-10}"

echo "[tor-peer-discovery] Starting Tor peer discovery for pod: ${POD_NAME}"

# Function to wait for Tor hidden service to be created
wait_for_tor_service() {
    local retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        if [ -f "$TOR_HOSTNAME_FILE" ]; then
            ONION_ADDRESS=$(cat "$TOR_HOSTNAME_FILE" | tr -d '[:space:]')
            if [ -n "$ONION_ADDRESS" ]; then
                echo "[tor-peer-discovery] Found .onion address: ${ONION_ADDRESS}"
                return 0
            fi
        fi
        echo "[tor-peer-discovery] Waiting for Tor hidden service... (attempt $((retries+1))/$MAX_RETRIES)"
        sleep $RETRY_DELAY
        retries=$((retries+1))
    done
    echo "[tor-peer-discovery] ERROR: Tor hidden service not available after ${MAX_RETRIES} attempts"
    return 1
}

# Function to get node ID from ENR/nodekey
get_node_id() {
    # Try to get from running geth admin API first
    if command -v curl >/dev/null 2>&1; then
        NODE_INFO=$(curl -s -X POST -H "Content-Type: application/json" \
            --data '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
            http://localhost:8545 2>/dev/null | grep -o '"enode://[^"]*"' | sed 's/"//g' || echo "")

        if [ -n "$NODE_INFO" ]; then
            # Extract node ID from enode URL (after enode:// and before @)
            NODE_ID=$(echo "$NODE_INFO" | sed 's/enode:\/\/\([^@]*\)@.*/\1/')
            if [ -n "$NODE_ID" ]; then
                echo "[tor-peer-discovery] Got node ID from geth: ${NODE_ID:0:16}..."
                echo "$NODE_ID"
                return 0
            fi
        fi
    fi

    # Fallback: use pod name as identifier
    echo "[tor-peer-discovery] Using pod name as node identifier"
    echo "pod-${POD_NAME}"
}

# Function to update ConfigMap with this pod's .onion address
update_configmap() {
    local onion_address="$1"
    local node_id="$2"
    local p2p_port="${P2P_PORT:-30303}"

    echo "[tor-peer-discovery] Updating ConfigMap with this pod's information"
    echo "[tor-peer-discovery]   Pod: ${POD_NAME}"
    echo "[tor-peer-discovery]   Onion: ${onion_address}"
    echo "[tor-peer-discovery]   Node ID: ${node_id:0:32}..."
    echo "[tor-peer-discovery]   Port: ${p2p_port}"

    # Create or patch ConfigMap with this pod's data
    # We store each pod's data as a separate key for easy merging
    kubectl get configmap "$CONFIGMAP_NAME" -n "$NAMESPACE" >/dev/null 2>&1 || \
        kubectl create configmap "$CONFIGMAP_NAME" -n "$NAMESPACE" --from-literal=init=true

    # Store the pod's data in JSON format
    POD_DATA=$(cat <<EOF
{
  "onion": "${onion_address}",
  "nodeId": "${node_id}",
  "port": ${p2p_port},
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
)

    kubectl patch configmap "$CONFIGMAP_NAME" -n "$NAMESPACE" \
        --type merge -p "{\"data\":{\"${POD_NAME}\": $(echo "$POD_DATA" | jq -R -s .)}}"

    echo "[tor-peer-discovery] ConfigMap updated successfully"
}

# Function to build static-nodes.json from ConfigMap
build_static_nodes() {
    local this_pod="$POD_NAME"
    local p2p_port="${P2P_PORT:-30303}"

    echo "[tor-peer-discovery] Building static-nodes.json from cluster peers"

    # Get ConfigMap data
    CONFIGMAP_DATA=$(kubectl get configmap "$CONFIGMAP_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo "{}")

    if [ "$CONFIGMAP_DATA" = "{}" ]; then
        echo "[tor-peer-discovery] ConfigMap not found or empty, creating empty static-nodes.json"
        echo "[]" > "$STATIC_NODES_FILE"
        return 0
    fi

    # Parse ConfigMap and build static nodes array
    STATIC_NODES="["
    FIRST=true

    # Get all pod data from ConfigMap (excluding 'init' key)
    PEER_KEYS=$(echo "$CONFIGMAP_DATA" | jq -r '.data | keys[]' | grep -v '^init$' || echo "")

    for POD_KEY in $PEER_KEYS; do
        # Skip self
        if [ "$POD_KEY" = "$this_pod" ]; then
            echo "[tor-peer-discovery] Skipping self: ${POD_KEY}"
            continue
        fi

        # Extract peer data
        PEER_DATA=$(echo "$CONFIGMAP_DATA" | jq -r ".data[\"${POD_KEY}\"]" | jq -r .)
        PEER_ONION=$(echo "$PEER_DATA" | jq -r '.onion')
        PEER_NODE_ID=$(echo "$PEER_DATA" | jq -r '.nodeId')
        PEER_PORT=$(echo "$PEER_DATA" | jq -r '.port')

        if [ -z "$PEER_ONION" ] || [ "$PEER_ONION" = "null" ]; then
            echo "[tor-peer-discovery] Skipping invalid peer: ${POD_KEY}"
            continue
        fi

        # Build enode URL
        ENODE_URL="enode://${PEER_NODE_ID}@${PEER_ONION}:${PEER_PORT}"

        echo "[tor-peer-discovery] Adding peer: ${POD_KEY} -> ${PEER_ONION}"

        # Add to array
        if [ "$FIRST" = true ]; then
            STATIC_NODES="${STATIC_NODES}\"${ENODE_URL}\""
            FIRST=false
        else
            STATIC_NODES="${STATIC_NODES},\"${ENODE_URL}\""
        fi
    done

    STATIC_NODES="${STATIC_NODES}]"

    # Write static-nodes.json
    echo "$STATIC_NODES" | jq . > "$STATIC_NODES_FILE"

    PEER_COUNT=$(echo "$STATIC_NODES" | jq '. | length')
    echo "[tor-peer-discovery] Built static-nodes.json with ${PEER_COUNT} peers"
    if [ "$PEER_COUNT" -gt 0 ]; then
        echo "[tor-peer-discovery] Static nodes content:"
        cat "$STATIC_NODES_FILE"
    fi
}

# Function to run continuous discovery (for sidecar mode)
run_continuous_discovery() {
    local update_interval="${UPDATE_INTERVAL:-60}"

    echo "[tor-peer-discovery] Running in continuous mode (update interval: ${update_interval}s)"

    while true; do
        echo "[tor-peer-discovery] Checking for peer updates..."

        # Rebuild static nodes from ConfigMap
        build_static_nodes

        # Signal geth to reload static nodes (if admin API available)
        if command -v curl >/dev/null 2>&1; then
            curl -s -X POST -H "Content-Type: application/json" \
                --data '{"jsonrpc":"2.0","method":"admin_addPeer","params":[],"id":1}' \
                http://localhost:8545 >/dev/null 2>&1 || true
        fi

        echo "[tor-peer-discovery] Sleeping for ${update_interval}s..."
        sleep "$update_interval"
    done
}

# Main execution
main() {
    local mode="${MODE:-init}"

    echo "[tor-peer-discovery] Running in ${mode} mode"

    # Wait for Tor hidden service
    if ! wait_for_tor_service; then
        echo "[tor-peer-discovery] ERROR: Failed to get .onion address"
        exit 1
    fi

    # Get node ID
    NODE_ID=$(get_node_id)

    # Update ConfigMap with this pod's information
    update_configmap "$ONION_ADDRESS" "$NODE_ID"

    # Build initial static-nodes.json
    build_static_nodes

    # If in sidecar mode, keep running and updating
    if [ "$mode" = "sidecar" ]; then
        run_continuous_discovery
    else
        echo "[tor-peer-discovery] Init complete, exiting"
        exit 0
    fi
}

# Run main function
main
