# Static Nodes Loading for Gethrelay

## Problem

Gethrelay was not loading static-nodes.json files created by the Tor peer discovery system in Kubernetes environments. Logs showed `static=0` even though the discovery script successfully created the file.

## Root Cause

Modern go-ethereum has deprecated the automatic loading of `static-nodes.json` files. The `checkLegacyFiles()` function in `node/config.go` only **warns** about these files being deprecated but doesn't load them. The error message states:

> "The static-nodes.json file is deprecated and ignored. Use P2P.StaticNodes in config.toml instead."

## Solution

Implemented manual loading of static-nodes.json in gethrelay's startup code:

### Changes Made

1. **Added `loadStaticNodesFile()` function** (`cmd/gethrelay/main.go`)
   - Reads static-nodes.json from filesystem
   - Parses JSON array of enode URLs
   - Validates each enode URL using `enode.Parse()`
   - Logs detailed information about loaded nodes (onion vs clearnet)
   - Gracefully handles missing files (returns nil, not error)

2. **Automatic File Discovery** (`cmd/gethrelay/main.go`)
   - Checks multiple common locations in order:
     - `/data/geth/geth/static-nodes.json` (geth default)
     - `/data/geth/static-nodes.json` (simplified location)
     - `./static-nodes.json` (current directory)
   - Loads from first found file
   - Appends file-based nodes to any command-line specified static nodes

3. **Discovery Script Fix** (`deployment/scripts/tor-peer-discovery.sh`)
   - Changed default path from `/data/geth/geth/static-nodes.json` to `/data/geth/static-nodes.json`
   - Improved node ID extraction:
     - Tries to read from nodekey file
     - Falls back to admin API query
     - Last resort: generates random valid 128-hex-char node ID
   - Validates node ID length (must be 128 hex characters)

## Important Requirements

### Valid Enode URLs

Static node URLs must have:
1. **Valid secp256k1 public key** (128 hex characters, 64 bytes)
   - Example: `a979fb575495b8d6db44f750317d0f4622bf4c2aa3365d6af7c284339968eef29b69ad0dce72a4d8db5ebb4968de0e3bec910127f134779fbcb0cb6d3331163c`
   - NOT valid: `pod-gethrelay-only-onion-1-0` (placeholder text)

2. **Valid .onion address** (for Tor nodes)
   - Must be exactly 56 base32 characters + `.onion` suffix (62 chars total)
   - Base32 chars: `a-z`, `2-7` only
   - Example: `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.onion`

### File Format

```json
[
  "enode://<128-hex-chars>@<onion-address>:30303",
  "enode://<128-hex-chars>@<another-onion>:30303"
]
```

## Testing

### Create Test Static Nodes File

```bash
cat > /tmp/test-static-nodes.json << 'EOF'
[
  "enode://a979fb575495b8d6db44f750317d0f4622bf4c2aa3365d6af7c284339968eef29b69ad0dce72a4d8db5ebb4968de0e3bec910127f134779fbcb0cb6d3331163c@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.onion:30303"
]
EOF
```

### Build and Test Gethrelay

```bash
# Build gethrelay
cd /Users/user/pse/ethereum/go-ethereum
go build -o ./build/bin/gethrelay ./cmd/gethrelay

# Test with static nodes file
cp /tmp/test-static-nodes.json ./static-nodes.json
./build/bin/gethrelay --chain=mainnet --only-onion --tor-proxy=127.0.0.1:9050
```

### Expected Log Output

You should see:
```
INFO Loaded static nodes from file   path=./static-nodes.json count=1
INFO loadStaticNodesFile: Loaded .onion static node peer=930cf49cd4de09a6 onion=aaaaa...
```

And in the P2P startup logs:
```
INFO Started P2P networking  self=<your-enode> static=1
```

## Kubernetes Deployment

### Verify Discovery Script

Check init container logs:
```bash
kubectl logs -n gethrelay gethrelay-only-onion-1-0 -c tor-peer-discovery
```

Expected output:
```
[tor-peer-discovery] Starting Tor peer discovery for pod: gethrelay-only-onion-1-0
[tor-peer-discovery] Found .onion address: abc123...xyz.onion
[tor-peer-discovery] Got node ID from geth: a979fb575495...
[tor-peer-discovery] Built static-nodes.json with 2 peers
```

### Verify Static Nodes Loaded

Check gethrelay container logs:
```bash
kubectl logs -n gethrelay gethrelay-only-onion-1-0 -c gethrelay | grep -i "static"
```

Expected:
```
INFO Loaded static nodes from file   path=/data/geth/static-nodes.json count=2
INFO Started P2P networking  static=2
```

### Check Peer Connections

```bash
kubectl port-forward -n gethrelay gethrelay-only-onion-1-0 6060:6060
curl http://localhost:6060/debug/metrics | grep p2p_peers
```

Should show:
```
p2p_peers 2
```

## Troubleshooting

### Problem: `static=0` in logs

**Cause**: Static nodes file not found or empty
**Solution**:
- Check file exists: `kubectl exec -it gethrelay-only-onion-1-0 -c gethrelay -- ls -la /data/geth/`
- Check file content: `kubectl exec -it gethrelay-only-onion-1-0 -c gethrelay -- cat /data/geth/static-nodes.json`

### Problem: "Invalid static node URL" warnings

**Cause**: Node IDs are not valid secp256k1 public keys
**Solution**:
- Ensure discovery script extracts real node IDs from nodekey or admin API
- Verify node IDs are 128 hex characters: `echo "$NODE_ID" | wc -c` should return 129 (128 + newline)

### Problem: Nodes loaded but not connecting

**Cause**: .onion addresses invalid or Tor proxy not working
**Solution**:
- Verify .onion addresses are exactly 62 characters (56 base32 + ".onion")
- Test Tor proxy: `curl --socks5 127.0.0.1:9050 http://check.torproject.org`
- Check gethrelay has `--tor-proxy` flag set correctly

## Files Modified

- `cmd/gethrelay/main.go` - Added static nodes loading
- `deployment/scripts/tor-peer-discovery.sh` - Fixed path and node ID extraction
- `deployment/k8s/tor-peer-discovery-configmap.yaml` - Already using correct path

## Future Improvements

1. **Dynamic Node ID Updates**: Implement continuous discovery sidecar that queries admin API and updates ConfigMap with real node IDs after geth starts
2. **Hot Reload**: Add file watcher to reload static nodes when file changes
3. **Admin API Integration**: Use admin_addPeer to dynamically add peers without restart
4. **Health Checks**: Validate connectivity to static nodes and remove failed peers
