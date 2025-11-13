# EIP Reference Clarification

## Important Correction

Earlier documentation and release materials incorrectly referenced "EIP-7691 (Ethereum Data Transmission)" as the foundation for gethrelay's relay architecture.

**This was incorrect.**

## What EIP-7691 Actually Is

**EIP-7691: Blob Throughput Increase**
- Increases blob count from 3 target/6 max → 6 target/9 max per block
- Focused on L2 scalability via increased data availability
- Implemented in the Pectra hardfork
- Unrelated to relay node architecture

Reference: https://eips.ethereum.org/EIPS/eip-7691

## What Gethrelay Actually Is

**Gethrelay is a lightweight Ethereum P2P relay node:**

1. **No specific EIP required** - Uses standard go-ethereum P2P infrastructure
2. **Network layer enhancement** - Adds Tor hidden service support to existing P2P protocols
3. **No consensus changes** - Works with current Ethereum mainnet without protocol modifications
4. **Relay architecture** - Forwards RPC calls to upstream while maintaining P2P connectivity

## Technical Foundation

The relay architecture is based on:
- **go-ethereum's P2P stack**: Standard discv5 DHT, devp2p protocols
- **Tor integration**: Custom TorDialer for .onion connections
- **Admin API**: For programmatic peer management
- **HTTP RPC proxy**: Lightweight forwarding to upstream nodes

## Corrected Messaging

**Instead of:** "EIP-7691 EDT enables relay layer innovation"

**Should be:** "Lightweight relay architecture enables network layer innovation without consensus changes"

---

**Date**: 2025-01-13
**Impact**: Documentation and release notes
**Action**: This clarification document supersedes any EIP-7691 references in historical commits or PR descriptions
