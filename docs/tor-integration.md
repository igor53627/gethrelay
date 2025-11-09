# Tor Integration for Gethrelay

## Overview

Gethrelay supports integration with the Tor anonymity network, enabling privacy-preserving P2P connections and censorship-resistant networking. This integration allows nodes to:

- **Advertise hidden service addresses** via ENR (Ethereum Node Records)
- **Connect to peers over Tor** using SOCKS5 proxy
- **Bypass NAT without port forwarding** using Tor hidden services
- **Preserve privacy** by avoiding IP address exposure
- **Resist censorship** by routing through the Tor network

## Features

### ENR-Based Discovery

Nodes advertise their Tor hidden service address (.onion) in their ENR under the `onion3` key. This allows other Tor-enabled nodes to discover and connect to them through the Tor network.

### Flexible Connectivity Modes

Gethrelay supports three operational modes:

1. **Tor with Clearnet Fallback (Default)** - Uses Tor when available, falls back to clearnet
2. **Prefer Tor Mode** - Prioritizes .onion addresses over clearnet when both available
3. **Tor-Only Mode** - Restricts all connections to .onion addresses only

### Automatic Hidden Service Creation

When Tor is enabled, gethrelay can automatically create a hidden service for its P2P port and update the local ENR with the .onion address.

## Quick Start

### Prerequisites

1. **Tor daemon installed and running**

   ```bash
   # macOS
   brew install tor
   brew services start tor

   # Debian/Ubuntu
   sudo apt-get install tor
   sudo systemctl start tor

   # Check Tor is running
   curl --socks5 127.0.0.1:9050 https://check.torproject.org
   ```

2. **Tor SOCKS5 proxy accessible** (default: `127.0.0.1:9050`)

3. **Tor control port configured** (optional, for hidden service creation)

   Edit `/etc/tor/torrc` or `~/.tor/torrc`:
   ```
   ControlPort 9051
   CookieAuthentication 1
   ```

### Basic Configuration

**Default Mode: Tor with clearnet fallback**

```bash
gethrelay --tor-proxy=127.0.0.1:9050
```

This mode:
- Connects to peers with .onion addresses via Tor
- Falls back to clearnet if Tor connection fails
- Uses clearnet for peers without .onion addresses

**Prefer Tor Mode: Prioritize .onion addresses**

```bash
gethrelay --tor-proxy=127.0.0.1:9050 --prefer-tor
```

This mode:
- Prefers .onion addresses when both .onion and clearnet are available
- Falls back to clearnet if Tor connection fails
- Useful for maximizing privacy while maintaining connectivity

**Tor-Only Mode: Strict Tor connections only**

```bash
gethrelay --tor-proxy=127.0.0.1:9050 --only-onion
```

This mode:
- Only connects to peers with .onion addresses
- Rejects all clearnet-only peers
- No fallback to clearnet
- Maximum privacy, reduced peer pool

## Configuration Reference

### Command-Line Flags

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `--tor-proxy` | string | SOCKS5 proxy address for Tor connections | (disabled) |
| `--prefer-tor` | boolean | Prefer .onion addresses when both available | false |
| `--only-onion` | boolean | Restrict to .onion addresses only | false |

### Environment Variables

All flags can be set via environment variables with the `GETHRELAY_` prefix:

```bash
export GETHRELAY_TOR_PROXY=127.0.0.1:9050
export GETHRELAY_PREFER_TOR=true
export GETHRELAY_ONLY_ONION=false
```

### Configuration Validation

- `--only-onion` **requires** `--tor-proxy` to be set
- `--prefer-tor` has no effect without `--tor-proxy`
- Invalid SOCKS5 proxy addresses will cause startup failure

## Operational Modes Explained

### Mode 1: Tor with Clearnet Fallback (Default)

**When to use:**
- You want privacy when possible but need maximum connectivity
- You're behind NAT and want to reach Tor-enabled peers
- You want to experiment with Tor without breaking existing connections

**Behavior:**
```
Peer has .onion only → Connect via Tor
Peer has clearnet only → Connect via clearnet
Peer has both → Connect via Tor (try clearnet if Tor fails)
Tor proxy unreachable → Fallback to clearnet for all peers
```

**Example:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050 \
  --port=30303 \
  --bootnodes="enr:..."
```

### Mode 2: Prefer Tor Mode

**When to use:**
- You prioritize privacy over latency
- You're willing to accept 3-10x latency for Tor routing
- You want to avoid IP address exposure when possible

**Behavior:**
```
Peer has .onion only → Connect via Tor
Peer has clearnet only → Connect via clearnet
Peer has both → Prefer .onion (try clearnet if Tor fails)
Tor proxy unreachable → Fallback to clearnet for all peers
```

**Example:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050 \
  --prefer-tor \
  --port=30303
```

