# Tor Integration Troubleshooting Guide

## Overview

This guide helps diagnose and resolve common issues with Tor integration in gethrelay.

## Quick Diagnostics

Run these commands to quickly diagnose Tor connectivity:

```bash
# 1. Check Tor daemon is running
ps aux | grep tor

# 2. Check SOCKS5 proxy is listening
netstat -an | grep 9050

# 3. Test SOCKS5 connectivity
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# 4. Check gethrelay logs
tail -f /var/log/gethrelay.log | grep -i tor
```

## Common Issues

### 1. Failed to Connect to SOCKS5 Proxy

**Error message:**
```
Failed to create SOCKS5 dialer: dial tcp 127.0.0.1:9050: connect: connection refused
```

**Cause:** Tor daemon not running or SOCKS5 proxy not listening on expected port.

**Solution:**

**Step 1: Check Tor is running**
```bash
# Check Tor process
ps aux | grep tor

# Check Tor service status
sudo systemctl status tor  # Linux
brew services list | grep tor  # macOS
```

**Step 2: Start Tor if not running**
```bash
# Linux
sudo systemctl start tor
sudo systemctl enable tor

# macOS
brew services start tor

# Manual start (all platforms)
tor --SOCKSPort 9050
```

**Step 3: Verify SOCKS5 port**
```bash
# Check what port Tor is listening on
netstat -an | grep LISTEN | grep 9050

# Or check Tor config
grep SOCKSPort /etc/tor/torrc
```

**Step 4: Update gethrelay command if needed**
```bash
# If Tor uses different port (e.g., 9150)
gethrelay --tor-proxy=127.0.0.1:9150
```

**Step 5: Test SOCKS5 directly**
```bash
# Should return Tor IP address
curl --socks5 127.0.0.1:9050 https://api.ipify.org

# Should say "Congratulations. This browser is configured to use Tor."
curl --socks5 127.0.0.1:9050 https://check.torproject.org | grep -i congratulations
```

---

### 2. Only-Onion Mode: Peer Has No .onion Address

**Error message:**
```
only-onion mode: peer <node-id> has no .onion address
```

**Cause:** In `--only-onion` mode, gethrelay rejects peers without .onion addresses in their ENR.

**Solution:**

**Step 1: Verify peer ENRs**
```bash
# Check if bootnodes have .onion addresses
# Decode ENR and look for onion3 key
```

**Step 2: Use Tor-compatible bootnodes**

Ensure your bootnodes include .onion addresses:
```bash
gethrelay --tor-proxy=127.0.0.1:9050 \
  --only-onion \
  --bootnodes="<enr-with-onion3-key>"
```

**Step 3: Consider alternative modes**

If few peers have .onion addresses:
```bash
# Use prefer-tor instead (allows clearnet fallback)
gethrelay --tor-proxy=127.0.0.1:9050 --prefer-tor

# Or default mode (Tor when available)
gethrelay --tor-proxy=127.0.0.1:9050
```

**Step 4: Build Tor-only peer network**

Create a network of Tor-only nodes:
1. Deploy multiple gethrelay instances with Tor
2. Share .onion ENRs among nodes
3. Use as bootnodes for --only-onion mode

---

### 3. SOCKS5 Dial Timeout

**Error message:**
```
SOCKS5 dial to abc...xyz.onion:30303 failed: i/o timeout
```

**Cause:** Tor circuit establishment timeout or .onion service unreachable.

**Solution:**

**Step 1: Increase timeout**

Tor connections take 1-5 seconds. Ensure adequate timeout:
```bash
# Gethrelay uses context deadlines from P2P config
# Default timeout is usually 30 seconds, which should be sufficient
```

**Step 2: Check Tor circuit establishment**

```bash
# Watch Tor logs for circuit errors
tail -f /var/log/tor/log | grep -i circuit

# Common issues:
# - "Circuit build timeout" → Tor network congestion
# - "Failed to establish circuit" → Connectivity issues
```

**Step 3: Test .onion reachability**

