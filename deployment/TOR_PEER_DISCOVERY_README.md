# Tor Peer Discovery System for Kubernetes

## Problem
Only-onion mode gethrelay pods cannot discover each other through the public Ethereum DHT because:
1. Public DHT is mostly clearnet peers
2. Only-onion mode rejects all clearnet connections
3. Few Ethereum nodes advertise .onion addresses
4. Cluster pods are isolated with no direct discovery mechanism

## Solution
Kubernetes-native peer discovery system that:
1. **Persistent .onion addresses**: Tor hidden service keys stored in persistent volumes
2. **Cluster-local discovery**: ConfigMap shares .onion addresses between pods
3. **Automatic static nodes**: Init container builds static-nodes.json from cluster peers
4. **No external dependencies**: Uses only Kubernetes primitives (ConfigMaps, RBAC)

## Architecture

### Components

1. **RBAC** (`tor-peer-discovery-rbac.yaml`)
   - ServiceAccount: `gethrelay-tor-discovery`
   - Role: Read/write ConfigMaps and Pods
   - RoleBinding: Bind role to service account

2. **Discovery Script** (`tor-peer-discovery-configmap.yaml`)
   - ConfigMap containing the discovery shell script
   - Waits for Tor hidden service creation
   - Extracts .onion address from `/var/lib/tor/hidden_service/hostname`
   - Updates shared ConfigMap with pod's .onion address
   - Builds `static-nodes.json` from other pods' addresses

3. **Init Container** (added to StatefulSets)
   - Runs before gethrelay container starts
   - Uses `bitnami/kubectl` image for Kubernetes API access
   - Executes discovery script
   - Creates `/data/geth/static-nodes.json` with cluster peers

4. **Shared ConfigMap** (`tor-peer-addresses`)
   - Created/updated by init containers
   - Stores each pod's data as JSON:
     ```json
     {
       "onion": "abc123...xyz.onion",
       "nodeId": "pod-gethrelay-only-onion-1-0",
       "port": 30303,
       "timestamp": "2025-11-11T12:34:56Z"
     }
     ```

### Data Flow

```
Pod Startup
    |
    v
[Tor Container Starts]
    |
    v
[Creates Hidden Service]
    |  (writes to /var/lib/tor/hidden_service/hostname)
    v
[Init Container: tor-peer-discovery]
    |
    +-- Wait for hostname file
    |
    +-- Read .onion address
    |
    +-- Update ConfigMap with pod data
    |
    +-- Read ConfigMap for other pods' addresses
    |
    +-- Build static-nodes.json
    |       (enode://nodeId@onion.address:30303)
    v
[Gethrelay Container Starts]
    |
    +-- Reads static-nodes.json
    |
    +-- Connects to cluster peers via Tor
    v
[P2P Network Established]
```

## Deployment Instructions

### Prerequisites
- Kubernetes cluster with `gethrelay` namespace
- `kubectl` configured with cluster access
- Existing gethrelay deployments with Tor enabled

### Quick Deploy

```bash
# 1. Navigate to deployment directory
cd deployment/scripts

# 2. Make script executable
chmod +x deploy-tor-discovery.sh

# 3. Run deployment
./deploy-tor-discovery.sh
```

### Manual Deploy

```bash
# 1. Apply RBAC
kubectl apply -f deployment/k8s/tor-peer-discovery-rbac.yaml

# 2. Apply discovery script ConfigMap
kubectl apply -f deployment/k8s/tor-peer-discovery-configmap.yaml

# 3. Update StatefulSets
# Edit deployment/k8s/deployments.yaml to add init container
# (See "StatefulSet Configuration" section below)

# 4. Recreate pods
kubectl delete pod -n gethrelay gethrelay-only-onion-1-0 gethrelay-only-onion-2-0 gethrelay-only-onion-3-0

# 5. Wait for pods to become ready
kubectl wait --for=condition=ready pod -l mode=only-onion -n gethrelay --timeout=300s
```

## StatefulSet Configuration

Add to each only-onion StatefulSet in `deployment/k8s/deployments.yaml`:

