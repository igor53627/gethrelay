# Gethrelay Docker Compose Deployment

Production-ready Docker Compose deployment for gethrelay nodes with Tor integration and automatic .onion peer discovery.

## Architecture Overview

This deployment implements the **Hybrid DHT + Admin API Dynamic Addition** pattern for peer discovery over Tor:

### Components

1. **Shared Tor Daemon**: Single Tor instance providing SOCKS5 proxy for all nodes
2. **Gethrelay Nodes**: Three P2P nodes with DHT discovery enabled
3. **Peer Manager Sidecars**: Monitors DHT-discovered peers and promotes .onion peers to trusted status

### How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                       Tor Network                                │
│  (Shared SOCKS5 Proxy + Control Port)                           │
└─────────────────────────────────────────────────────────────────┘
              │                │                │
              ▼                ▼                ▼
    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
    │ Gethrelay-1  │  │ Gethrelay-2  │  │ Gethrelay-3  │
    │ (DHT enabled)│  │ (DHT enabled)│  │ (DHT enabled)│
    │  localhost   │  │  localhost   │  │  localhost   │
    │  RPC: 8545   │  │  RPC: 8545   │  │  RPC: 8545   │
    └──────────────┘  └──────────────┘  └──────────────┘
              │                │                │
              ▼                ▼                ▼
    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
    │Peer Manager-1│  │Peer Manager-2│  │Peer Manager-3│
    │  (Sidecar)   │  │  (Sidecar)   │  │  (Sidecar)   │
    │network_mode: │  │network_mode: │  │network_mode: │
    │service:geth-1│  │service:geth-2│  │service:geth-3│
    └──────────────┘  └──────────────┘  └──────────────┘
```

### Discovery Flow

1. **Phase 1 (Startup)**: All nodes start with DHT discovery enabled (`--v5disc`)
2. **Phase 2 (Discovery)**: Nodes discover each other via DHT over Tor
3. **Phase 3 (Monitoring)**: Peer manager sidecars continuously monitor connected peers
4. **Phase 4 (Promotion)**: .onion peers discovered via DHT are promoted to trusted status via `admin_addTrustedPeer`
5. **Result**: Dynamic discovery with static-like persistence

## Quick Start

### Prerequisites

- Docker 20.10+
- Docker Compose 1.29+
- 8GB RAM minimum
- 20GB disk space (for blockchain data)

### 1. Clone and Setup

```bash
# Navigate to the docker-compose directory
cd deployment/docker-compose

# Copy environment configuration (optional)
cp .env.example .env