```bash
# Try connecting to .onion address manually
curl --socks5 127.0.0.1:9050 http://abc...xyz.onion:30303

# If this fails, .onion service is unreachable
```

**Step 4: Verify peer's hidden service**

The peer may not have a working hidden service:
- Hidden service not started
- Tor daemon not running on peer
- Port mapping incorrect

**Step 5: Use clearnet fallback (if available)**

```bash
# Default mode will fallback to clearnet
gethrelay --tor-proxy=127.0.0.1:9050

# Prefer-tor mode also has fallback
gethrelay --tor-proxy=127.0.0.1:9050 --prefer-tor
```

---

### 4. .onion Address Not in ENR

**Issue:** Hidden service created but .onion address not appearing in ENR.

**Diagnosis:**
```bash
# Check node info
gethrelay admin.nodeInfo

# Look for onion3 field in ENR
# If missing, .onion address wasn't added
```

**Solution:**

**Step 1: Verify Tor control port access**

```bash
# Check Tor control port
netstat -an | grep 9051

# Test control port access
telnet 127.0.0.1 9051
# Type: PROTOCOLINFO 1
# Should get: 250-PROTOCOLINFO 1
```

**Step 2: Check Tor authentication**

Edit `/etc/tor/torrc`:
```
ControlPort 9051
CookieAuthentication 1
```

Restart Tor:
```bash
sudo systemctl restart tor
```

**Step 3: Verify cookie file permissions**

```bash
# Check cookie file exists
ls -la /var/run/tor/control.authcookie

# Ensure gethrelay can read it
sudo chmod 644 /var/run/tor/control.authcookie

# Or run gethrelay as tor user
sudo -u debian-tor gethrelay ...
```

**Step 4: Check gethrelay logs**

```bash
# Look for Tor-related errors
grep -i "tor" /var/log/gethrelay.log

# Should see: "P2P Tor hidden service ready"
```

**Step 5: Manual .onion address retrieval**

If automatic creation fails, check Tor's hidden service directory:
```bash
# Check hidden service hostname file
cat /var/lib/tor/gethrelay/hostname

# Should contain: abc...xyz.onion
```

---

### 5. Slow Connection Establishment

**Issue:** Connections take 3-5+ seconds to establish.

**Cause:** Normal Tor behavior. Circuit building takes time.

**Solution:**

**This is expected.** Tor connections are slower than clearnet:

| Phase | Clearnet | Tor |
|-------|----------|-----|
| DNS resolution | 10-50ms | N/A (.onion) |
| TCP handshake | 20-100ms | N/A |
| Circuit building | N/A | 500-3000ms |
| SOCKS5 handshake | N/A | 50-100ms |
| P2P handshake | 50-200ms | 100-500ms |
| **Total** | **80-350ms** | **650-3600ms** |

**Optimizations:**

1. **Accept the latency trade-off**
   - Tor prioritizes privacy over speed
   - 3-5 seconds is normal for first connection

2. **Use prefer-tor for hot paths**
   - Keep some clearnet peers for low-latency needs
   - Use Tor for privacy-sensitive connections

3. **Pre-build circuits (future work)**
   - Tor supports circuit pre-building
   - Not currently implemented

---

### 6. Fallback to Clearnet Not Working

**Issue:** In default mode, clearnet fallback not happening when Tor fails.

**Diagnosis:**
```bash
# Check gethrelay logs
grep -i "fallback" /var/log/gethrelay.log

# Should see fallback attempts
```

**Cause:** Peer may not have clearnet address in ENR.

**Solution:**

**Step 1: Verify peer has clearnet endpoint**

```bash
# Check peer ENR for TCP endpoint
# ENR should include: tcp=<port>, ip=<address>
```

**Step 2: Ensure not in only-onion mode**

```bash
# Only-onion mode disables fallback
# Remove --only-onion flag
gethrelay --tor-proxy=127.0.0.1:9050
```

**Step 3: Check clearnet connectivity**

```bash
# Test direct connection to peer
nc -zv <peer-ip> <peer-port>
```

