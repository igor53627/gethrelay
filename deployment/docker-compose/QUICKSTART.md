# Quick Start Guide

Get your gethrelay cluster with Tor running in 5 minutes.

## Prerequisites

- Docker 20.10+
- Docker Compose 1.29+
- 8GB RAM
- 20GB disk space

## 1. One-Line Start

```bash
cd deployment/docker-compose && chmod +x scripts/*.sh && docker-compose up -d
```

## 2. Verify Deployment (2-3 minutes)

```bash
# Check all containers are running
docker-compose ps

# Expected output:
# NAME                COMMAND                  SERVICE             STATUS              PORTS
# gethrelay-1         "gethrelay --tor-soc…"   gethrelay-1         running (healthy)
# gethrelay-2         "gethrelay --tor-soc…"   gethrelay-2         running (healthy)
# gethrelay-3         "gethrelay --tor-soc…"   gethrelay-3         running (healthy)
# peer-manager-1      "/scripts/peer-manag…"   peer-manager-1      running
# peer-manager-2      "/scripts/peer-manag…"   peer-manager-2      running
# peer-manager-3      "/scripts/peer-manag…"   peer-manager-3      running
# tor-proxy           "/usr/bin/tor --Sock…"   tor                 running (healthy)
```

## 3. Watch Discovery in Action

```bash
# Stream logs from all peer managers
docker-compose logs -f peer-manager-1 peer-manager-2 peer-manager-3

# You should see:
# [peer-manager] New .onion peer discovered via DHT: enode://...
# [peer-manager] Successfully promoted to trusted peer
```

## 4. Verify Peer Connections

```bash
# Check peer count on node 1
docker-compose exec gethrelay-1 sh -c '
  wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
  --header="Content-Type: application/json" \
  http://127.0.0.1:8545
'

# Expected: {"jsonrpc":"2.0","id":1,"result":"0x2"}  (2 peers)
```

## 5. Get Node .onion Addresses

```bash
# Get all node .onion addresses
for i in 1 2 3; do
  echo "Node $i:"
  docker-compose exec gethrelay-$i sh -c '
    wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
    --header="Content-Type: application/json" \
    http://127.0.0.1:8545 | grep -o "[a-z0-9]\{56\}\.onion"
  '
done
```

## Common Commands

```bash
# Stop all services
docker-compose down

# Restart
docker-compose restart

# View logs
docker-compose logs -f

# Check resource usage
docker stats

# Clean up (WARNING: deletes all data)
docker-compose down -v
```

## Troubleshooting

### No Peers After 5 Minutes?

```bash
# 1. Check Tor is working
docker-compose logs tor | grep -i "bootstrapped 100%"

# 2. Check DHT discovery is enabled
docker-compose logs gethrelay-1 | grep -i "discv5"

# 3. Manually add a peer
ENODE=$(docker-compose exec gethrelay-2 sh -c 'wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' --header="Content-Type: application/json" http://127.0.0.1:8545 | grep -o "enode://[^\"]*"')

docker-compose exec gethrelay-1 sh -c "
  wget -q -O - --post-data='{\"jsonrpc\":\"2.0\",\"method\":\"admin_addPeer\",\"params\":[\"$ENODE\"],\"id\":1}' \
  --header='Content-Type: application/json' \
  http://127.0.0.1:8545
"
```

### Still Having Issues?

See the [full README.md](README.md) for comprehensive troubleshooting guide.

## Next Steps

- Read [README.md](README.md) for detailed documentation
- Check [docker-compose.dev.yml](docker-compose.dev.yml) for development setup
- Review [.env.example](.env.example) for configuration options

## Success Criteria

Your deployment is successful when:

1. All 7 containers are running (healthy)
2. Each node reports 2+ peers
3. Peer managers are promoting .onion peers
4. Nodes have generated .onion addresses
5. No error messages in logs

**Deployment time**: 2-3 minutes for full cluster startup and peer discovery.
