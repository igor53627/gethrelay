# HTTP RPC Server for Gethrelay

## Overview

Gethrelay now supports an HTTP RPC server with admin API access, enabling ENR extraction and node management for the Tor peer discovery system.

## Command-Line Flags

### Enable HTTP RPC

```bash
--http
```

Enable the HTTP-RPC server (default: disabled)

### Configure Host

```bash
--http.addr <address>
```

HTTP-RPC server listening interface (default: `localhost`)

**Security Note:** The default binding to `localhost` ensures the RPC server is only accessible from the local machine. Only change this if you understand the security implications.

### Configure Port

```bash
--http.port <port>
```

HTTP-RPC server listening port (default: `8545`)

### Configure API Modules

```bash
--http.api <modules>
```

Comma-separated list of API modules to expose (default: `eth,net,web3`)

Available modules:
- `admin` - Node administration and ENR access
- `eth` - Ethereum protocol APIs
- `net` - Network status APIs
- `web3` - Web3 client version
- `debug` - Debugging and profiling APIs

## Usage Examples

### Basic HTTP RPC with Admin API

Enable HTTP RPC with admin API for ENR extraction:

```bash
gethrelay \
  --chain mainnet \
  --http \
  --http.addr 127.0.0.1 \
  --http.port 8545 \
  --http.api admin,eth,net
```

### Extract ENR via HTTP RPC

Once the server is running, you can extract the ENR:

```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
  http://127.0.0.1:8545
```

Response includes:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "id": "a3c9e0...",
    "name": "gethrelay/v1.0.0",
    "enode": "enode://...",
    "enr": "enr:-Jm4QC...",
    "ports": {
      "discovery": 30303,
      "listener": 30303
    },
    "protocols": {
      "eth": {...}
    }
  }
}
```

### Tor Integration with HTTP RPC

Combine Tor hidden service with HTTP RPC for secure peer discovery:

```bash
gethrelay \
  --chain mainnet \
  --tor-enabled \
  --tor-control 127.0.0.1:9051 \
  --http \
  --http.addr 127.0.0.1 \
  --http.port 8545 \
  --http.api admin
```

This enables:
1. Tor hidden service for P2P networking
2. HTTP RPC on localhost for ENR extraction
3. Admin API to retrieve node information including the .onion address in ENR

## Security Considerations

### Localhost-Only Binding

By default, the HTTP RPC server binds to `localhost` (127.0.0.1), making it accessible only from the local machine. This is the recommended configuration for security.

### API Module Selection

Only enable the API modules you need:
- For ENR extraction: `--http.api admin`
- For full functionality: `--http.api admin,eth,net,web3`

**Warning:** The `admin` API provides node management capabilities. Never expose it to untrusted networks.

### Firewall Configuration

If running in a containerized or cloud environment, ensure firewall rules prevent external access to the HTTP RPC port unless explicitly required.

## Testing

The HTTP RPC implementation includes comprehensive tests:

```bash
# Run all HTTP RPC tests
go test ./cmd/gethrelay -run TestHTTP -v

# Run integration test
go test ./cmd/gethrelay -run TestHTTPRPCIntegration -v
```

## Backward Compatibility

The legacy RPC proxy (on port 8545) remains available when `--http` is not specified, maintaining backward compatibility with existing deployments.

## Troubleshooting

### Port Already in Use

If port 8545 is already in use:

```bash
gethrelay --http --http.port 8546 ...
```

### Admin API Not Available

Ensure `admin` is included in the API list:

```bash
gethrelay --http --http.api admin,eth,net ...
```

### Cannot Connect to HTTP RPC

1. Verify the server is running: Check logs for "HTTP server started"
2. Check the listening address: Default is `localhost` (127.0.0.1)
3. Verify firewall rules allow local connections
4. Use the correct endpoint: `http://127.0.0.1:<port>`

## Environment Variables

All flags can be set via environment variables with the `GETHRELAY_` prefix:

```bash
export GETHRELAY_HTTP=true
export GETHRELAY_HTTP_ADDR=127.0.0.1
export GETHRELAY_HTTP_PORT=8545
export GETHRELAY_HTTP_API=admin,eth,net

gethrelay --chain mainnet
```
