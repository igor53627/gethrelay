# Tor+ENR P2P Integration Test Suite

Comprehensive Hive-compatible test suite for validating Tor hidden service integration with Ethereum P2P networking.

## Overview

This test suite validates the complete Tor+ENR integration across all operational modes:
- **ENR Propagation:** `.onion` address distribution via ENR records
- **SOCKS5 Connectivity:** Tor proxy-based peer connections
- **Fallback Behavior:** Clearnet fallback when Tor is unavailable
- **Operational Modes:** Default, prefer-tor, and only-onion modes
- **Dual-Stack:** Simultaneous Tor and clearnet reachability
- **Error Handling:** Graceful failure scenarios

## Test Categories

### 1. ENR Propagation Tests

**TorENRPropagation**
- Validates `.onion` address announcement in ENR
- Tests peer discovery via discovery protocol
- Verifies `Onion3` entry extraction
- **Expected:** `.onion` address correctly embedded in ENR

**TorENRValidation**
- Tests rejection of invalid `.onion` formats
- Validates base32 character restrictions
- Tests address length requirements (56 chars + ".onion")
- **Expected:** Invalid addresses rejected during ENR creation

**TorENRDiscovery**
- Validates dual-stack ENR structure
- Tests discovery protocol compatibility
- Verifies both `.onion` and clearnet addresses
- **Expected:** Nodes discoverable via both transports

### 2. SOCKS5 Connection Tests

**TorSOCKS5Connection**
- Tests successful SOCKS5 connection to `.onion`
- Validates Tor circuit establishment
- Tests peer handshake completion
- **Expected:** P2P handshake succeeds via Tor

**TorSOCKS5Handshake**
- Validates proper SOCKS5 protocol (RFC 1928)
- Tests authentication methods
- Verifies CONNECT command handling
- **Expected:** SOCKS5 handshake completes correctly

**TorConnectionTimeout**
- Tests timeout handling for slow circuits
- Validates context cancellation
- Tests graceful failure on timeout
- **Expected:** Timeout error within expected duration

### 3. Fallback Behavior Tests

**TorClearnetFallback**
- Tests fallback when Tor connection fails
- Validates fallback for dual-stack peers
- Tests automatic retry mechanism
- **Expected:** Clearnet used when Tor unavailable

**TorProxyUnavailable**
- Tests behavior when SOCKS5 proxy is down
- Validates graceful degradation
- Tests clearnet fallback activation
- **Expected:** Clearnet fallback without hanging

**TorCircuitFailure**
- Simulates Tor circuit establishment failure
- Tests SOCKS5 error code handling
- Validates retry logic
- **Expected:** Fallback to clearnet on circuit failure

### 4. Operational Mode Tests

**TorPreferMode**
- Tests prefer-tor mode (`--prefer-tor`)
- Validates `.onion` preference when available
- Tests fallback when Tor fails
- **Expected:** Tor used first, clearnet as fallback

**TorOnlyOnionMode**
- Tests only-onion mode (`--only-onion`)
- Validates acceptance of `.onion` peers
- Tests Tor-only connectivity
- **Expected:** `.onion` peers accepted

**TorOnlyOnionRejectsClearnet**
- Tests rejection of clearnet-only peers
- Validates strict Tor-only enforcement
- Tests error messages
- **Expected:** Clearnet-only peers rejected with error

### 5. Dual-Stack Tests

**TorDualStackReachability**
- Tests reachability via both Tor and clearnet
- Validates simultaneous connection capability
- Tests transport preference logic
- **Expected:** Node reachable via both transports

**TorNoDuplicateConnections**
- Tests duplicate connection prevention
- Validates single connection per peer
- Tests connection preference enforcement
- **Expected:** No duplicate connections to same peer

### 6. Error Handling Tests

**TorInvalidOnionAddress**
- Tests rejection of malformed `.onion` addresses
- Validates error messages
- Tests various invalid formats
- **Expected:** Invalid addresses rejected gracefully

**TorMalformedENR**
- Tests handling of incomplete ENRs
- Validates missing port handling
- Tests graceful error reporting
- **Expected:** Malformed ENRs handled without panic

**TorNoUsableAddresses**
- Tests peers with no valid addresses
- Validates error reporting
- Tests graceful failure
- **Expected:** Error indicating no usable addresses

## Running Tests

### Via Go Test

```bash
# Run all Tor tests
go test ./cmd/devp2p/internal/tortest -v

# Run specific test
go test ./cmd/devp2p/internal/tortest -v -run TestENRPropagation

# Run with coverage
go test ./cmd/devp2p/internal/tortest -v -cover

# Benchmarks
go test ./cmd/devp2p/internal/tortest -bench=. -benchmem
```

### Via Hive

```bash
# Build Docker image
docker build -f cmd/gethrelay/Dockerfile.gethrelay -t ethereum/gethrelay:local .

# Run Hive with Tor tests
hive --sim=devp2p --client=gethrelay:local --loglevel=5
```

### CI Integration

Tests are integrated into GitHub Actions workflow:
- **Workflow:** `.github/workflows/gethrelay-integration-tests.yml`
- **Trigger:** Push to main, PRs, manual dispatch
- **Steps:**
  1. Build Docker image
  2. Setup Hive environment
  3. Run devp2p simulator
  4. Upload test results

## Test Configuration

### Environment Variables