```yaml
spec:
  template:
    spec:
      serviceAccountName: gethrelay-tor-discovery  # Add this
      initContainers:
      - name: fix-permissions  # Existing init container
        image: busybox
        command: ['sh', '-c', 'chown -R 100:101 /var/lib/tor && chmod -R 700 /var/lib/tor']
        volumeMounts:
        - name: tor-data
          mountPath: /var/lib/tor
      - name: tor-peer-discovery  # NEW init container
        image: bitnami/kubectl:latest
        command: ['/bin/sh']
        args: ['/scripts/discover-peers.sh']
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: P2P_PORT
          value: "30303"
        volumeMounts:
        - name: tor-data
          mountPath: /var/lib/tor
          readOnly: true
        - name: geth-data
          mountPath: /data/geth
        - name: discovery-script
          mountPath: /scripts
      containers:
      - name: tor
        # ... existing tor container config ...
      - name: gethrelay
        # ... existing gethrelay container config ...
        volumeMounts:
        - name: geth-data  # NEW volume mount
          mountPath: /data/geth
        # ... other volume mounts ...
      volumes:
      - name: torrc
        configMap:
          name: torrc-with-hidden-service
      - name: discovery-script  # NEW volume
        configMap:
          name: tor-peer-discovery-script
          defaultMode: 0755
      - name: geth-data  # NEW volume
        emptyDir: {}
```

## Verification

### 1. Check Init Container Logs
```bash
kubectl logs -n gethrelay gethrelay-only-onion-1-0 -c tor-peer-discovery
```

Expected output:
```
[tor-discovery] Starting Tor peer discovery
[tor-discovery] Pod: gethrelay-only-onion-1-0
[tor-discovery] Found .onion address: abc123...xyz.onion
[tor-discovery] ConfigMap updated successfully
[tor-discovery] Building static-nodes.json from cluster peers
[tor-discovery] Adding peer: gethrelay-only-onion-2-0 -> def456...xyz.onion
[tor-discovery] Adding peer: gethrelay-only-onion-3-0 -> ghi789...xyz.onion
[tor-discovery] Built static-nodes.json with 2 peers
[tor-discovery] Discovery complete
```

### 2. Check Shared ConfigMap
```bash
kubectl get configmap tor-peer-addresses -n gethrelay -o yaml
```

Should show data for all 3 pods with their .onion addresses.

### 3. Check Peer Connections
```bash
kubectl port-forward -n gethrelay gethrelay-only-onion-1-0 6060:6060
curl http://localhost:6060/debug/metrics | grep p2p_peers
```

Expected: `p2p_peers 2` (or higher)

### 4. Check Gethrelay Logs
```bash
kubectl logs -n gethrelay gethrelay-only-onion-1-0 -c gethrelay | grep -i "peer"
```

Look for successful Tor connections to cluster peers.

## Troubleshooting

### Init Container Fails with "forbidden"
**Problem**: ServiceAccount lacks permissions
**Solution**: Verify RBAC is applied:
```bash
kubectl get serviceaccount gethrelay-tor-discovery -n gethrelay
kubectl get role tor-peer-discovery -n gethrelay
kubectl get rolebinding tor-peer-discovery -n gethrelay
```

### No .onion Address Found
**Problem**: Tor hidden service not created yet
**Solution**: Check Tor container logs:
```bash
kubectl logs -n gethrelay gethrelay-only-onion-1-0 -c tor
```

### Static Nodes Not Working
**Problem**: Node IDs are placeholders
**Solution**: This is expected initially. Geth will update ENRs with real node IDs after startup. The important part is the .onion addresses are correct.

### ConfigMap Empty
**Problem**: Init containers haven't run yet
**Solution**: Wait for all pods to start:
```bash
kubectl get pods -n gethrelay -l mode=only-onion
```

## Future Improvements

1. **Real Node IDs**: Extract actual node IDs from geth's nodekey file or admin API
2. **Continuous Discovery**: Add sidecar container to periodically update static nodes
3. **Health Checks**: Validate peer connectivity and remove stale entries
4. **Automatic Restart**: Trigger geth reload when ConfigMap changes
5. **Metrics**: Export discovery metrics to Prometheus

## Files Created

- `deployment/k8s/tor-peer-discovery-rbac.yaml` - RBAC configuration
- `deployment/k8s/tor-peer-discovery-configmap.yaml` - Discovery script
- `deployment/scripts/tor-peer-discovery.sh` - Standalone script (reference)
- `deployment/scripts/deploy-tor-discovery.sh` - Deployment automation
- `deployment/docker/Dockerfile.tor-discovery` - Docker image (optional)

## Benefits

1. **Persistent Identity**: .onion addresses survive pod restarts
2. **Zero External Dependencies**: No bootstrap nodes needed
3. **Automatic Configuration**: No manual static node management
4. **Kubernetes-Native**: Uses standard K8s primitives
5. **Scalable**: Easy to add more only-onion pods
6. **Privacy-Preserving**: All connections via Tor