---

### 7. Tor Daemon Crashes or Restarts

**Issue:** Tor daemon crashes, causing all Tor connections to fail.

**Diagnosis:**
```bash
# Check Tor logs
tail -f /var/log/tor/log

# Check system logs
journalctl -u tor -f
```

**Solution:**

**Step 1: Enable Tor service monitoring**

```bash
# Linux: use systemd to auto-restart
sudo systemctl enable tor
sudo systemctl restart tor

# Add restart policy to service
sudo systemctl edit tor

# Add:
[Service]
Restart=always
RestartSec=10
```

**Step 2: Configure gethrelay to retry**

Gethrelay will automatically fallback to clearnet in default mode.

**Step 3: Monitor Tor health**

```bash
# Use monitoring tools
watch -n 10 'curl --socks5 127.0.0.1:9050 https://check.torproject.org 2>&1 | grep -i congratulations'
```

**Step 4: Set up alerts**

```bash
# Alert if Tor goes down
*/5 * * * * curl --socks5 127.0.0.1:9050 https://check.torproject.org 2>&1 | grep -q congratulations || echo "Tor is down!" | mail -s "Tor Alert" admin@example.com
```

---

### 8. Invalid .onion Address Error

**Error message:**
```
invalid onion address: rlp: expected string or byte array
```

**Cause:** .onion address format is invalid (must be 56 base32 chars + ".onion").

**Solution:**

**Step 1: Validate .onion address format**

Valid Tor v3 address:
- Exactly 56 base32 characters (a-z, 2-7)
- Followed by ".onion"
- Total length: 62 characters

Example: `abc...xyz.onion` (56 chars + 6 chars)

**Step 2: Check hidden service creation**

```bash
# Read hostname file
cat /var/lib/tor/gethrelay/hostname

# Should be valid .onion address
```

**Step 3: Manually verify address**

```bash
# Length check
echo "abc...xyz.onion" | wc -c
# Should be 63 (62 + newline)

# Character check (only a-z, 2-7, and .onion)
echo "abc...xyz.onion" | grep -E '^[a-z2-7]{56}\.onion$'
```

---

### 9. Permission Denied Reading Tor Cookie

**Error message:**
```
failed to read Tor cookie: open /var/run/tor/control.authcookie: permission denied
```

**Solution:**

**Step 1: Check cookie file permissions**

```bash
ls -la /var/run/tor/control.authcookie
# Should be readable by gethrelay user
```

**Step 2: Add gethrelay user to tor group**

```bash
# Linux
sudo usermod -a -G debian-tor gethrelay-user
sudo systemctl restart gethrelay

# Or make cookie world-readable (less secure)
sudo chmod 644 /var/run/tor/control.authcookie
```

**Step 3: Run gethrelay as tor user**

```bash
sudo -u debian-tor gethrelay --tor-proxy=127.0.0.1:9050
```

---

### 10. Peers Not Connecting to My .onion Address

**Issue:** .onion address is advertised in ENR, but no peers connecting.

**Diagnosis:**

**Step 1: Test .onion service externally**

```bash
# From different machine with Tor
curl --socks5 127.0.0.1:9050 http://<your-onion>:30303

# Should connect (or timeout if firewall blocking)
```

**Step 2: Check hidden service is running**

```bash
# Check Tor logs
grep -i "hidden service" /var/log/tor/log

# Should see: "Opened Hidden Service descriptor file..."
```

**Step 3: Verify port mapping**

```bash
# Check hidden service config
cat /etc/tor/torrc | grep HiddenServicePort

# Should map to correct P2P port:
# HiddenServicePort 30303 127.0.0.1:30303
```

**Step 4: Check P2P server is listening**

```bash
netstat -an | grep LISTEN | grep 30303
# Should show: 127.0.0.1:30303 (or 0.0.0.0:30303)
```

**Step 5: Check firewall (local only)**

```bash
# Tor hidden services don't need inbound firewall rules
# But local firewall may block Tor daemon
sudo ufw status
```

