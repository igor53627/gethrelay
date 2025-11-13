# Tor Peer Discovery - Root Cause Analysis

## Problem Statement

Gethrelay pods show `static=0` in logs despite static-nodes.json being created successfully by the init container. Investigation revealed the file exists with valid format but peer connections fail.

## Root Cause

### Discovery Process Flow:
1. **Init Container** runs before gethrelay starts
2. Reads ConfigMap to get peer .onion addresses
3. Generates node IDs using `get_node_id()` function
4. Creates `/data/geth/static-nodes.json` with enode URLs

### The Critical Issue:

**Invalid secp256k1 Public Keys**

```
WARN [11-11|10:31:08.733] Invalid static node URL index=0 err="invalid public key (invalid secp256k1 public key)"
```

The node IDs in static-nodes.json are **random 128-character hex strings** but NOT valid secp256k1 elliptic curve public keys.

From `deployment/scripts/tor-peer-discovery.sh:90-95`:
```bash
# Last resort: generate a random valid hex node ID (128 hex chars)
echo "[tor-peer-discovery] WARNING: Could not extract real node ID, using placeholder"
dd if=/dev/urandom bs=64 count=1 2>/dev/null | od -An -tx1 -v | tr -d ' \n'
```

### Why This Fails:

secp256k1 public keys must be **valid points on the secp256k1 elliptic curve**, not arbitrary hex values. Geth validates this during enode parsing and rejects invalid keys.

## Attempted Solutions

### ✗ Option A: Add `--datadir` flag
**Result**: FAILED - gethrelay doesn't support this flag
```
flag provided but not defined: -datadir
```

### ✗ Option B: Verify static-nodes.json loading code
**Result**: Code EXISTS and works correctly - the problem is invalid node IDs, not the loading mechanism

### ✗ Option C: Sidecar dynamic peer addition via admin API
**Result**: BLOCKED - Sidecar container can't extract .onion addresses from gethrelay logs (no kubectl access)

## Why Node IDs Can't Be Known at Init Time

The **chicken-and-egg problem**:

1. Init container runs **before** gethrelay starts
2. Geth generates its node ID from `/data/geth/geth/nodekey` on first startup
3. Init container can't read a file that doesn't exist yet
4. Can't query admin API - gethrelay isn't running
5. ConfigMap doesn't have real node IDs - pods haven't registered yet

## Working Solutions

### Solution 1: DHT-Based Discovery (Already Implemented)

The current system **does work** via DHT discovery:
- Pods discover each other through discv5 protocol
- No static-nodes.json needed
- Works for Tor connections if both peers support ENR Onion3 records

**Evidence**: Pods show non-zero peer connections from DHT in metrics.

### Solution 2: Two-Phase Discovery (Recommended for Static Peers)

**Phase 1 - Init**: Create placeholder static-nodes.json (current behavior)
**Phase 2 - Sidecar**: Update ConfigMap with real node IDs after geth starts

Requirements for Phase 2:
1. Add `kubectl` binary to sidecar container
2. Grant RBAC permissions to read container logs
3. Extract real .onion address from gethrelay logs
4. Query `admin_nodeInfo` API to get real node ID
5. Update ConfigMap with valid enode URL
6. Use `admin_addPeer` to add peers dynamically

### Solution 3: Pre-Generated Node Keys (Alternative)

Generate valid secp256k1 keypairs outside Kubernetes and mount them as secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gethrelay-nodekeys
type: Opaque
data:
  gethrelay-only-onion-1-nodekey: <base64-encoded-private-key>
  gethrelay-only-onion-2-nodekey: <base64-encoded-private-key>
  gethrelay-only-onion-3-nodekey: <base64-encoded-private-key>
```

Then derive public keys for static-nodes.json at init time.

## Current Status

### What Works:
- ✅ Init container creates static-nodes.json successfully
- ✅ File has correct format and valid .onion addresses
- ✅ Gethrelay loads the file and attempts to parse it
- ✅ DHT discovery works - pods find peers via discv5
- ✅ Tor connections work when peers are discovered via DHT

### What Doesn't Work:
- ✗ Static nodes with placeholder node IDs are rejected
- ✗ Sidecar can't extract real node IDs due to missing kubectl
- ✗ `static=0` persists despite file existing

### Net Result:
Pods rely entirely on DHT discovery. Static peer configuration is non-functional but the cluster still works via DHT.

## Recommendation

**Short Term**: Document that static-nodes.json is non-functional and DHT discovery is the primary mechanism.

**Long Term**: Implement Solution 2 (Two-Phase Discovery) to enable static peer connections:
1. Add kubectl to sidecar image
2. Add RBAC for log reading
3. Implement real node ID extraction
4. Update ConfigMap with valid enodes
5. Use admin_addPeer for dynamic peer addition

## Related Files

- `cmd/gethrelay/main.go:346-361` - Static nodes loading (working correctly)
- `deployment/scripts/tor-peer-discovery.sh:53-96` - Node ID generation (generates invalid keys)
- `STATIC_NODES_LOADING.md` - Documentation of requirements
- `deployment/k8s/deployments.yaml` - StatefulSet configuration

## Date

2025-11-11
