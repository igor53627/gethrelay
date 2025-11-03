# gethrelay - Ethereum P2P Relay Node

A lightweight Ethereum P2P relay node that forwards blockchain messages between peers without maintaining full blockchain state. Includes JSON-RPC proxy functionality for transaction handling.

[![Go Report Card](https://goreportcard.com/badge/github.com/igor53627/gethrelay)](https://goreportcard.com/report/github.com/igor53627/gethrelay)
[![CI](https://github.com/igor53627/gethrelay/actions/workflows/gethrelay-tests.yml/badge.svg)](https://github.com/igor53627/gethrelay/actions/workflows/gethrelay-tests.yml)

## Overview

`gethrelay` is a specialized Ethereum node implementation focused on:

- **P2P Relay**: Forwards ETH protocol messages between peers without storing blockchain state
- **RPC Proxy**: Accepts `eth_sendRawTransaction` locally while proxying other RPC requests upstream
- **Lightweight**: Minimal resource requirements compared to full nodes

## Features

### P2P Relay Functionality
- Operates without storing full blockchain state
- Relays ETH protocol messages between peers
- Configurable block range for handshake compatibility
- Multiple network support: Mainnet, Holesky, Sepolia, and custom networks

### JSON-RPC Proxy
- JSON-RPC server enabled on port 8545 by default
- Local handling of `eth_sendRawTransaction` requests
- Upstream proxying for all other RPC methods
- Configurable upstream endpoint (default: `https://ethereum-rpc.publicnode.com`)

### Tor Hidden RPC (experimental)
- Optional Tor hidden-service exposure for HTTP/WS RPC endpoints
- Onion address persisted under the datadir for stable URLs across restarts
- Address announced via logs, stdout, and the `GETH_TOR_ONION` environment variable
- GitHub Actions integration test spins up a Tor sidecar (built from source) and verifies RPC reachability through SOCKS

## Quick Start

### Building

```bash
# Build gethrelay
make gethrelay

# Or build manually
cd cmd/gethrelay
go build -o gethrelay
```

### Running

Start a relay node on mainnet:

```bash
./gethrelay --chain mainnet
```

Start with custom upstream RPC:

```bash
./gethrelay --chain mainnet --rpc.upstream https://your-upstream-rpc.com
```

See the [complete documentation](cmd/gethrelay/README.md) for all options.

## Installation

For prerequisites and detailed build instructions, see the [gethrelay documentation](cmd/gethrelay/README.md).

Building `gethrelay` requires Go (version 1.24 or later) and a C compiler.

## Usage

### Basic Relay Node

```bash
./gethrelay \
  --chain mainnet \
  --port 30303 \
  --maxpeers 200 \
  --identity "my-relay-node" \
  --rpc.upstream https://ethereum-rpc.publicnode.com
```

### Testnet Support

```bash
# Holesky testnet
./gethrelay --chain holesky

# Sepolia testnet  
./gethrelay --chain sepolia
```

## Command Line Options

See `./gethrelay --help` for complete options. Key flags:

- `--chain`: Chain preset (mainnet, holesky, sepolia)
- `--rpc.upstream`: Upstream RPC endpoint URL
- `--port`: Network listening port (default: 30303)
- `--maxpeers`: Maximum number of peers (default: 200)
- `--nodiscover`: Disable peer discovery

## Testing

### Unit Tests

```bash
make gethrelay-test
# or
cd cmd/gethrelay && go test -v .
```

### Hive Integration Tests

```bash
make gethrelay-hive
# or
./cmd/gethrelay/test-hive.sh
```

See [CI/CD documentation](cmd/gethrelay/README-CI.md) for details.

## Docker

Build Docker image:

```bash
make gethrelay-docker
# or
docker build -f cmd/gethrelay/Dockerfile.gethrelay -t ethereum/gethrelay:latest .
```

Run with Docker Compose:

```bash
cd cmd/gethrelay
docker-compose up
```

## Documentation

- **[gethrelay README](cmd/gethrelay/README.md)** - Complete documentation
- **[CI/CD Guide](cmd/gethrelay/README-CI.md)** - Docker and GitHub Actions setup
- **[Cleanup Summary](cmd/gethrelay/CLEANUP-SUMMARY.md)** - Codebase cleanup details
- **[Codebase Size](cmd/gethrelay/CODEBASE-SIZE.md)** - Lines of code analysis

## Architecture

gethrelay consists of:

- **P2P Relay Backend** (`eth/relay/`) - Message forwarding between peers
- **RPC Proxy** (`cmd/gethrelay/rpc_proxy.go`) - Request routing and handling
- **Protocol Handlers** (`eth/protocols/eth/`) - ETH protocol message handling

## Tor sidecar & hidden RPC

- Tor Dockerfile: `docker/tor/Dockerfile`, built from official Tor sources with BuildKit caching.
- Published image tag: `ghcr.io/<your-org>/geth-tor-hidden-service:latest` (built via `.github/workflows/tor-image.yml`).
- Sidecar exposes `SOCKSPort 9150` and `ControlPort 9051` with cookie authentication written to `/data/control_auth_cookie`.
- Share the `/data` volume with gethrelay so `tor/control_auth_cookie` (relative to the datadir) is available for control-port authentication.
- When `Tor.Enabled` is true in `node.Config`, the node provisions a v3 onion service for the configured HTTP/WS endpoints, persists the private key, and publishes the address via stdout/logs and `GETH_TOR_ONION`.
- Integration test: `go test ./tests/torhidden -run TestHiddenServiceIntegration` (requires `TOR_INTEGRATION_TEST=1` and `GETH_TOR_IMAGE` to be set to the pulled image).

## Contributing

This is a focused fork of go-ethereum optimized for relay node functionality. The codebase has been cleaned to include only:

- `cmd/gethrelay/` - Main relay binary
- `cmd/devp2p/` - P2P testing tools (for Hive)
- Essential dependencies: `core/`, `eth/`, `p2p/`, `node/`, `rpc/`

## License

The go-ethereum library (i.e., all code outside of the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in our repository in the `COPYING.LESSER` file.

The gethrelay binary (i.e., all code inside of the `cmd/gethrelay` directory) is licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also
included in our repository in the `COPYING` file.
