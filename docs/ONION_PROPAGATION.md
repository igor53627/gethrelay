# How .onion Addresses Propagate in Ethereum P2P Network

## Overview

.onion addresses propagate through Ethereum's standard peer discovery mechanism via **ENR (Ethereum Node Record)** entries, using the `onion3` key defined in the P2P protocol.

## ENR Structure with .onion Support

### Standard ENR Keys
```go
type ENR struct {
    id          string    // "v4" identity scheme
    secp256k1   pubkey    // Node's public key
    ip          ipv4      // Clearnet IPv4 (optional)
    tcp         uint16    // Clearnet TCP port (optional)
    udp         uint16    // Discovery UDP port (optional)
    onion3      string    // Tor v3 hidden service address (NEW)
}
```

### Onion3 ENR Entry

**Implementation**: `p2p/enr/entries.go:97-101`

```go
// Onion3 is the "onion3" key, which holds a Tor v3 hidden service address.
// A valid Tor v3 address consists of 56 base32 characters followed by ".onion".
type Onion3 string

func (v Onion3) ENRKey() string { return "onion3" }
```

**Format**: `"abc123...xyz.onion"` (56 base32 chars + `.onion`)

## Propagation Flow

### 1. Node Startup with Tor Hidden Service

```
┌─────────────────────────────────┐
│  Node starts with Tor enabled   │
│  --tor-enabled                  │
│  --tor-control=127.0.0.1:9051   │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ Tor creates hidden service      │
│ Returns: abc...xyz.onion        │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ Node adds onion3 to ENR         │
│ ENR.Set("onion3", "abc...onion")│
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ Node signs ENR with private key │
│ Creates enode URL with ENR data │
└─────────────────────────────────┘
```

### 2. Discovery via discv5 DHT

```
Node A                     DHT Network                   Node B
  │                              │                           │
  │── FINDNODE(target) ────────►│                           │
  │                              │                           │
  │◄────── NODES(list) ─────────│                           │
  │   [ENR1, ENR2, ENR3...]      │                           │
  │                              │                           │
  │── Parse ENR3 ───────────────►│                           │
  │   Contains: onion3="xyz.onion"                          │
  │                              │                           │
  │── TorDialer.Dial(ENR3) ─────────────────────────────────►│
  │   Extract: xyz.onion:30303   │                           │
  │   Via: SOCKS5 proxy          │                           │
  │                              │                           │
  │◄─────────────────── RLPx Handshake ─────────────────────│
  │                              │                           │
  │◄─────────────────── P2P Connection Established ─────────│
```

### 3. ENR Exchange During Connection

**When Node B connects to Node A:**

1. **PING/PONG Exchange**: Nodes exchange ENRs during handshake
2. **ENR Caching**: Each node caches peer ENRs locally
3. **DHT Updates**: Nodes announce their ENR to DHT neighbors
4. **Peer Gossip**: ENRs propagate through network via peer exchange

### 4. ENR Update Propagation

```
Node updates ENR (new .onion address)
        ↓
Signs new ENR with private key
        ↓
Announces to DHT neighbors (PING with ENR)
        ↓
Neighbors cache new ENR
        ↓
Neighbors gossip to their peers
        ↓
Network-wide propagation (eventual consistency)
```

## Current Implementation Status

### ✅ Implemented

1. **ENR Onion3 Support**: `p2p/enr/entries.go`
   - RLP encoding/decoding
   - Validation (56 base32 chars + .onion)
   - ENR key registration

2. **TorDialer Integration**: `p2p/tor_dialer.go:102`
   ```go
   // Extract .onion address from ENR or hostname
   var onionAddr string
   onion := enr.Onion3("")
   if err := node.Load(&onion); err == nil {
       onionAddr = string(onion)
   }
   ```

3. **Enode URL Parsing**: `p2p/enode/urlv4.go:220`
   - Extracts .onion from enode URLs
   - Parses ENR entries for onion3 key

4. **Integration Tests**: `p2p/tor_integration_test.go`
   - TestTorIntegration_TwoNodesDiscoverAndConnect
   - TestTorIntegration_ENRPropagation
   - Validates ENR round-trip encoding

### ⚠️ Manual Configuration Required

**Current limitation**: Nodes must manually configure static .onion addresses or use bootstrap nodes.

**Why**: Tor hidden service creation requires:
1. Tor control port access
2. Hidden service key generation
3. Coordination with Tor daemon

### 🚧 Future Enhancement

**Automatic ENR Updates** (Not yet implemented):

```go
// When Tor hidden service is created
func (srv *Server) updateENRWithOnion(onionAddr string) {
    srv.localnode.Set(enr.Onion3(onionAddr))
    srv.DiscV5.UpdateNode(srv.localnode) // Announce to DHT
}
```

## How Our Production Setup Works

### Current Approach: Static Configuration + Peer Manager

```yaml
# docker-compose.yml
gethrelay-1:
  environment:
    - ONION_ADDRESS=hugonyvxxn...onion  # Static from Tor

peer-manager-1:
  script: |
    # Poll admin API every 30s
    while true; do
      # Discover .onion peers via admin_peers
      curl admin_peers | jq '.result[] | select(.enode | contains(".onion"))'

      # Promote to trusted
      curl admin_addTrustedPeer $ONION_ENODE

      sleep 30
    done
```

### Discovery Flow in Production

1. **Bootstrap**: Nodes start with static .onion addresses in ENR
2. **DHT Discovery**: discv5 runs over Tor SOCKS5 proxy
3. **ENR Reception**: Nodes receive ENRs containing onion3 keys
4. **TorDialer**: Detects hasOnion=true, connects via SOCKS5
5. **Peer Manager**: Monitors connections, promotes .onion peers
6. **Persistent**: Trusted peers maintained across restarts

## Technical Details

### ENR Encoding

**Onion3 in ENR**:
```
ENR format (RLP-encoded):
[
  signature,
  seq,           // Sequence number (increments on update)
  "id", "v4",
  "secp256k1", <pubkey>,
  "onion3", "abc...xyz.onion",
  "tcp", 30303
]
```

**Enode URL**:
```
enode://pubkey@abc...xyz.onion:30303?onion3=abc...xyz.onion
```

### Discovery Protocol

**discv5 FINDNODE**:
- Request: Target node ID + distance
- Response: List of ENRs matching distance
- Contains: All ENR keys including onion3

**DHT Routing**:
- K-buckets organize by XOR distance
- ENRs propagate through k-bucket refresh
- Network-wide visibility in ~log(N) hops

### Security Considerations

1. **ENR Signatures**: ENRs are cryptographically signed
2. **Key Verification**: Node public key must match signature
3. **Tor Anonymity**: .onion addresses don't leak IP information
4. **ENR Seq Numbers**: Prevent replay of old ENRs

## Implementation References

**Core Files**:
- `p2p/enr/entries.go` - Onion3 type definition
- `p2p/tor_dialer.go` - ENR onion3 extraction
- `p2p/enode/urlv4.go` - Enode URL parsing
- `p2p/tor_integration_test.go` - ENR propagation tests

**Ethereum Spec**:
- [EIP-778: Ethereum Node Records (ENR)](https://eips.ethereum.org/EIPS/eip-778)
- [discv5 specification](https://github.com/ethereum/devp2p/blob/master/discv5/discv5.md)

---

**Summary**: .onion addresses propagate through standard ENR mechanisms using the `onion3` key. Nodes advertise their hidden service addresses in signed ENRs, which spread through the DHT network. TorDialer extracts these addresses and connects via SOCKS5 proxy.
