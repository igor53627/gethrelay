#!/bin/bash
# Deploy Tor Peer Discovery System
# This script sets up Kubernetes-native peer discovery for only-onion gethrelay nodes

set -e

KUBECONFIG="${KUBECONFIG:-kubeconfig.yaml}"
NAMESPACE="gethrelay"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="${SCRIPT_DIR}/../k8s"

echo "=== Deploying Tor Peer Discovery System ==="
echo "Namespace: ${NAMESPACE}"
echo "KUBECONFIG: ${KUBECONFIG}"
echo ""

# Apply RBAC (ServiceAccount, Role, RoleBinding)
echo "[1/5] Applying RBAC configuration..."
kubectl apply -f "${K8S_DIR}/tor-peer-discovery-rbac.yaml"

# Apply discovery script ConfigMap
echo "[2/5] Applying discovery script ConfigMap..."
kubectl apply -f "${K8S_DIR}/tor-peer-discovery-configmap.yaml"

# Delete existing StatefulSets to force recreation with new init containers
echo "[3/5] Deleting existing only-onion StatefulSets..."
kubectl delete statefulset -n ${NAMESPACE} \
  gethrelay-only-onion-1 \
  gethrelay-only-onion-2 \
  gethrelay-only-onion-3 \
  --ignore-not-found=true

# Wait for pods to terminate
echo "[4/5] Waiting for pods to terminate..."
sleep 10

# Recreate StatefulSets with updated deployments.yaml
echo "[5/5] Recreating StatefulSets with discovery init container..."
kubectl apply -f "${K8S_DIR}/deployments.yaml"

echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Waiting for pods to become ready..."
kubectl wait --for=condition=ready pod -l mode=only-onion -n ${NAMESPACE} --timeout=300s || true

echo ""
echo "=== Pod Status ==="
kubectl get pods -n ${NAMESPACE} -l mode=only-onion

echo ""
echo "=== Checking Discovery ConfigMap ==="
sleep 20  # Give time for init containers to run
kubectl get configmap tor-peer-addresses -n ${NAMESPACE} -o yaml || echo "ConfigMap not created yet"

echo ""
echo "=== Checking Init Container Logs ==="
for POD in gethrelay-only-onion-1-0 gethrelay-only-onion-2-0 gethrelay-only-onion-3-0; do
  echo "--- Pod: ${POD} ---"
  kubectl logs -n ${NAMESPACE} ${POD} -c tor-peer-discovery || echo "Pod not ready yet"
  echo ""
done

echo ""
echo "=== Next Steps ==="
echo "1. Monitor pod logs: kubectl logs -n ${NAMESPACE} gethrelay-only-onion-1-0 -c gethrelay -f"
echo "2. Check ConfigMap: kubectl get configmap tor-peer-addresses -n ${NAMESPACE} -o yaml"
echo "3. Verify peer connections: kubectl port-forward -n ${NAMESPACE} gethrelay-only-onion-1-0 6060:6060"
echo "   Then: curl http://localhost:6060/debug/metrics | grep p2p_peers"
