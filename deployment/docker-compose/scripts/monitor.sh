#!/bin/bash

# Real-time monitoring dashboard for gethrelay deployment
# Shows peer counts, .onion addresses, and system health

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Change to compose directory
cd "$COMPOSE_DIR"

clear_screen() {
    clear
    echo -e "${BOLD}========================================="
    echo " Gethrelay Cluster Monitor"
    echo "=========================================${NC}"
    echo "Press Ctrl+C to exit"
    echo ""
}

get_peer_count() {
    local node=$1
    docker exec gethrelay-$node sh -c '
        wget -q -O - --timeout=2 --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
        --header="Content-Type: application/json" \
        http://127.0.0.1:8545 2>/dev/null | grep -o "\"result\":\"0x[0-9a-f]*\"" | cut -d"\"" -f4
    ' 2>/dev/null || echo ""
}

get_onion_address() {
    local node=$1
    docker exec gethrelay-$node sh -c '
        wget -q -O - --timeout=2 --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
        --header="Content-Type: application/json" \
        http://127.0.0.1:8545 2>/dev/null | grep -o "[a-z0-9]\{56\}\.onion"
    ' 2>/dev/null || echo "not available"
}

get_trusted_peer_count() {
    local node=$1
    docker exec gethrelay-$node sh -c '
        wget -q -O - --timeout=2 --post-data='\''{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}'\'' \
        --header="Content-Type: application/json" \
        http://127.0.0.1:8545 2>/dev/null | grep -o "\"trusted\":true" | wc -l
    ' 2>/dev/null || echo "0"
}

get_container_status() {
    local container=$1
    local status=$(docker inspect --format='{{.State.Status}}' $container 2>/dev/null || echo "unknown")

    if [ "$status" = "running" ]; then
        echo -e "${GREEN}running${NC}"
    elif [ "$status" = "exited" ]; then
        echo -e "${RED}exited${NC}"
    else
        echo -e "${YELLOW}$status${NC}"
    fi
}

get_container_health() {
    local container=$1
    local health=$(docker inspect --format='{{.State.Health.Status}}' $container 2>/dev/null || echo "none")

    if [ "$health" = "healthy" ]; then
        echo -e "${GREEN}✓${NC}"
    elif [ "$health" = "unhealthy" ]; then
        echo -e "${RED}✗${NC}"
    elif [ "$health" = "starting" ]; then
        echo -e "${YELLOW}⋯${NC}"
    else
        echo -e "${CYAN}-${NC}"
    fi
}

# Main monitoring loop
while true; do
    clear_screen

    # Tor status
    echo -e "${BOLD}${CYAN}Tor Service${NC}"
    TOR_STATUS=$(get_container_status tor-proxy)
    TOR_HEALTH=$(get_container_health tor-proxy)
    echo -e "Status: $TOR_STATUS  Health: $TOR_HEALTH"
    echo ""

    # Node status table
    echo -e "${BOLD}${CYAN}Gethrelay Nodes${NC}"
    printf "%-12s %-10s %-8s %-8s %-12s %-56s\n" "NODE" "STATUS" "HEALTH" "PEERS" "TRUSTED" "ONION ADDRESS"
    echo "─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────"

    for i in 1 2 3; do
        # Get node status
        STATUS=$(get_container_status gethrelay-$i)
        HEALTH=$(get_container_health gethrelay-$i)

        # Get peer counts
        PEER_COUNT_HEX=$(get_peer_count $i)
        if [ -n "$PEER_COUNT_HEX" ]; then
            PEER_COUNT=$((${PEER_COUNT_HEX}))
            if [ "$PEER_COUNT" -ge 2 ]; then
                PEER_DISPLAY="${GREEN}$PEER_COUNT${NC}"
            elif [ "$PEER_COUNT" -ge 1 ]; then
                PEER_DISPLAY="${YELLOW}$PEER_COUNT${NC}"
            else
                PEER_DISPLAY="${RED}$PEER_COUNT${NC}"
            fi
        else
            PEER_DISPLAY="${YELLOW}?${NC}"
        fi

        # Get trusted peer count
        TRUSTED_COUNT=$(get_trusted_peer_count $i)
        if [ "$TRUSTED_COUNT" -gt 0 ]; then
            TRUSTED_DISPLAY="${GREEN}$TRUSTED_COUNT${NC}"
        else
            TRUSTED_DISPLAY="${CYAN}$TRUSTED_COUNT${NC}"
        fi

        # Get .onion address
        ONION_ADDR=$(get_onion_address $i)
        if [ "$ONION_ADDR" != "not available" ]; then
            ONION_SHORT="${ONION_ADDR:0:16}...${ONION_ADDR: -10}"
        else
            ONION_SHORT="${YELLOW}initializing...${NC}"
        fi

        printf "%-12s %-24s %-18s %-20s %-24s %-56s\n" \
            "gethrelay-$i" \
            "$STATUS" \
            "$HEALTH" \
            "$PEER_DISPLAY" \
            "$TRUSTED_DISPLAY" \
            "$ONION_SHORT"
    done

    echo ""

    # Peer managers status
    echo -e "${BOLD}${CYAN}Peer Managers${NC}"
    printf "%-18s %-10s %-30s\n" "SIDECAR" "STATUS" "RECENT ACTIVITY"
    echo "─────────────────────────────────────────────────────────────────────────────"

    for i in 1 2 3; do
        PM_STATUS=$(get_container_status peer-manager-$i)

        # Get last log line
        LAST_LOG=$(docker logs peer-manager-$i 2>&1 | tail -1 | cut -c 1-50 || echo "")

        printf "%-18s %-24s %-30s\n" \
            "peer-manager-$i" \
            "$PM_STATUS" \
            "$LAST_LOG"
    done

    echo ""

    # Resource usage
    echo -e "${BOLD}${CYAN}Resource Usage${NC}"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" \
        gethrelay-1 gethrelay-2 gethrelay-3 tor-proxy 2>/dev/null | head -4

    echo ""

    # Discovery summary
    echo -e "${BOLD}${CYAN}Discovery Summary${NC}"
    TOTAL_PEERS=0
    TOTAL_TRUSTED=0

    for i in 1 2 3; do
        PEER_COUNT_HEX=$(get_peer_count $i)
        if [ -n "$PEER_COUNT_HEX" ]; then
            PEER_COUNT=$((${PEER_COUNT_HEX}))
            TOTAL_PEERS=$((TOTAL_PEERS + PEER_COUNT))
        fi

        TRUSTED_COUNT=$(get_trusted_peer_count $i)
        TOTAL_TRUSTED=$((TOTAL_TRUSTED + TRUSTED_COUNT))
    done

    AVG_PEERS=$((TOTAL_PEERS / 3))

    echo -e "Total peer connections:  ${BOLD}$TOTAL_PEERS${NC}"
    echo -e "Average per node:        ${BOLD}$AVG_PEERS${NC}"
    echo -e "Total trusted peers:     ${BOLD}$TOTAL_TRUSTED${NC}"

    if [ "$AVG_PEERS" -ge 2 ]; then
        echo -e "\nCluster status: ${GREEN}${BOLD}Healthy${NC}"
    elif [ "$AVG_PEERS" -ge 1 ]; then
        echo -e "\nCluster status: ${YELLOW}${BOLD}Discovery in progress${NC}"
    else
        echo -e "\nCluster status: ${RED}${BOLD}No peers connected${NC}"
    fi

    echo ""
    echo -e "${CYAN}Last updated: $(date '+%Y-%m-%d %H:%M:%S')${NC}"

    # Refresh every 5 seconds
    sleep 5
done
