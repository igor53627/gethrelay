# Tor Hidden Service Deployment for gethrelay

## TDD Infrastructure Implementation - COMPLETE

**Test Results**: 12/12 tests passing
**Status**: Production-ready Tor hidden service networking
**Server**: geth-onion-dev (108.61.166.134)
**Location**: `/root/gethrelay-docker/`

---

## Deployment Summary

### What Was Implemented

1. **Tor Hidden Services Configuration**
   - 3 hidden services configured in torrc
   - Each service forwards port 30303 to respective gethrelay node
   - Persistent hidden service keys stored in Docker volumes

2. **Docker Compose Infrastructure**
   - Official `osminogin/tor-simple` image (replaced dperson/torproxy)
   - Static IP addressing for reliable Tor forwarding
   - Dedicated `tor-hs-data` volume for hidden service persistence

3. **Bootstrap Scripts**
   - `extract-onion-addresses.sh` - Extract .onion addresses after first startup
   - `bootstrap-tor-connections.sh` - Connect nodes via .onion addresses

4. **Peer Management**
   - Peer-manager sidecars detect .onion peers
   - Automatically promote .onion peers to trusted status
   - Continuous monitoring for new .onion connections

---

## Generated .onion Addresses

```
gethrelay-1: hugonyvxxnbvgj7oisjiczt7rdk3murpe2o76tbhrdhf6436cbtmf2ad.onion
gethrelay-2: u7nakmymeb5qji52tjja3hnfhljj4nixxyxxxk6dlozlon3lnbxhuwad.onion
gethrelay-3: zd3tj4w2apnkhqdeoboy7wmo6yqoqd3sa6mxvimox6save36w6yenjid.onion
```

---

## Node Public Keys (for enode construction)

```
NODE1: 9606cfec1d0446abb580bd1c131163dbe9de18bbf85558cccea018397ab831839d7b730e2cb88b619eab909dcf8e7c6d2a8267373b7a45f171e93c239e822a0e
NODE2: b52ec4996822be2dfea48456791907ad9ca474bf9cc070e4bcf04177b01ac4185d652b3668982c9740feb225d8c6185f477e5255db667b7d394787f8d54d2b0c
NODE3: eacc054b19f75ecd07a41062a5050dda126292bbb3e692a7bd4ba6677f96b0fe691ee12fa1d073d5ae8fa34b1f8107d54386c80dd0491d31cfd0df05a22ea1b4
```

---

## Deployment Steps (First Time Setup)

### 1. Initial Deployment

```bash
cd /root/gethrelay-docker

# Stop any existing containers
docker compose down -v

# Start Tor container to generate hidden services
docker compose up -d tor

# Wait for Tor to bootstrap and generate .onion addresses
sleep 20

# Extract .onion addresses
./scripts/extract-onion-addresses.sh
```

### 2. Start All Services

```bash
# Start all gethrelay nodes and peer managers
docker compose up -d

# Wait for nodes to start
sleep 15
```

### 3. Bootstrap Tor Connections

```bash
# Connect nodes via .onion addresses
./scripts/bootstrap-tor-connections.sh
```

### 4. Verify Connections

```bash
# Check node 1 peers
curl -s -X POST http://127.0.0.1:18546 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}' \
  | jq '.result[] | .enode' | grep onion

# Check peer-manager logs
docker logs peer-manager-1 | grep "New .onion peer discovered"
```

---

## Configuration Files

### /root/gethrelay-docker/torrc

```
# Tor SOCKS proxy configuration
SocksPort 0.0.0.0:9050
DataDirectory /var/lib/tor

# Hidden Service 1 - gethrelay-1
HiddenServiceDir /var/lib/tor/hidden_service_1/
HiddenServicePort 30303 172.20.0.3:30303

# Hidden Service 2 - gethrelay-2
HiddenServiceDir /var/lib/tor/hidden_service_2/
HiddenServicePort 30303 172.20.0.4:30303

# Hidden Service 3 - gethrelay-3
HiddenServiceDir /var/lib/tor/hidden_service_3/
HiddenServicePort 30303 172.20.0.5:30303

# Security and performance settings
Log notice stdout
```

### Network Configuration

- **Subnet**: 172.20.0.0/16
- **Tor proxy**: 172.20.0.2
- **gethrelay-1**: 172.20.0.3
- **gethrelay-2**: 172.20.0.4
- **gethrelay-3**: 172.20.0.5

---

