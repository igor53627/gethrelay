#!/bin/bash

# Verification script for Tor hidden services in gethrelay deployment
# This script checks if hidden services are properly configured and .onion addresses are advertised

set -e

NAMESPACE="gethrelay"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "Gethrelay Tor Hidden Service Verification"
echo "========================================="
echo ""

# Function to print success
success() {
    echo -e "${GREEN}✓${NC} $1"
}

# Function to print error
error() {
    echo -e "${RED}✗${NC} $1"
}

# Function to print warning
warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to check if pod exists and is ready
check_pod() {
    local pod=$1
    if kubectl get pod -n "$NAMESPACE" "$pod" &> /dev/null; then
        if kubectl wait --for=condition=ready pod -n "$NAMESPACE" "$pod" --timeout=5s &> /dev/null; then
            return 0
        else
            return 1
        fi
    else
        return 1
    fi
}

# Function to get .onion address
get_onion_address() {
    local pod=$1
    kubectl exec -n "$NAMESPACE" "$pod" -c tor -- cat /var/lib/tor/hidden_service/hostname 2>/dev/null || echo ""
}

# Function to check if ENR contains .onion
check_enr_onion() {
    local pod=$1
    kubectl logs -n "$NAMESPACE" "$pod" -c gethrelay --tail=100 | grep -i "P2P Tor hidden service ready" | grep -o "onion=[^ ]*" | head -1 || echo ""
}

echo "Step 1: Checking namespace and resources..."
echo ""

if kubectl get namespace "$NAMESPACE" &> /dev/null; then
    success "Namespace $NAMESPACE exists"
else
    error "Namespace $NAMESPACE not found"
    exit 1
fi

# Check ConfigMaps
if kubectl get configmap -n "$NAMESPACE" torrc-basic &> /dev/null; then
    success "ConfigMap torrc-basic exists"
else
    error "ConfigMap torrc-basic not found"
fi

if kubectl get configmap -n "$NAMESPACE" torrc-with-hidden-service &> /dev/null; then
    success "ConfigMap torrc-with-hidden-service exists"
else
    error "ConfigMap torrc-with-hidden-service not found"
fi

echo ""
echo "Step 2: Checking default instances (no hidden service expected)..."
echo ""

DEFAULT_INSTANCES=("gethrelay-default-1" "gethrelay-default-2" "gethrelay-default-3")
for deployment in "${DEFAULT_INSTANCES[@]}"; do
    pods=$(kubectl get pods -n "$NAMESPACE" -l "instance=${deployment}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "$pods" ]; then
        if check_pod "$pods"; then
            success "Default instance $deployment is running"
        else
            warning "Default instance $deployment exists but not ready"
        fi
    else
        warning "Default instance $deployment not found"
    fi
done

echo ""
echo "Step 3: Checking prefer-tor instances (hidden service expected)..."
echo ""

PREFER_TOR_COUNT=0
for i in {1..4}; do
    pod="gethrelay-prefer-tor-$i-0"
    echo "Checking $pod..."

    if check_pod "$pod"; then
        success "  Pod is ready"

        # Check PVC
        pvc="tor-data-$pod"
        if kubectl get pvc -n "$NAMESPACE" "$pvc" &> /dev/null; then
            status=$(kubectl get pvc -n "$NAMESPACE" "$pvc" -o jsonpath='{.status.phase}')
            if [ "$status" = "Bound" ]; then
                success "  PVC is bound"
            else
                error "  PVC status: $status"
            fi
        else
            error "  PVC not found"
        fi

        # Check .onion address
        onion=$(get_onion_address "$pod")
        if [ -n "$onion" ] && [[ "$onion" =~ ^[a-z2-7]{56}\.onion$ ]]; then
            success "  .onion address: $onion"
            PREFER_TOR_COUNT=$((PREFER_TOR_COUNT + 1))
        else
            error "  .onion address not found or invalid"
        fi

        # Check ENR
        enr_onion=$(check_enr_onion "$pod")
        if [ -n "$enr_onion" ]; then
            success "  ENR contains .onion: $enr_onion"
        else
            warning "  .onion not yet in gethrelay logs (may need more time)"
        fi
    else
        error "  Pod not ready or not found"
    fi
    echo ""
done

echo ""
echo "Step 4: Checking tor-only instances (hidden service expected)..."
echo ""

ONLY_ONION_COUNT=0
for i in {1..3}; do
    pod="gethrelay-only-onion-$i-0"
    echo "Checking $pod..."

    if check_pod "$pod"; then
        success "  Pod is ready"

        # Check PVC
        pvc="tor-data-$pod"
        if kubectl get pvc -n "$NAMESPACE" "$pvc" &> /dev/null; then
            status=$(kubectl get pvc -n "$NAMESPACE" "$pvc" -o jsonpath='{.status.phase}')
            if [ "$status" = "Bound" ]; then
                success "  PVC is bound"
            else
                error "  PVC status: $status"
            fi
        else
            error "  PVC not found"
        fi

        # Check .onion address
        onion=$(get_onion_address "$pod")
        if [ -n "$onion" ] && [[ "$onion" =~ ^[a-z2-7]{56}\.onion$ ]]; then
            success "  .onion address: $onion"
            ONLY_ONION_COUNT=$((ONLY_ONION_COUNT + 1))
        else
            error "  .onion address not found or invalid"
        fi

        # Check ENR
        enr_onion=$(check_enr_onion "$pod")
        if [ -n "$enr_onion" ]; then
            success "  ENR contains .onion: $enr_onion"
        else
            warning "  .onion not yet in gethrelay logs (may need more time)"
        fi
    else
        error "  Pod not ready or not found"
    fi
    echo ""
done

echo ""
echo "========================================="
echo "Summary"
echo "========================================="
echo ""

echo "Prefer-Tor instances with .onion: $PREFER_TOR_COUNT / 4"
echo "Tor-Only instances with .onion: $ONLY_ONION_COUNT / 3"

if [ $PREFER_TOR_COUNT -eq 4 ] && [ $ONLY_ONION_COUNT -eq 3 ]; then
    echo ""
    success "All instances have valid .onion addresses!"
elif [ $PREFER_TOR_COUNT -gt 0 ] || [ $ONLY_ONION_COUNT -gt 0 ]; then
    echo ""
    warning "Some instances have .onion addresses. Others may need more time."
    warning "Tor circuit establishment can take 30-60 seconds."
else
    echo ""
    error "No .onion addresses found. Check Tor container logs."
fi

echo ""
echo "========================================="
echo "Detailed Logs"
echo "========================================="
echo ""
echo "To view detailed logs for a specific instance:"
echo "  kubectl logs -n $NAMESPACE gethrelay-prefer-tor-1-0 -c tor"
echo "  kubectl logs -n $NAMESPACE gethrelay-prefer-tor-1-0 -c gethrelay"
echo ""
echo "To check peer connections:"
echo "  kubectl logs -n $NAMESPACE gethrelay-only-onion-1-0 -c gethrelay | grep -i peer"
echo ""
echo "To test .onion persistence (after pod restart):"
echo "  kubectl delete pod -n $NAMESPACE gethrelay-prefer-tor-1-0"
echo "  kubectl wait --for=condition=ready pod -n $NAMESPACE gethrelay-prefer-tor-1-0 --timeout=120s"
echo "  kubectl exec -n $NAMESPACE gethrelay-prefer-tor-1-0 -c tor -- cat /var/lib/tor/hidden_service/hostname"
echo ""
