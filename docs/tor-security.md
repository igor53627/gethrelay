# Tor Integration Security Considerations

## Overview

This document analyzes the security and privacy implications of Tor integration in gethrelay. While Tor provides strong anonymity properties, integrating it with P2P systems introduces unique trade-offs and risks.

## Privacy Model

### Threat Model

**Attacker capabilities:**
- Network-level observer (ISP, state actor)
- Tor exit relay operator
- Malicious P2P peers
- Tor directory authority

**Attacker goals:**
- De-anonymize node operator (link .onion to IP address)
- Correlate P2P activity with real-world identity
- Disrupt or censor P2P connectivity
- Perform traffic analysis on P2P protocol

**Out of scope:**
- Compromise of Tor network itself
- Cryptographic breaks of Tor or P2P protocols
- Physical device compromise

### Privacy Guarantees

**What Tor provides:**
- **IP address hiding:** .onion addresses don't reveal IP address
- **Location privacy:** Tor circuits hide geographic location
- **Censorship resistance:** Tor bypasses network-level blocking
- **Traffic encryption:** End-to-end encryption through Tor circuit

**What Tor does NOT provide:**
- **Protocol-level anonymity:** P2P protocol may leak identity (node ID, etc.)
- **Timing attack resistance:** Long-lived connections vulnerable to traffic analysis
- **Dual-stack unlinkability:** Running both .onion and clearnet links identities
- **Exit relay privacy:** Clearnet connections from exit relays expose traffic

## Security Considerations by Mode

### Default Mode (Tor with Clearnet Fallback)

**Privacy level:** LOW to MEDIUM

**Risks:**
1. **Dual-stack linkability:** Running both .onion and clearnet addresses allows correlation
   - Network observer sees clearnet IP
   - P2P peers see both .onion and clearnet endpoints
   - Same node ID used for both transports
   - **Mitigation:** Use separate node identities for Tor and clearnet

2. **Fallback exposure:** Tor failures expose clearnet IP
   - SOCKS5 proxy unreachable → immediate clearnet connection
   - Tor circuit failure → fallback to clearnet
   - **Mitigation:** Monitor Tor proxy availability, alert on fallbacks

3. **Selective Tor usage:** Only some peers use Tor
   - Clearnet-only peers see your IP
   - Tor-only peers see your .onion
   - Traffic analysis can correlate the two
   - **Mitigation:** Use --prefer-tor or --only-onion for consistency

**Best practices:**
- Use for testing or low-risk scenarios
- Monitor fallback events
- Consider separate node identities
- Understand dual-stack linkability risk

### Prefer Tor Mode

**Privacy level:** MEDIUM

**Risks:**
1. **Clearnet fallback still possible**
   - Tor failures still expose clearnet IP
   - Reduces but doesn't eliminate linkability

2. **Inconsistent transport selection**
   - Different peers may see different transports
   - Traffic patterns may reveal dual-stack operation

**Best practices:**
- Better privacy than default mode
- Still suitable for low-risk scenarios
- Monitor Tor circuit health
- Consider --only-onion for higher privacy

### Tor-Only Mode (--only-onion)

**Privacy level:** HIGH

**Risks:**
1. **Reduced peer pool:** Smaller network of Tor-only peers
   - Fewer peers available for connection
   - May impact sync performance
   - Network partition risk if Tor fails

2. **Tor network dependency:** Complete reliance on Tor
   - Tor outages break all connectivity
   - Tor censorship breaks connectivity
   - Need reliable Tor proxy

3. **Long-lived connections:** P2P connections may last hours/days
   - Vulnerable to circuit correlation attacks
   - Traffic analysis can link sessions
   - **Mitigation:** Rotate circuits periodically (future work)

4. **Protocol fingerprinting:** P2P protocol may be identifiable
   - Deep packet inspection can identify Ethereum P2P
   - Even through Tor encryption layers
   - **Mitigation:** Use pluggable transports (future work)

**Best practices:**
- Maximum privacy for P2P networking
- Ensure reliable Tor proxy
- Monitor Tor circuit health
- Accept smaller peer pool trade-off
- Use for high-risk scenarios (censorship, surveillance)

## Attack Vectors

### 1. Dual-Stack Linkability Attack

**Description:** Attacker correlates .onion and clearnet identities using node ID or ENR.

**Attack steps:**
1. Discover node's ENR via clearnet connection
2. Extract node ID and .onion address from ENR
3. Connect to .onion address via Tor
4. Verify same node ID
5. Conclude: .onion address belongs to clearnet IP

**Impact:** De-anonymizes node operator