# Make scripts executable
chmod +x scripts/*.sh
```

### 2. Start the Cluster

```bash
# Production deployment
docker-compose up -d

# Development deployment (with exposed ports and verbose logging)
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Watch logs
docker-compose logs -f

# Watch specific node
docker-compose logs -f gethrelay-1
```

### 3. Verify Deployment

```bash
# Check all containers are running
docker-compose ps

# Check Tor is operational
docker-compose exec tor sh -c 'nc -z localhost 9051 && echo "Tor OK"'

# Check node health
docker-compose exec gethrelay-1 sh -c 'nc -z localhost 30303 && echo "Node OK"'

# Get node .onion address
docker-compose exec gethrelay-1 sh -c '
  wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
  --header="Content-Type: application/json" \
  http://127.0.0.1:8545 | grep -o "enode://[^\"]*"
'
```

## Configuration

### Environment Variables

See `.env.example` for all available configuration options. Key variables:

```bash
# Tor Configuration
TOR_SOCKS_PORT=9050          # SOCKS5 proxy port
TOR_CONTROL_PORT=9051        # Control port

# Gethrelay Configuration
GETH_VERBOSITY=3             # Log verbosity (0-5)
GETH_MAX_PEERS=50            # Maximum network peers
GETH_NETWORK_ID=1            # Network ID (1=mainnet)

# Peer Manager Configuration
PEER_CHECK_INTERVAL=30       # Peer check interval (seconds)
```

### Customizing Node Count

To add a 4th node:

```yaml
# In docker-compose.yml
gethrelay-4:
  build:
    context: ../../
    dockerfile: Dockerfile
  container_name: gethrelay-4
  depends_on:
    tor:
      condition: service_healthy
  volumes:
    - geth-data-4:/data
    - ./scripts:/scripts:ro
  command:
    - --tor-socks-proxy=tor:9050
    - --only-onion
    - --datadir=/data/geth
    - --v5disc
    - --maxpeers=50
    - --http
    - --http.addr=127.0.0.1
    - --http.port=8545
    - --http.api=admin,eth,net
    - --verbosity=3
  networks:
    - tor-network
  environment:
    - NODE_NAME=gethrelay-4
  healthcheck:
    test: ["CMD", "nc", "-z", "localhost", "30303"]
    interval: 10s
    timeout: 5s
    retries: 10
    start_period: 30s
  restart: unless-stopped

peer-manager-4:
  image: alpine:latest
  container_name: peer-manager-4
  network_mode: service:gethrelay-4
  depends_on:
    gethrelay-4:
      condition: service_healthy
  volumes:
    - ./scripts:/scripts:ro
  command: ["/scripts/peer-manager.sh"]
  environment:
    - GETH_RPC=http://127.0.0.1:8545
    - NODE_NAME=gethrelay-4
    - PEER_CHECK_INTERVAL=30
  restart: unless-stopped

volumes:
  geth-data-4:
    driver: local
```

## Monitoring and Debugging

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f gethrelay-1
docker-compose logs -f peer-manager-1
docker-compose logs -f tor

# Last 100 lines
docker-compose logs --tail=100 gethrelay-1
```

### Check Peer Count

```bash
# For node 1
docker-compose exec gethrelay-1 sh -c '
  wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
  --header="Content-Type: application/json" \
  http://127.0.0.1:8545
'

# All nodes
for i in 1 2 3; do
  echo "Node $i peer count:"
  docker-compose exec gethrelay-$i sh -c '
    wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
    --header="Content-Type: application/json" \
    http://127.0.0.1:8545
  '
done
```

### List Connected Peers

```bash
# Detailed peer information
docker-compose exec gethrelay-1 sh -c '
  wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}'\'' \
  --header="Content-Type: application/json" \
  http://127.0.0.1:8545
'
```

### Check Trusted Peers

```bash
# Trusted peers have persistent connections
docker-compose exec gethrelay-1 sh -c '
  wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}'\'' \
  --header="Content-Type: application/json" \
  http://127.0.0.1:8545 | grep -o "\"trusted\":[^,]*"
'
```

### Verify Tor Connectivity

```bash
# Test SOCKS5 proxy
docker-compose exec gethrelay-1 sh -c '
  apk add --no-cache curl &&
  curl --socks5-hostname tor:9050 https://check.torproject.org/api/ip
'

# Check Tor circuit
docker-compose exec tor sh -c 'cat /var/lib/tor/state'
```

### Inspect .onion Addresses

```bash
# Get all node .onion addresses
for i in 1 2 3; do
  echo "Node $i .onion address:"
  docker-compose exec gethrelay-$i sh -c '
    wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
    --header="Content-Type: application/json" \
    http://127.0.0.1:8545 | grep -o "[a-z0-9]\{56\}\.onion"
  '
done
```

## Management Operations

### Start/Stop Services

```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Stop without removing volumes (preserves data)
docker-compose stop

# Restart specific service
docker-compose restart gethrelay-1
```

### Update and Rebuild

```bash
# Rebuild gethrelay image after code changes
docker-compose build gethrelay-1 gethrelay-2 gethrelay-3

# Rebuild and restart
docker-compose up -d --build

# Pull latest Tor image
docker-compose pull tor
```

### Volume Management

```bash
# List volumes
docker volume ls | grep docker-compose

# Inspect volume
docker volume inspect docker-compose_geth-data-1

# Backup node data
docker run --rm -v docker-compose_geth-data-1:/data -v $(pwd):/backup \
  alpine tar czf /backup/geth-1-backup.tar.gz /data

# Restore node data
docker run --rm -v docker-compose_geth-data-1:/data -v $(pwd):/backup \
  alpine tar xzf /backup/geth-1-backup.tar.gz -C /

# Clean up all volumes (WARNING: deletes all data)
docker-compose down -v
```

### Resource Usage

```bash
# Show resource usage
docker stats $(docker-compose ps -q)

# Show disk usage
docker system df

# Clean up unused resources
docker system prune -a
```

## Troubleshooting

### Problem: Nodes Not Discovering Each Other

**Symptoms**: Peer count remains 0 after several minutes

**Solutions**:

1. Check Tor connectivity:
   ```bash
   docker-compose logs tor
   docker-compose exec gethrelay-1 sh -c 'nc -z tor 9050 && echo "Tor reachable"'
   ```

2. Verify DHT is enabled:
   ```bash
   docker-compose logs gethrelay-1 | grep -i "discv5"
   ```

3. Check peer manager is running:
   ```bash
   docker-compose logs peer-manager-1
   ```

4. Manually add peers:
   ```bash
   # Get enode from node 2
   ENODE=$(docker-compose exec gethrelay-2 sh -c '
     wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
     --header="Content-Type: application/json" \
     http://127.0.0.1:8545 | grep -o "enode://[^\"]*"
   ')

   # Add to node 1
   docker-compose exec gethrelay-1 sh -c "
     wget -q -O - --post-data='{\"jsonrpc\":\"2.0\",\"method\":\"admin_addPeer\",\"params\":[\"$ENODE\"],\"id\":1}' \
     --header='Content-Type: application/json' \
     http://127.0.0.1:8545
   "
   ```

### Problem: Tor Connection Failures

**Symptoms**: Logs show "connection refused" or "SOCKS5 proxy error"

**Solutions**:

1. Restart Tor:
   ```bash
   docker-compose restart tor
   ```

2. Check Tor health:
   ```bash
   docker-compose exec tor sh -c 'nc -z localhost 9051 && echo "Control port OK"'
   docker-compose exec tor sh -c 'nc -z localhost 9050 && echo "SOCKS5 OK"'
   ```

3. Verify network connectivity:
   ```bash
   docker network inspect docker-compose_tor-network
   ```

### Problem: RPC Not Responding

**Symptoms**: `wget` or `curl` commands timeout

**Solutions**:

1. Check gethrelay is running:
   ```bash
   docker-compose ps gethrelay-1
   docker-compose logs gethrelay-1 --tail=50
   ```

2. Verify RPC port:
   ```bash
   docker-compose exec gethrelay-1 sh -c 'nc -z localhost 8545 && echo "RPC listening"'
   ```

3. Check for initialization errors:
   ```bash
   docker-compose logs gethrelay-1 | grep -i error
   ```

### Problem: High Memory Usage

**Symptoms**: Containers consuming too much RAM

**Solutions**:

1. Reduce max peers:
   ```yaml
   command:
     - --maxpeers=25  # Reduced from 50
   ```

2. Add memory limits:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 2G
   ```

3. Monitor resource usage:
   ```bash
   docker stats --no-stream
   ```

### Problem: Peer Manager Not Promoting Peers

**Symptoms**: Peers connect but aren't promoted to trusted

**Solutions**:

1. Check peer manager logs:
   ```bash
   docker-compose logs peer-manager-1 --tail=100
   ```

2. Verify RPC access from sidecar:
   ```bash
   docker-compose exec peer-manager-1 sh -c 'nc -z 127.0.0.1 8545 && echo "RPC accessible"'
   ```

3. Manually test addTrustedPeer:
   ```bash
   docker-compose exec gethrelay-1 sh -c '
     wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_addTrustedPeer","params":["enode://..."],"id":1}'\'' \
     --header="Content-Type: application/json" \
     http://127.0.0.1:8545
   '
   ```

## Security Considerations

### Production Security Checklist

- [ ] RPC bound to `127.0.0.1` only (localhost)
- [ ] Limited API exposure (`admin,eth,net` only, no `personal` or `accounts`)
- [ ] Tor cookie authentication enabled
- [ ] Volumes are not world-readable
- [ ] No sensitive data in environment variables
- [ ] Container images are from trusted sources
- [ ] Regular security updates applied
- [ ] Network isolation configured correctly

### Security Best Practices

1. **RPC Access**: Never expose RPC to `0.0.0.0` in production
2. **API Modules**: Only enable necessary API modules
3. **Tor Authentication**: Keep cookie authentication enabled
4. **Volume Permissions**: Set appropriate file permissions
5. **Network Isolation**: Use dedicated Docker network
6. **Log Monitoring**: Monitor logs for suspicious activity
7. **Regular Updates**: Keep Docker images updated

## Performance Tuning

### Resource Limits

```yaml
# In docker-compose.yml
services:
  gethrelay-1:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '1'
          memory: 2G
```

### Peer Configuration

```yaml
command:
  - --maxpeers=50          # Adjust based on bandwidth
  - --netrestrict=10.0.0.0/8  # Restrict to private network if needed
```

### Storage Optimization

```yaml
volumes:
  geth-data-1:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /mnt/fast-ssd/geth-data-1  # Use SSD for better performance
```

## Development Workflow

### Local Testing

```bash
# Use development override
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

# Access RPC from host
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}'

# Access pprof profiling
curl http://localhost:6060/debug/pprof/
```

### Testing Changes

```bash
# 1. Make code changes
# 2. Rebuild containers
docker-compose build

# 3. Restart with new image
docker-compose up -d

# 4. Verify changes
docker-compose logs -f gethrelay-1
```

### CI/CD Integration

```yaml
# .gitlab-ci.yml or .github/workflows/docker-compose.yml
test:
  script:
    - docker-compose up -d
    - docker-compose exec gethrelay-1 sh -c 'nc -z localhost 30303'
    - docker-compose down -v
```

## Scaling

### Adding More Nodes

1. Copy existing node configuration in `docker-compose.yml`
2. Update node name, container name, and volume name
3. Add corresponding peer-manager sidecar
4. Restart deployment

### Docker Swarm Deployment

For multi-host deployments, consider Docker Swarm:

```bash
# Initialize swarm
docker swarm init

# Deploy stack
docker stack deploy -c docker-compose.yml gethrelay

# Scale service
docker service scale gethrelay_gethrelay-1=5
```

## Backup and Recovery

### Backup Strategy

```bash
# Backup all node data
for i in 1 2 3; do
  docker run --rm \
    -v docker-compose_geth-data-$i:/data \
    -v $(pwd)/backups:/backup \
    alpine tar czf /backup/geth-$i-$(date +%Y%m%d).tar.gz /data
done

# Backup configuration
tar czf config-backup.tar.gz docker-compose.yml .env scripts/
```

### Recovery Procedure

```bash
# Stop services
docker-compose down

# Restore data
docker run --rm \
  -v docker-compose_geth-data-1:/data \
  -v $(pwd)/backups:/backup \
  alpine tar xzf /backup/geth-1-20231113.tar.gz -C /

# Restart services
docker-compose up -d
```

## Additional Resources

- [Gethrelay Documentation](../../README.md)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Tor Project](https://www.torproject.org/)
- [Ethereum P2P Networking](https://github.com/ethereum/devp2p)

## License

This deployment configuration is part of the go-ethereum project and is licensed under the same terms.
