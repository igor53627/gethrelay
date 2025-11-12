# Fix: Static Node .onion Address Resolution via Tor SOCKS5

## Problem

The `--staticnodes` flag was failing to connect to .onion addresses because gethrelay was attempting DNS resolution for .onion addresses, which always fails:

```
WARN DNS lookup of static node failed - address 55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion: no suitable address found
```

.Onion addresses are special-use top-level domains that must be resolved through Tor's SOCKS5 proxy, not through standard DNS.

## Root Cause

In `p2p/dial.go`, the `dnsResolveHostname()` function was attempting DNS resolution for all hostnames, including .onion addresses. This prevented static nodes with .onion addresses from connecting via Tor.

## Solution

Implemented a three-part fix following TDD methodology:

### 1. Onion Address Detection (`dial.go`)

Added `isOnionAddress()` helper function with case-insensitive detection:

```go
func isOnionAddress(hostname string) bool {
    return strings.HasSuffix(strings.ToLower(hostname), ".onion")
}
```

### 2. Skip DNS Resolution for .onion (`dial.go`)

Modified `dnsResolveHostname()` to skip DNS lookup for .onion addresses:

```go
func (d *dialScheduler) dnsResolveHostname(n *enode.Node) (*enode.Node, error) {
    hostname := n.Hostname()
    if hostname == "" {
        return n, nil
    }

    // Skip DNS resolution for .onion addresses - they must be resolved via Tor SOCKS5
    if isOnionAddress(hostname) {
        d.log.Trace("Skipping DNS resolution for .onion address", "hostname", hostname)
        return n, nil
    }

    // ... normal DNS resolution for regular hostnames
}
```

### 3. Skip IP Validation for .onion (`dial.go`)

Modified `dialTask.run()` to skip IP address validation for .onion addresses since they will be resolved by TorDialer through SOCKS5:

```go
func (t *dialTask) run(d *dialScheduler) {
    if t.isStatic() {
        // Resolve DNS for regular hostnames
        if n := t.dest(); n.Hostname() != "" {
            resolved, err := d.dnsResolveHostname(n)
            // ...
        }

        // Skip DHT resolution for .onion addresses
        dest := t.dest()
        isOnion := dest.Hostname() != "" && isOnionAddress(dest.Hostname())
        if !dest.IPAddr().IsValid() && !isOnion {
            if !t.resolve(d) {
                return // DHT resolve failed, skip dial.
            }
        }
    }
    // ... continue to dial
}
```

### 4. TorDialer Hostname Support (`tor_dialer.go`)

Enhanced TorDialer to extract .onion addresses from hostnames (for static nodes) in addition to ENR Onion3 entries:

```go
func (t *TorDialer) Dial(ctx context.Context, dest *enode.Node) (net.Conn, error) {
    // Extract .onion address from ENR or hostname
    var onion enr.Onion3
    var onionAddr string
    hasOnion := false

    // Check ENR first
    if dest.Load(&onion) == nil && onion != "" {
        onionAddr = string(onion)
        hasOnion = true
    } else if hostname := dest.Hostname(); hostname != "" && strings.HasSuffix(strings.ToLower(hostname), ".onion") {
        // Check hostname for .onion address (common for static nodes)
        onionAddr = hostname
        hasOnion = true
    }

    // ... proceed with Tor dial if hasOnion
}
```

## Test Coverage

Created comprehensive TDD test suite in `dial_onion_test.go`:

1. **TestIsOnionAddress** - Validates .onion detection (case-insensitive)
2. **TestDNSResolveSkipsOnionAddresses** - Ensures DNS is NOT called for .onion
3. **TestDNSResolveWorksForRegularHostnames** - Ensures DNS still works for regular domains
4. **TestStaticDialTaskOnionAddress** - Validates static node dial flow for .onion
5. **TestOnionAddressHandlingInTorDialer** - Confirms TorDialer handles .onion

Added TorDialer tests in `tor_dialer_test.go`:

6. **TestTorDialer_OnionHostname** - Validates hostname-based .onion resolution
7. **TestTorDialer_OnionHostnameCaseInsensitive** - Tests case variations

## Behavior Changes

### Before Fix

```
--staticnodes=enode://...@abc123.onion:30303
↓
DNS lookup: abc123.onion → FAIL ❌
↓
Connection abandoned
```

### After Fix

```
--staticnodes=enode://...@abc123.onion:30303
↓
Detected .onion → Skip DNS ✓
↓
TorDialer extracts hostname ✓
↓
SOCKS5 proxy to abc123.onion:30303 ✓
↓
Connection established via Tor ✓
```

## Verification

All existing tests pass:
```bash
cd /Users/user/pse/ethereum/go-ethereum
go test ./p2p
# PASS - 4.990s
```

## Usage

Static nodes with .onion addresses now work correctly:

```bash
gethrelay \
  --tor-socks-proxy=127.0.0.1:9050 \
  --only-onion \
  --staticnodes=enode://abc...@55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion:30303
```

Expected log output:
```
TRACE Skipping DNS resolution for .onion address hostname=55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion
DEBUG Starting p2p dial id=... endpoint=55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion flag=staticdial
DEBUG Adding p2p peer peercount=1 id=... conn=staticdial addr=...
```

## Files Modified

1. `p2p/dial.go` - Added .onion detection and DNS skip logic
2. `p2p/tor_dialer.go` - Enhanced to extract .onion from hostnames
3. `p2p/dial_onion_test.go` - NEW: TDD test suite for .onion handling
4. `p2p/tor_dialer_test.go` - Added hostname-based .onion tests

## Technical Notes

- .onion addresses are special-use top-level domains (RFC 7686)
- They cannot be resolved through standard DNS
- Must be resolved through Tor's SOCKS5 proxy
- Both v2 (16 chars) and v3 (56 chars) .onion formats are supported
- Case-insensitive detection (per DNS standards)
- Maintains backward compatibility with ENR Onion3 entries
