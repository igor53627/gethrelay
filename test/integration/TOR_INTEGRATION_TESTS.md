# Tor Integration Tests for Gethrelay

This directory contains comprehensive integration tests that validate end-to-end Tor+ENR functionality for gethrelay nodes.

## Overview

The integration tests prove that two gethrelay nodes can:

1. **Discover each other via ENR** - Node A announces .onion address in ENR, Node B discovers it
2. **Connect through Tor** - Node B connects to Node A's .onion via SOCKS5 proxy
3. **Complete P2P handshake** - RLPx protocol handshake succeeds
4. **Exchange P2P messages** - Both nodes report successful peer connection

## Test Suite

### Core Integration Tests

Located in: `p2p/tor_integration_test.go`

1. **TestTorIntegration_TwoNodesDiscoverAndConnect** (MAIN TEST)
   - Proves complete ENR discovery → Tor connection flow
   - Two real Server instances with full P2P stack
   - Mock SOCKS5 proxy simulates Tor
   - Verifies peer connection establishment

2. **TestTorIntegration_ENRPropagation**
   - Verifies .onion addresses in ENR records
   - Tests ENR encoding/decoding round-trip
   - Validates .onion format (56 base32 + .onion)

3. **TestTorIntegration_DualStackConnectivity**
   - Nodes with both .onion and clearnet addresses
   - Tests connectivity via Tor and clearnet separately
   - Verifies prefer-tor mode works correctly

4. **TestTorIntegration_OnlyOnionMode**
   - Verifies only-onion mode rejects clearnet peers
   - Tests strict Tor-only operation

5. **TestTorIntegration_FallbackToClearnet**
   - Tests fallback behavior when Tor fails
   - Verifies clearnet fallback in default mode

## Running Tests

### Quick Run (Mock SOCKS5)

```bash
# Run all integration tests (uses mock SOCKS5 proxy)
go test -v -run TestTorIntegration ./p2p

# Run specific test
go test -v -run TestTorIntegration_TwoNodesDiscoverAndConnect ./p2p

# Run with timeout
go test -v -run TestTorIntegration ./p2p -timeout 2m
```

### Run with Real Tor Daemon

```bash
# Using the test runner script
./test/integration/run_tor_tests.sh --real-tor --verbose

# Or manually
tor --ControlPort 9051 --SOCKSPort 9050 &
go test -v -run TestTorIntegration ./p2p
killall tor
```

### CI/CD Integration

```bash
# Skip integration tests in short mode
go test -short ./p2p  # Skips TestTorIntegration tests

# Run all tests including integration
go test ./p2p
```

## Test Architecture

### Mock SOCKS5 Proxy

The tests include a full SOCKS5 proxy implementation (`mockSOCKS5Proxy`) that:

- Implements RFC 1928 SOCKS5 protocol
- Handles .onion address requests
- Relays data bidirectionally
- Simulates Tor circuit behavior

### Real Node Setup

Tests create real `p2p.Server` instances with:

- Full P2P stack (RLPx transport)
- ENR records with .onion addresses
- Tor dialer configuration
- Static peer connections

### Connection Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Node A (Hidden Service)                                      │
│  - Creates .onion address: abc...xyz.onion                   │
│  - Announces in ENR: onion3="abc...xyz.onion"                │
│  - Listens on 127.0.0.1:30303                                │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ ENR Discovery
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Node B (Tor Client)                                          │
│  - Discovers Node A's ENR                                    │
│  - Extracts .onion from ENR                                  │
│  - Configured with SOCKS5 proxy: 127.0.0.1:9050              │
│  - Dials abc...xyz.onion:30303 via SOCKS5                    │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ SOCKS5 Connection
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Mock SOCKS5 Proxy                                            │
│  - Receives SOCKS5 handshake                                 │
│  - Parses .onion address                                     │
│  - Maps to Node A's actual address (127.0.0.1:30303)         │
│  - Establishes TCP connection                                │
│  - Relays P2P data bidirectionally                           │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ TCP Connection
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Node A accepts connection                                    │
│  - RLPx handshake                                            │
│  - P2P protocol negotiation                                  │
│  - Peer connection established                               │
└─────────────────────────────────────────────────────────────┘
```

## Test Requirements

### System Requirements

- Go 1.21 or higher
- Network connectivity (for clearnet fallback tests)
- Optional: Tor daemon (for real Tor tests)

### Installing Tor (Optional)

```bash
# macOS
brew install tor