### Mode 3: Tor-Only Mode

**When to use:**
- Maximum privacy is required
- You operate in a censored network
- You only want to connect to privacy-conscious peers
- You accept a smaller peer pool

**Behavior:**
```
Peer has .onion only → Connect via Tor
Peer has clearnet only → REJECT
Peer has both → Connect via Tor (.onion)
Tor proxy unreachable → NO CONNECTIONS (fail)
```

**Example:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050 \
  --only-onion \
  --port=30303 \
  --bootnodes="enr:..." # Must include peers with .onion addresses
```

**Warning:** In Tor-only mode, you must ensure your bootnodes have .onion addresses, or you won't be able to discover any peers.

## Creating a Hidden Service

### Automatic Hidden Service (Recommended)

Gethrelay can automatically create a Tor hidden service for its P2P port and update the local ENR:

**Step 1: Configure Tor control access**

Edit `/etc/tor/torrc`:
```
ControlPort 9051
CookieAuthentication 1
```

Restart Tor:
```bash
sudo systemctl restart tor  # Linux
brew services restart tor   # macOS
```

**Step 2: Run gethrelay with Tor enabled**

```bash
gethrelay --tor-proxy=127.0.0.1:9050 \
  --port=30303
```

The hidden service will be created automatically, and the .onion address will be added to your ENR.

### Manual Hidden Service

You can also manually configure a Tor hidden service:

**Step 1: Edit torrc**

Add to `/etc/tor/torrc`:
```
HiddenServiceDir /var/lib/tor/gethrelay/
HiddenServicePort 30303 127.0.0.1:30303
```

**Step 2: Restart Tor and get .onion address**

```bash
sudo systemctl restart tor
sudo cat /var/lib/tor/gethrelay/hostname
# Output: abc...xyz.onion
```

**Step 3: Run gethrelay**

The .onion address will be automatically detected and added to your ENR if gethrelay can read the hostname file.

## Use Cases

### 1. NAT Traversal Without Port Forwarding

**Problem:** You're behind NAT and can't forward ports, limiting inbound connections.

**Solution:** Create a Tor hidden service to allow inbound connections without port forwarding.

```bash
# Tor handles NAT traversal via introduction points
gethrelay --tor-proxy=127.0.0.1:9050 --port=30303
```

Your node will be reachable via its .onion address from any Tor-enabled peer.

### 2. Censorship Circumvention

**Problem:** Your ISP or government blocks P2P connections or specific protocols.

**Solution:** Route all connections through Tor to circumvent censorship.

```bash
# Tor-only mode bypasses censorship
gethrelay --tor-proxy=127.0.0.1:9050 --only-onion
```

### 3. Privacy-Preserving P2P Networking

**Problem:** You don't want to expose your IP address to peers.

**Solution:** Use Tor-only mode to hide your IP address.

```bash
# All connections via Tor
gethrelay --tor-proxy=127.0.0.1:9050 --only-onion
```

### 4. Hybrid Operation (Privacy + Performance)

**Problem:** You want privacy but also need good connectivity and performance.

**Solution:** Use default mode with Tor fallback.

```bash
# Balance privacy and connectivity
gethrelay --tor-proxy=127.0.0.1:9050 --prefer-tor
```

This mode uses Tor when available but falls back to clearnet for better performance.

## Monitoring and Verification

### Check ENR for .onion Address

```bash
# Get your node's ENR
gethrelay admin.nodeInfo

# Decode ENR and check for onion3 key
# The output should include: onion3: "abc...xyz.onion"
```

### Verify Tor Connectivity

```bash
# Check Tor proxy is working
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Check gethrelay logs
# Should see: "P2P Tor hidden service ready" with .onion address
```

### Monitor Peer Connections

```bash
# View connected peers
gethrelay admin.peers