```bash
# Tor SOCKS5 proxy address (default: 127.0.0.1:9050)
export TOR_SOCKS_ADDR="127.0.0.1:9050"

# Test timeout (default: 10s)
export TEST_TIMEOUT="10s"

# Enable verbose logging
export HIVE_LOGLEVEL=5
```

### Mock Components

The test suite uses mock components for isolation:

**Mock SOCKS5 Server**
- Implements RFC 1928 (SOCKS5 protocol)
- Simulates Tor proxy behavior
- Supports `.onion` address handling
- Configurable failure modes

**Mock Dialer**
- Simulates clearnet connectivity
- Tracks connection attempts
- Configurable success/failure behavior

## Test Scenarios

### Scenario 1: Full Tor Integration

```
Node A (Tor hidden service):
  - ENR: { onion3: "abc...xyz.onion", tcp: 30303 }

Node B (Tor client):
  - Config: --tor-proxy=127.0.0.1:9050

Flow:
  1. B discovers A via devp2p
  2. B extracts .onion from A's ENR
  3. B connects to A via SOCKS5
  4. P2P handshake completes

Expected: Connection succeeds via Tor
```

### Scenario 2: Clearnet Fallback

```
Node A (dual-stack):
  - ENR: { onion3: "abc...xyz.onion", ip: 192.168.1.1, tcp: 30303 }

Node B (Tor unavailable):
  - Config: --tor-proxy=127.0.0.1:9050 (proxy down)

Flow:
  1. B discovers A
  2. B attempts Tor connection (fails)
  3. B falls back to clearnet
  4. Connection succeeds via clearnet

Expected: Fallback to clearnet succeeds
```

### Scenario 3: Only-Onion Mode

```
Node A (clearnet-only):
  - ENR: { ip: 192.168.1.1, tcp: 30303 }

Node B (only-onion mode):
  - Config: --only-onion

Flow:
  1. B discovers A
  2. B sees no .onion address
  3. B rejects connection

Expected: Connection rejected with error

---

Node C (Tor):
  - ENR: { onion3: "abc...xyz.onion", tcp: 30303 }

Flow:
  1. B discovers C
  2. B sees .onion address
  3. B connects via Tor

Expected: Connection accepted
```

## Implementation Details

### ENR Structure

```go
type ENRRecord struct {
    ID      string  // "v4"
    Onion3  string  // "abc...xyz.onion" (56 chars + .onion)
    IP      net.IP  // Optional clearnet IP
    TCP     uint16  // TCP port
    UDP     uint16  // Optional UDP port
}
```

### Tor Dialer Modes

```go
// Default mode: Tor with clearnet fallback
dialer := NewTorDialer(socksAddr, clearnet, false, false)

// Prefer-tor mode: Prefer Tor when available
dialer := NewTorDialer(socksAddr, clearnet, true, false)

// Only-onion mode: Reject clearnet
dialer := NewTorDialer(socksAddr, clearnet, false, true)
```

### SOCKS5 Protocol

```
Client -> Server: [version, nmethods, methods...]
Server -> Client: [version, method]

Client -> Server: [version, cmd, rsv, atyp, dst.addr, dst.port]
Server -> Client: [version, rep, rsv, atyp, bnd.addr, bnd.port]

version: 0x05 (SOCKS5)
cmd:     0x01 (CONNECT)
atyp:    0x03 (domain name for .onion)
rep:     0x00 (success)
```

## Success Criteria

- [ ] All 17 test scenarios passing
- [ ] ENR .onion propagation validated
- [ ] SOCKS5 connectivity validated
- [ ] Fallback behavior validated
- [ ] All operational modes tested
- [ ] Dual-stack functionality verified
- [ ] Error handling comprehensive
- [ ] Hive integration complete
- [ ] CI pipeline integrated
- [ ] Documentation complete

## Coverage Report

Run coverage analysis:

```bash
go test ./cmd/devp2p/internal/tortest -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**Expected Coverage:**
- ENR handling: 100%
- Dialer logic: 95%+
- Error paths: 90%+
- Integration flows: 85%+

## Troubleshooting

### Mock SOCKS5 Server Issues

```bash
# Check if port is available
lsof -i :9050

# Use alternative port
export TOR_SOCKS_ADDR="127.0.0.1:9051"
```

### ENR Validation Failures

- Verify `.onion` address is exactly 62 characters (56 + ".onion")
- Ensure only lowercase a-z and digits 2-7 in base32 part
- Check ENR signature is valid

### Connection Timeouts

- Increase timeout: `export TEST_TIMEOUT="30s"`
- Check Tor proxy is running: `curl --socks5 127.0.0.1:9050 https://check.torproject.org`
- Verify firewall allows connections

## Related Files

- **Implementation:** `p2p/tor_dialer.go`
- **ENR Types:** `p2p/enr/entries.go`
- **Unit Tests:** `p2p/tor_dialer_test.go`
- **Server Config:** `p2p/config.go`, `p2p/server.go`
- **Hidden Service:** `node/tor.go`

## References

- [RFC 1928 - SOCKS5 Protocol](https://www.rfc-editor.org/rfc/rfc1928)
- [Tor v3 Hidden Services](https://gitweb.torproject.org/torspec.git/tree/rend-spec-v3.txt)
- [Ethereum ENR Specification](https://eips.ethereum.org/EIPS/eip-778)
- [Hive Testing Framework](https://github.com/ethereum/hive)

## License

Copyright 2025 The go-ethereum Authors. Licensed under GNU GPL v3.0.