# Ubuntu/Debian
sudo apt-get install tor

# Arch Linux
sudo pacman -S tor

# Fedora
sudo dnf install tor
```

### Running Tor Manually

```bash
# Start Tor with custom ports
tor --ControlPort 9051 --SOCKSPort 9050 --DataDirectory /tmp/tor-test

# Verify Tor is running
curl --socks5 127.0.0.1:9050 https://check.torproject.org/api/ip
```

## Test Output Examples

### Successful Test Run

```
=== RUN   TestTorIntegration_TwoNodesDiscoverAndConnect
    tor_integration_test.go:77: Node A .onion address: abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuvw.onion
    tor_integration_test.go:99: Successfully connected: Node B -> Node A (via Tor)
    tor_integration_test.go:100:   Peer ID: a7c8d9e0f1234567890abcdef1234567890abcdef1234567890abcdef123456
    tor_integration_test.go:101:   Remote Address: 127.0.0.1:30303
    tor_integration_test.go:113: Integration test passed: Two nodes successfully connected via Tor
    tor_integration_test.go:114:   Node A peers: 1
    tor_integration_test.go:115:   Node B peers: 1
--- PASS: TestTorIntegration_TwoNodesDiscoverAndConnect (0.52s)
```

### ENR Propagation Test

```
=== RUN   TestTorIntegration_ENRPropagation
    tor_integration_test.go:150: ENR propagation verified: .onion=abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuvw.onion
    tor_integration_test.go:173: ENR encoding/decoding verified
--- PASS: TestTorIntegration_ENRPropagation (0.02s)
```

## Debugging Tests

### Enable Verbose Logging

```bash
# Set log level to trace
go test -v -run TestTorIntegration ./p2p -args -log.level=trace
```

### Check Tor Connectivity

```bash
# Verify SOCKS5 proxy is accessible
nc -zv 127.0.0.1 9050

# Test .onion resolution through Tor
curl --socks5 127.0.0.1:9050 https://www.facebookcorewwwi.onion
```

### Common Issues

1. **Timeout errors**
   - Increase test timeout: `-timeout 5m`
   - Check network connectivity
   - Verify Tor daemon is running

2. **Connection refused**
   - Ensure ports 9050/9051 are available
   - Check firewall settings
   - Verify Node A listener started successfully

3. **ENR validation errors**
   - Verify .onion format (56 base32 + .onion)
   - Check ENR signing
   - Validate ENR record structure

## Integration with Hive Tests

These integration tests complement the Hive test suite in `cmd/devp2p/internal/tortest/`:

- **Hive tests**: Mock-based scenario testing (faster, more scenarios)
- **Integration tests**: Real multi-node E2E testing (slower, real behavior)

Both test suites validate the same functionality from different angles.

## Coverage

Current integration test coverage:

- ✅ ENR propagation with .onion addresses
- ✅ SOCKS5 connectivity via Tor proxy
- ✅ Two-node P2P handshake and connection
- ✅ Dual-stack reachability (Tor + clearnet)
- ✅ Only-onion mode enforcement
- ✅ Clearnet fallback behavior
- ✅ ENR encoding/decoding round-trip

## Future Enhancements

Potential additions:

- [ ] Multi-node network (3+ nodes)
- [ ] Discovery protocol integration (discv5)
- [ ] Real Tor hidden service creation tests
- [ ] Performance benchmarks
- [ ] Network partition testing
- [ ] Tor circuit failure simulation

## Contributing

When adding new integration tests:

1. Follow the naming convention: `TestTorIntegration_<TestName>`
2. Use `testing.Short()` to allow skipping in fast CI
3. Add cleanup with `defer` to prevent resource leaks
4. Document test scenarios in comments
5. Update this README with new test descriptions

## References

- [Tor SOCKS5 Protocol](https://gitweb.torproject.org/torspec.git/tree/socks-extensions.txt)
- [Ethereum ENR Specification](https://eips.ethereum.org/EIPS/eip-778)
- [RLPx Protocol](https://github.com/ethereum/devp2p/blob/master/rlpx.md)
- [Hive Test Framework](https://github.com/ethereum/hive)

## License

Copyright 2025 The go-ethereum Authors

Licensed under the GNU Lesser General Public License v3.0