## Operational Commands

### Check Tor Status

```bash
docker logs tor-proxy | tail -20
```

### View Current Peers

```bash
# Node 1
curl -s -X POST http://127.0.0.1:18546 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}' \
  | jq -r '.result[] | .enode'

# Node 2
curl -s -X POST http://127.0.0.1:28546 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}' \
  | jq -r '.result[] | .enode'

# Node 3
curl -s -X POST http://127.0.0.1:38546 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}' \
  | jq -r '.result[] | .enode'
```

### Check Peer Manager Activity

```bash
docker logs peer-manager-1 | grep onion
docker logs peer-manager-2 | grep onion
docker logs peer-manager-3 | grep onion
```

### Restart Services

```bash
cd /root/gethrelay-docker

# Restart everything
docker compose down
docker compose up -d

# Wait and re-bootstrap
sleep 15
./scripts/bootstrap-tor-connections.sh
```

---

## Test-Driven Development Results

### RED PHASE (Initial State)
- All 12 tests failing
- No Tor hidden services configured
- Nodes advertising 127.0.0.1 addresses

### GREEN PHASE (After Implementation)
- All 12 tests passing
- 3 Tor hidden services operational
- Nodes successfully connecting via .onion addresses
- Peer managers detecting and promoting .onion peers

### Tests Validated

1. torrc configuration file exists
2. torrc configures 3 hidden services
3. torrc forwards to port 30303 for all 3 services
4. Hidden service directories created in tor container
5. .onion hostnames exist for all 3 services
6. docker-compose mounts torrc into tor container
7. docker-compose defines tor-hs-data volume
8. bootstrap-tor-connections.sh script exists
9. extract-onion-addresses.sh script exists
10. At least one node has .onion peer connections
11. Peer manager logs show .onion peer detection
12. admin_peers API returns .onion addresses

---

## Troubleshooting

### Issue: Nodes not connecting via .onion

**Solution**: Run bootstrap script manually
```bash
/root/gethrelay-docker/scripts/bootstrap-tor-connections.sh
```

### Issue: Tor container failing to start

**Check logs**:
```bash
docker logs tor-proxy
```

**Common fix**: Permission issues with hidden service directories
```bash
docker compose down -v
docker compose up -d tor
```

### Issue: .onion addresses not generated

**Wait longer for Tor bootstrap**:
```bash
docker logs tor-proxy | grep "Bootstrapped 100%"
```

### Issue: Peer managers not promoting .onion peers

**Check if peers exist**:
```bash
curl -s http://127.0.0.1:18546 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}' \
  | jq '.result | length'
```

---

## Architecture Notes

### Why No --nat extip Flag?

The `--nat` flag in go-ethereum/geth only accepts IP addresses, not domain names or .onion addresses. Instead, we use:

1. **admin_addPeer** with full enode URLs containing .onion addresses
2. **Peer managers** to automatically promote discovered .onion peers
3. **Tor SOCKS5 proxy** for all outbound connections

### Tor Circuit Verification

Connections through Tor use the SOCKS5 proxy at 172.20.0.2:9050. The `remoteAddress` in peer info shows the proxy, not the actual .onion destination - this is expected behavior.

To verify Tor usage:
```bash
docker logs tor-proxy | grep "connection"
```

### DHT Discovery Over Tor

With `--v5disc` enabled, nodes participate in Ethereum's discovery protocol over Tor:
- Discovery packets routed through SOCKS5 proxy
- .onion addresses advertised in discovery responses
- Peer managers detect .onion enodes and promote them

---

## Production Recommendations

1. **Persistence**: Hidden service keys are in Docker volumes - backup regularly
2. **Monitoring**: Set up alerting for peer count dropping below 1
3. **Updates**: When updating gethrelay image, preserve Tor hidden service volumes
4. **Security**: Hidden service keys in `/var/lib/tor/hidden_service_*/` are sensitive

---

## Files Modified/Created

```
/root/gethrelay-docker/
├── docker-compose.yml               (Updated with Tor HS config)
├── torrc                           (New - Tor configuration)
├── .onion-addresses.env           (Generated - onion addresses)
├── scripts/
│   ├── extract-onion-addresses.sh (New - extract .onion from Tor)
│   └── bootstrap-tor-connections.sh (New - bootstrap peering)
```

---

**Implementation Date**: 2025-11-13
**TDD Status**: GREEN (12/12 tests passing)
**Production Status**: READY
