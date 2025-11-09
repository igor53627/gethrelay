#!/bin/bash

# Deployment script for gethrelay with Tor hidden services
# This script handles the migration from Deployments to StatefulSets for tor-only and prefer-tor instances

set -e

NAMESPACE="gethrelay"
DEPLOYMENT_FILE="deployment/k8s/deployments.yaml"

echo "========================================="
echo "Gethrelay Tor Hidden Service Deployment"
echo "========================================="
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    echo "Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE"
else
    echo "Namespace $NAMESPACE already exists"
fi

echo ""
echo "Step 1: Applying ConfigMaps..."
kubectl apply -f "$DEPLOYMENT_FILE" --dry-run=client -o yaml | grep -A 100 "kind: ConfigMap" | kubectl apply -f - || true

echo ""
echo "Step 2: Checking for existing Deployments to migrate..."

# List of deployments to migrate to StatefulSets
PREFER_TOR_DEPLOYMENTS=(
    "gethrelay-prefer-tor-1"
    "gethrelay-prefer-tor-2"
    "gethrelay-prefer-tor-3"
    "gethrelay-prefer-tor-4"
)

ONLY_ONION_DEPLOYMENTS=(
    "gethrelay-only-onion-1"
    "gethrelay-only-onion-2"
    "gethrelay-only-onion-3"
)

# Function to check and delete deployment
delete_if_exists() {
    local name=$1
    if kubectl get deployment -n "$NAMESPACE" "$name" &> /dev/null; then
        echo "  - Deleting deployment: $name"
        kubectl delete deployment -n "$NAMESPACE" "$name" --wait=true
        return 0
    else
        echo "  - Deployment $name not found (skipping)"
        return 1
    fi
}

echo ""
echo "Migrating prefer-tor instances from Deployment to StatefulSet..."
for deployment in "${PREFER_TOR_DEPLOYMENTS[@]}"; do
    delete_if_exists "$deployment"
done

echo ""
echo "Migrating tor-only instances from Deployment to StatefulSet..."
for deployment in "${ONLY_ONION_DEPLOYMENTS[@]}"; do
    delete_if_exists "$deployment"
done

echo ""
echo "Step 3: Applying full deployment configuration..."
kubectl apply -f "$DEPLOYMENT_FILE"

echo ""
echo "Step 4: Waiting for resources to be ready..."
echo ""

# Wait for default deployments
echo "Waiting for default deployments..."
kubectl wait --for=condition=available --timeout=120s \
    deployment -n "$NAMESPACE" -l mode=default || true

# Wait for prefer-tor StatefulSets
echo "Waiting for prefer-tor StatefulSets..."
kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 --timeout=180s \
    statefulset -n "$NAMESPACE" -l mode=prefer-tor || true

# Wait for tor-only StatefulSets
echo "Waiting for tor-only StatefulSets..."
kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 --timeout=180s \
    statefulset -n "$NAMESPACE" -l mode=only-onion || true

echo ""
echo "========================================="
echo "Deployment Summary"
echo "========================================="
echo ""

# Show deployment status
echo "Deployments (default mode):"
kubectl get deployments -n "$NAMESPACE" -l mode=default -o wide

echo ""
echo "StatefulSets (prefer-tor mode):"
kubectl get statefulsets -n "$NAMESPACE" -l mode=prefer-tor -o wide

echo ""
echo "StatefulSets (tor-only mode):"
kubectl get statefulsets -n "$NAMESPACE" -l mode=only-onion -o wide

echo ""
echo "Persistent Volume Claims:"
kubectl get pvc -n "$NAMESPACE" -o wide

echo ""
echo "========================================="
echo "Verification Commands"
echo "========================================="
echo ""
echo "Check Tor hidden service creation:"
echo "  kubectl logs -n $NAMESPACE gethrelay-prefer-tor-1-0 -c tor | grep -i hidden"
echo ""
echo "Get .onion address for prefer-tor-1:"
echo "  kubectl exec -n $NAMESPACE gethrelay-prefer-tor-1-0 -c tor -- cat /var/lib/tor/hidden_service/hostname"
echo ""
echo "Check gethrelay ENR with .onion:"
echo "  kubectl logs -n $NAMESPACE gethrelay-prefer-tor-1-0 -c gethrelay | grep -i 'onion\|P2P Tor'"
echo ""
echo "Monitor peer connections:"
echo "  kubectl logs -n $NAMESPACE gethrelay-only-onion-1-0 -c gethrelay -f | grep -i peer"
echo ""
echo "========================================="
echo "Deployment Complete!"
echo "========================================="
echo ""
echo "Please allow 5-10 minutes for:"
echo "  - Tor circuits to establish"
echo "  - Hidden services to propagate"
echo "  - Peer discovery to complete"
echo ""
echo "For detailed verification steps, see:"
echo "  deployment/TOR_HIDDEN_SERVICE_SETUP.md"
echo ""