**Mitigation:**
- Use --only-onion mode (no clearnet exposure)
- Use separate node identities for Tor and clearnet
- Don't advertise .onion in clearnet ENR (requires code changes)

**Residual risk:** Node ID still linkable if same identity used

### 2. Traffic Correlation Attack

**Description:** Attacker correlates Tor circuit traffic with P2P protocol patterns.

**Attack steps:**
1. Operate Tor entry or exit relay
2. Observe traffic patterns (timing, volume)
3. Correlate with known P2P protocol patterns
4. Identify P2P sessions despite encryption

**Impact:** Reveals P2P activity, potential identity linkage

**Mitigation:**
- Use Tor bridges (hides Tor usage from ISP)
- Randomize traffic patterns (future work)
- Use pluggable transports to obfuscate protocol

**Residual risk:** Sophisticated traffic analysis still possible

### 3. Sybil Attack on Tor-Only Network

**Description:** Attacker creates many Tor-only nodes to dominate peer pool.

**Attack steps:**
1. Create hundreds of .onion addresses
2. Announce all in ENR via bootnodes
3. Tor-only nodes preferentially connect to attacker
4. Attacker gains network visibility

**Impact:** Network surveillance, eclipse attacks

**Mitigation:**
- Peer diversity requirements (mix Tor and clearnet)
- Reputation-based peer selection
- Limit peers from same /16 (doesn't work with Tor)

**Residual risk:** Tor makes Sybil attacks cheaper

### 4. Tor Exit Relay Attack

**Description:** Attacker operates Tor exit relay to spy on clearnet fallback traffic.

**Attack steps:**
1. Operate Tor exit relay
2. Wait for P2P clearnet fallback traffic
3. Inspect unencrypted traffic (P2P handshake is encrypted, but metadata visible)
4. Correlate with Tor circuit

**Impact:** Partial traffic visibility, metadata leakage

**Mitigation:**
- Use --only-onion mode (no clearnet fallback)
- Monitor fallback events
- Encrypt all P2P traffic (already done)

**Residual risk:** Metadata still visible to exit relay

### 5. SOCKS5 Proxy Compromise

**Description:** Attacker compromises local SOCKS5 proxy (Tor daemon).

**Attack steps:**
1. Compromise localhost or Tor daemon
2. Log all SOCKS5 requests (destinations)
3. Correlate with P2P activity

**Impact:** Complete de-anonymization

**Mitigation:**
- Run Tor daemon on trusted system
- Use authentication for SOCKS5 proxy
- Monitor Tor daemon integrity

**Residual risk:** Local compromise defeats all Tor protections

## Privacy Best Practices

### 1. Use Tor-Only Mode for High Privacy

```bash
gethrelay --tor-proxy=127.0.0.1:9050 --only-onion
```

This eliminates dual-stack linkability and clearnet exposure.

### 2. Run Tor Daemon on Localhost

Don't use remote SOCKS5 proxies. Run Tor daemon locally to avoid proxy trust issues.

```bash
# Good: localhost Tor
--tor-proxy=127.0.0.1:9050

# Bad: remote SOCKS5 proxy
--tor-proxy=remote.proxy.com:1080
```

### 3. Use Tor Bridges

If Tor usage itself is sensitive, use Tor bridges to hide Tor from ISP:

Edit `/etc/tor/torrc`:
```
UseBridges 1
Bridge obfs4 <bridge-address>
```

### 4. Separate Node Identities

Use different node keys for Tor and clearnet operations:

```bash
# Tor node (separate data directory)
gethrelay --datadir=/path/to/tor-node --tor-proxy=127.0.0.1:9050 --only-onion

# Clearnet node (different identity)
gethrelay --datadir=/path/to/clearnet-node
```

### 5. Monitor Fallback Events

In default/prefer-tor mode, log and alert on clearnet fallbacks:

```bash
# Check logs for fallback events
grep "fallback to clearnet" /var/log/gethrelay.log
```

Frequent fallbacks indicate Tor proxy issues or attacks.

### 6. Rotate Circuits (Future Work)

Long-lived connections are vulnerable to traffic analysis. Periodically rotate Tor circuits (not currently implemented):

```bash
# Send NEWNYM signal to Tor (requires implementation)
killall -HUP tor
```

### 7. Don't Mix Sensitive and Non-Sensitive Activity

If running Tor-only node for sensitive activity, don't run clearnet node from same IP:

```
Bad:  Tor-only gethrelay + clearnet geth → linkable
Good: Tor-only gethrelay only (separate machine/VPN for clearnet)
```

## Deployment Scenarios

### Scenario 1: Casual User (Low Privacy Needs)

**Configuration:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050
```

**Privacy:**
- Some IP address exposure acceptable
- Clearnet fallback is fine
- Dual-stack linkability not a concern

**Risk:** LOW

### Scenario 2: Privacy-Conscious User

**Configuration:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050 --prefer-tor
```

**Privacy:**
- Minimize IP address exposure
- Accept some clearnet fallback
- Understand dual-stack linkability

**Risk:** MEDIUM

### Scenario 3: High-Risk User (Censored Network)

**Configuration:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050 --only-onion
```

**Additional hardening:**
- Use Tor bridges
- Separate node identity
- Monitor Tor circuit health
- Run on trusted system

**Privacy:**
- Maximum IP address protection
- No clearnet fallback
- Smaller peer pool

**Risk:** MEDIUM (if best practices followed)

### Scenario 4: Whistleblower / Journalist

**Configuration:**
```bash
# Dedicated Tor-only node with hardening
gethrelay --datadir=/secure/tor-only \
  --tor-proxy=127.0.0.1:9050 \
  --only-onion \
  --bootnodes="<trusted-tor-only-peers>"
```

**Additional hardening:**
- Run in VM or Whonix
- Use Tor bridges with obfuscation
- Separate physical machine
- No clearnet activity from same IP
- Monitor all network traffic

**Privacy:**
- Maximum anonymity
- Complete Tor dependency
- Trusted peer bootstrap

**Risk:** LOW (if operational security maintained)

## Limitations

### 1. Protocol-Level Linkability

Even with Tor-only mode, the P2P protocol may leak identity:

- **Node ID:** Persistent across sessions (unless rotated)
- **ENR signature:** Proves control of private key
- **Protocol fingerprints:** Ethereum P2P is identifiable

**Mitigation:** Use separate node identities for different contexts

### 2. Timing Attacks

Long-lived P2P connections are vulnerable to timing correlation:

- Traffic patterns may be distinctive
- Connection establishment timing
- Message frequency and timing

**Mitigation:** Rotate circuits (future work), add traffic padding (future work)

### 3. Tor Network Risks

Tor itself has known vulnerabilities:

- **Directory authority compromise:** Can manipulate network
- **Exit relay attacks:** Can spy on clearnet traffic
- **Timing correlation:** Sophisticated attackers can correlate circuits

**Mitigation:** Accept residual Tor risks, use Tor best practices

### 4. No Perfect Unlinkability

Even with all best practices, perfect unlinkability is impossible:

- Node behavior may be distinctive
- P2P protocol metadata leaks info
- Operational security errors

**Mitigation:** Accept residual risks, follow best practices

## Security Checklist

**Before deploying Tor-only mode:**

- [ ] Understand threat model and privacy requirements
- [ ] Tor daemon running and accessible
- [ ] SOCKS5 proxy tested and working
- [ ] Bootnodes include Tor-only peers
- [ ] Monitoring for Tor proxy failures
- [ ] Separate node identity if needed
- [ ] Tor bridges configured if needed
- [ ] Operational security procedures in place
- [ ] Accept smaller peer pool trade-off
- [ ] Accept 3-10x latency trade-off

**Ongoing monitoring:**

- [ ] Tor circuit health
- [ ] Peer diversity (not all from same source)
- [ ] Fallback events (if not only-onion)
- [ ] Network connectivity
- [ ] Tor daemon logs for anomalies

## Responsible Disclosure

If you discover security vulnerabilities in the Tor integration:

1. **Do NOT** disclose publicly
2. Email security@ethereum.org with details
3. Include proof-of-concept if available
4. Allow 90 days for patch before disclosure

## Further Reading

- [Tor Project Security](https://www.torproject.org/docs/documentation.html)
- [Tor Threats and Attacks](https://svn.torproject.org/svn/projects/design-paper/tor-design.html#sec:attacks)
- [Ethereum P2P Security](https://github.com/ethereum/devp2p/blob/master/discv5/discv5-wire.md#security-considerations)
- [Privacy-Preserving P2P](https://eprint.iacr.org/2019/218.pdf)

## Conclusion

Tor integration provides strong privacy properties when used correctly, but introduces trade-offs:

**Advantages:**
- IP address hiding in Tor-only mode
- Censorship resistance
- NAT traversal without port forwarding

**Trade-offs:**
- 3-10x latency increase
- Smaller peer pool (Tor-only mode)
- Dual-stack linkability (default mode)
- Tor network dependency

**Recommendation:**
- **Low privacy needs:** Default mode is fine
- **Medium privacy needs:** Use --prefer-tor
- **High privacy needs:** Use --only-onion with best practices
- **Critical scenarios:** Use dedicated Tor-only deployment with operational security

---

**Version:** 1.0
**Last Updated:** 2025-11-09
**Status:** Stable
