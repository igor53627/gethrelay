# Gethrelay Docker Compose Deployment

Hybrid DHT + Admin API pattern for automatic .onion peer discovery.

## Quick Start

```bash
# Start all services
docker compose up -d

# Watch logs
docker compose logs -f

# Check peer counts
docker exec gethrelay-1 gethrelay attach --exec "admin.peers.length"
docker exec gethrelay-2 gethrelay attach --exec "admin.peers.length"
docker exec gethrelay-3 gethrelay attach --exec "admin.peers.length"

# Monitor peer manager
docker compose logs -f peer-manager-1

# Stop all services
docker compose down
```

## Architecture

- **Tor Proxy**: Shared SOCKS5 proxy (port 9050)
- **3 Gethrelay Nodes**: DHT discovery enabled, only-onion mode
- **3 Peer Managers**: Monitor DHT, promote .onion peers to trusted

## How It Works

1. Nodes start with DHT discovery enabled (--v5disc)
2. Discover each other via discv5 over Tor
3. Peer managers monitor connected peers via admin API
4. .onion peers are promoted to trusted status
5. Persistent connections maintained

## Monitoring

```bash
# Check node health
docker compose ps

# View specific node logs
docker compose logs gethrelay-1

# Check Tor status
docker compose logs tor

# Monitor peer discovery
docker compose logs -f peer-manager-1 peer-manager-2 peer-manager-3
```

## Troubleshooting

### No peers connecting
- Check Tor is running: `docker compose logs tor`
- Verify DHT is enabled: Look for "Started P2P networking" in logs
- Check peer manager logs: `docker compose logs peer-manager-1`

### Container won't start
- Check logs: `docker compose logs <service>`
- Verify image exists: `docker images | grep gethrelay`
- Restart: `docker compose restart <service>`

## Scaling

Add more nodes by copying the gethrelay-3 and peer-manager-3 sections in docker-compose.yml.
