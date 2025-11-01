# gethrelay - Ethereum P2P Relay Node with JSON-RPC Proxy

`gethrelay` is a lightweight Ethereum P2P relay node that forwards blockchain messages between peers without maintaining full blockchain state. It includes JSON-RPC proxy functionality that accepts `eth_sendRawTransaction` locally while proxying all other RPC requests to an upstream endpoint.

## Features

### P2P Relay Functionality
- **Lightweight Node**: Operates without storing full blockchain state
- **Message Forwarding**: Relays ETH protocol messages between peers
- **Block Range Support**: Configurable block range for handshake compatibility
- **Multiple Network Support**: Mainnet, Holesky, Sepolia, and custom networks

### JSON-RPC Proxy
- **Default JSON-RPC Server**: Enabled on port 8545 by default
- **Local Transaction Handling**: Accepts `eth_sendRawTransaction` requests locally
- **Upstream Proxying**: Routes all other RPC requests to configurable upstream endpoint
- **Configurable Upstream**: Default upstream is `https://ethereum-rpc.publicnode.com`, configurable via flag

## Installation

Build from source:

```bash
cd cmd/gethrelay
go build -o gethrelay
```

Or build as part of the go-ethereum suite:

```bash
make gethrelay
```

## Usage

### Basic Usage

Start a relay node on mainnet:

```bash
./gethrelay --chain mainnet
```

Start with custom upstream RPC:

```bash
./gethrelay --chain mainnet --rpc.upstream https://your-upstream-rpc.com
```

### Complete Example

```bash
./gethrelay \
  --chain mainnet \
  --port 30303 \
  --maxpeers 200 \
  --identity "my-relay-node" \
  --rpc.upstream https://ethereum-rpc.publicnode.com
```

## Command Line Options

### Network Configuration
- `--chain`: Chain preset (mainnet, holesky, sepolia, default: mainnet)
- `--networkid`: Network identifier (1=Mainnet, 17000=Holesky, 11155111=Sepolia)
- `--port`: Network listening port (default: 30303)
- `--genesis`: Genesis block hash (optional override)

### JSON-RPC Proxy
- `--rpc.upstream`: Upstream RPC endpoint URL (default: https://ethereum-rpc.publicnode.com)

### Other Options
- `--maxpeers`: Maximum number of network peers (default: 200)
- `--bootnodes`: Comma-separated list of bootstrap nodes
- `--v4disc`: Enable discv4 discovery
- `--v5disc`: Enable discv5 discovery
- `--nodiscover`: Disable peer discovery
- `--earliest-block`: Earliest available block number
- `--latest-block`: Latest available block number
- `--latest-hash`: Latest block hash (hex)
- `--nat`: NAT port mapping mechanism
- `--netrestrict`: Restrict network communication to given IP networks (CIDR masks)
- `--identity`: Custom node name

See `./gethrelay --help` for all options.

## Architecture

### P2P Relay Layer
```
Peer A <---> gethrelay <---> Peer B
              (forwards messages)
```

The relay node:
1. Establishes P2P connections with multiple peers
2. Forwards ETH protocol messages (blocks, transactions, etc.)
3. Proxies P2P requests (block headers, bodies, receipts)
4. Does NOT maintain blockchain state or validate blocks

### JSON-RPC Proxy Layer
```
Client <---> gethrelay:8545 <---> Upstream RPC
              (local: eth_sendRawTransaction)
              (proxy: all other methods)
```

The RPC proxy:
1. Accepts JSON-RPC requests on port 8545
2. Intercepts `eth_sendRawTransaction` and handles locally
3. Forwards all other requests to upstream endpoint
4. Supports single and batch JSON-RPC requests

## Implementation Details

### Files
- `main.go`: Main entry point and CLI configuration
- `rpc_setup.go`: RPC server setup and eth API implementation
- `rpc_proxy.go`: RPC proxy handler that routes requests
- `protocols.go`: Protocol registration for P2P

### RPC Request Flow

1. **Single Request - eth_sendRawTransaction**:
   ```
   Client → gethrelay → Local Handler → Upstream RPC → Response → Client
   ```

2. **Single Request - Other Method**:
   ```
   Client → gethrelay → Upstream RPC → Response → Client
   ```

3. **Batch Request** (mixed):
   ```
   Client → gethrelay → Split Requests
                    ├─→ Local Handler (eth_sendRawTransaction)
                    └─→ Upstream RPC (other methods)
                    → Merge Responses → Client
   ```

## Testing

### Unit Tests

Run unit tests:
```bash
cd cmd/gethrelay
go test -v .
```

### Integration Tests with Hive

Run Hive integration tests:
```bash
bash cmd/gethrelay/test-hive.sh
```

Or manually:
```bash
# Build Docker image
docker build -f cmd/gethrelay/Dockerfile.gethrelay -t ethereum/gethrelay:local .

# Run Hive tests (if Hive is installed)
hive --client=gethrelay:local --sim=devp2p --sim=ethereum/rpc
```

## Limitations

1. **No Blockchain State**: The relay does not maintain blockchain state, so it cannot:
   - Answer queries about account balances
   - Execute contract calls locally
   - Provide historical state access

2. **Transaction Broadcasting**: Currently, `eth_sendRawTransaction` is forwarded to upstream. Future versions may support P2P broadcasting.

3. **Single Upstream**: Only one upstream RPC endpoint is supported (no failover).

## Future Enhancements

- [ ] P2P transaction broadcasting for `eth_sendRawTransaction`
- [ ] Multiple upstream endpoints with failover
- [ ] Metrics and monitoring endpoints
- [ ] WebSocket RPC support
- [ ] Rate limiting and request throttling

## License

This code is part of the go-ethereum library and is licensed under the GNU Lesser General Public License v3.0.

## References

- [Ethereum P2P Relay Architecture](../../eth/relay)
- [Hive Test Framework](https://github.com/ethereum/hive)
- [Ethereum JSON-RPC Specification](https://ethereum.org/en/developers/docs/apis/json-rpc/)