---

## Debugging Techniques

### Enable Verbose Logging

**Gethrelay logging:**
```bash
gethrelay --tor-proxy=127.0.0.1:9050 --verbosity=5
```

**Tor logging:**

Edit `/etc/tor/torrc`:
```
Log notice file /var/log/tor/log
Log debug file /var/log/tor/debug.log
```

Restart Tor:
```bash
sudo systemctl restart tor
```

### Packet Capture

Capture SOCKS5 traffic to debug connection issues:

```bash
# Capture traffic to Tor SOCKS5 proxy
sudo tcpdump -i lo -A port 9050

# Should see SOCKS5 handshake and .onion address
```

### Test Components Independently

**1. Test Tor SOCKS5:**
```bash
curl --socks5 127.0.0.1:9050 https://check.torproject.org
```

**2. Test Tor control port:**
```bash
telnet 127.0.0.1 9051
PROTOCOLINFO 1
# Should get: 250-PROTOCOLINFO 1
```

**3. Test .onion reachability:**
```bash
curl --socks5 127.0.0.1:9050 http://<onion-address>:<port>
```

**4. Test P2P port locally:**
```bash
nc -zv 127.0.0.1 30303
```

### Check ENR Encoding

Decode ENR to verify .onion address:

```bash
# Get ENR from gethrelay
gethrelay admin.nodeInfo | jq -r '.protocols.eth.network.localAddress'

# Decode using devp2p tool (if available)
# Should show onion3 field
```

## Performance Issues

### High Latency

**Expected:** Tor adds 300-1000ms latency (3-10x slower than clearnet).

**Optimization:**
- Use `--prefer-tor` instead of `--only-onion` for critical paths
- Keep some clearnet peers for low-latency needs
- Accept trade-off: privacy vs. performance

### Low Throughput

**Expected:** Tor circuits limited to 1-10 MB/s (varies by circuit quality).

**Optimization:**
- Use clearnet for bulk data transfer
- Use Tor for metadata/discovery only
- Accept trade-off: privacy vs. bandwidth

### Connection Pool Exhaustion

**Issue:** Too many Tor connections causing resource exhaustion.

**Solution:**

```bash
# Limit max peers
gethrelay --tor-proxy=127.0.0.1:9050 --maxpeers=50

# Increase Tor circuit limits
# Edit /etc/tor/torrc:
MaxCircuitDirtiness 600
NumEntryGuards 8
```

## Getting Help

### Collect Diagnostic Information

Before asking for help, collect:

```bash
# 1. Gethrelay version
gethrelay version

# 2. Tor version
tor --version

# 3. Operating system
uname -a

# 4. Gethrelay logs (last 100 lines)
tail -100 /var/log/gethrelay.log > gethrelay-logs.txt

# 5. Tor logs (last 100 lines)
tail -100 /var/log/tor/log > tor-logs.txt

# 6. Configuration
echo "Gethrelay command:" > config.txt
ps aux | grep gethrelay >> config.txt
echo "\nTor config:" >> config.txt
cat /etc/tor/torrc >> config.txt

# 7. Network diagnostics
echo "SOCKS5 test:" > network-diag.txt
curl --socks5 127.0.0.1:9050 https://check.torproject.org >> network-diag.txt 2>&1
```

### Report Issues

**GitHub Issues:**
https://github.com/ethereum/go-ethereum/issues

**Include:**
- Diagnostic information (above)
- Steps to reproduce
- Expected vs. actual behavior
- Tor configuration

**Do NOT include:**
- Private keys
- .onion addresses (if privacy-sensitive)
- IP addresses (if privacy-sensitive)

## Further Reading

- [Tor Project Troubleshooting](https://support.torproject.org/)
- [SOCKS5 Protocol (RFC 1928)](https://www.rfc-editor.org/rfc/rfc1928)
- [Ethereum P2P Networking](https://github.com/ethereum/devp2p)

---

**Version:** 1.0
**Last Updated:** 2025-11-09
**Status:** Stable