# Check if peers are connected via Tor
# Tor connections will show .onion addresses
```

## Performance Considerations

### Latency

- **Clearnet connections:** ~10-100ms typical
- **Tor connections:** ~300-1000ms typical (3-10x slower)

Tor routing introduces additional latency due to:
- Three-hop circuit (entry → middle → exit)
- Encryption/decryption at each hop
- Network congestion in Tor network

### Throughput

- Tor circuits are bandwidth-limited by the slowest relay
- Expect 1-10 MB/s throughput (varies by circuit quality)
- Clearnet connections typically have higher bandwidth

### Connection Establishment

- **Clearnet:** ~50-200ms TCP handshake
- **Tor:** ~1-3 seconds circuit establishment + handshake

First connection to a new .onion address takes longer as Tor builds a circuit.

### Resource Usage

- Tor proxy (separate process): ~50-100 MB memory
- Gethrelay overhead: ~1-2 MB additional memory per Tor connection
- CPU impact: minimal (~1-2% per connection for SOCKS5 routing)

## Security Considerations

See [tor-security.md](./tor-security.md) for detailed security analysis.

### Key Points

- **Dual-stack linkability:** Running both .onion and clearnet addresses can link your identities
- **Tor proxy trust:** You must trust the SOCKS5 proxy (typically localhost Tor daemon)
- **Exit relay exposure:** Outbound clearnet connections from Tor exit relays expose traffic
- **Circuit correlation:** Long-lived connections may be vulnerable to traffic analysis

## Troubleshooting

See [tor-troubleshooting.md](./tor-troubleshooting.md) for detailed troubleshooting guide.

### Common Issues

**Problem:** `failed to connect to SOCKS5 proxy`

```
Solution:
1. Check Tor is running: sudo systemctl status tor
2. Verify SOCKS5 port: netstat -an | grep 9050
3. Test SOCKS5: curl --socks5 127.0.0.1:9050 https://check.torproject.org
```

**Problem:** `only-onion mode: peer has no .onion address`

```
Solution:
1. Ensure bootnodes have .onion addresses in their ENR
2. Check peer ENRs include onion3 key
3. Consider using --prefer-tor instead of --only-onion
```

**Problem:** Slow connection establishment

```
This is expected with Tor due to circuit building.
First connection to a .onion: ~2-5 seconds
Subsequent connections: ~1-2 seconds
```

## Architecture

See [adr/001-tor-enr-integration.md](./adr/001-tor-enr-integration.md) for detailed architecture decisions.

### High-Level Design

```
┌─────────────────────────────────────────────┐
│ Gethrelay Node A                            │
│  ┌────────────────────────────────────────┐ │
│  │ P2P Server                             │ │
│  │  - Listen: 127.0.0.1:30303             │ │
│  │  - ENR: onion3="abc...xyz.onion"       │ │
│  └────────────────────────────────────────┘ │
│                                             │
│  ┌────────────────────────────────────────┐ │
│  │ Tor Hidden Service                     │ │
│  │  - .onion address: abc...xyz.onion     │ │
│  │  - Virtual port: 30303                 │ │
│  │  - Target: 127.0.0.1:30303             │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
                     │
                     │ ENR Discovery
                     ▼
┌─────────────────────────────────────────────┐
│ Gethrelay Node B                            │
│  ┌────────────────────────────────────────┐ │
│  │ P2P Client                             │ │
│  │  - Discovers Node A's ENR              │ │
│  │  - Extracts .onion address             │ │
│  │  - Uses TorDialer                      │ │
│  └────────────────────────────────────────┘ │
│                                             │
│  ┌────────────────────────────────────────┐ │
│  │ TorDialer                              │ │
│  │  - SOCKS5 proxy: 127.0.0.1:9050        │ │
│  │  - Dials: abc...xyz.onion:30303        │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
                     │
                     │ SOCKS5 Protocol
                     ▼
┌─────────────────────────────────────────────┐
│ Tor Network                                 │
│  - Circuit: Entry → Middle → Exit/RP       │
│  - Establishes connection to hidden service │
└─────────────────────────────────────────────┘
```

## Further Reading

- [Security Considerations](./tor-security.md) - Privacy and security trade-offs
- [Troubleshooting Guide](./tor-troubleshooting.md) - Common issues and solutions
- [Performance Benchmarks](./tor-performance.md) - Latency and throughput analysis
- [Deployment Guide](./tor-deployment.md) - Production deployment best practices
- [Architecture Decision Record](./adr/001-tor-enr-integration.md) - Design decisions

## FAQ

**Q: Can I run gethrelay entirely over Tor?**

A: Yes, use `--only-onion` mode to restrict all connections to Tor.

**Q: Will Tor slow down my node?**

A: Yes, Tor adds 3-10x latency. Use `--prefer-tor` for a balance, or default mode for performance.

**Q: Do I need to configure port forwarding?**

A: No, Tor hidden services work behind NAT without port forwarding.

**Q: Can peers see my IP address?**

A: In `--only-onion` mode, no. In default mode, clearnet connections expose your IP.

**Q: Is Tor required to run gethrelay?**

A: No, Tor support is optional. Gethrelay works fine without it.

**Q: Can I connect to non-Tor nodes?**

A: Yes, in default and prefer-tor modes. Only `--only-onion` restricts to Tor nodes only.

**Q: How do I know if my .onion address is working?**

A: Check logs for "P2P Tor hidden service ready" and verify your ENR includes the onion3 key.

---

**Version:** 1.0
**Last Updated:** 2025-11-09
**Status:** Stable
