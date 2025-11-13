# Docker Compose Deployment Guide

Complete production deployment guide for gethrelay with Tor integration using the Hybrid DHT + Admin API pattern.

## Table of Contents

1. [Architecture](#architecture)
2. [Prerequisites](#prerequisites)
3. [Installation](#installation)
4. [Configuration](#configuration)
5. [Deployment](#deployment)
6. [Validation](#validation)
7. [Monitoring](#monitoring)
8. [Maintenance](#maintenance)
9. [Troubleshooting](#troubleshooting)
10. [Security](#security)

## Architecture

### Overview

This deployment implements **Approach 7: Hybrid DHT + Admin API Dynamic Addition** from the research document.

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Compose Stack                      │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Tor Daemon (Shared)                      │  │
│  │  - SOCKS5 Proxy: 9050                                │  │
│  │  - Control Port: 9051                                │  │
│  │  - Cookie Authentication                             │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                   │
│         ┌────────────────┼────────────────┐                │
│         │                │                │                 │
│         ▼                ▼                ▼                 │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐          │
│  │ Node 1   │     │ Node 2   │     │ Node 3   │          │
│  │ (geth)   │     │ (geth)   │     │ (geth)   │          │
│  │ DHT: ✓   │     │ DHT: ✓   │     │ DHT: ✓   │          │
│  │ RPC: 8545│     │ RPC: 8545│     │ RPC: 8545│          │
│  └──────────┘     └──────────┘     └──────────┘          │
│         │                │                │                 │
│         ▼                ▼                ▼                 │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐          │
│  │ Peer     │     │ Peer     │     │ Peer     │          │
│  │ Manager 1│     │ Manager 2│     │ Manager 3│          │
│  │ (sidecar)│     │ (sidecar)│     │ (sidecar)│          │
│  └──────────┘     └──────────┘     └──────────┘          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Components

1. **Tor Daemon**: Provides SOCKS5 proxy and control port for all nodes
2. **Gethrelay Nodes**: P2P nodes with DHT discovery enabled
3. **Peer Managers**: Sidecars that promote DHT-discovered .onion peers to trusted status

### Discovery Flow

1. **Startup**: All nodes start with DHT discovery enabled (`--v5disc`)
2. **DHT Discovery**: Nodes discover each other via discv5 over Tor
3. **Monitoring**: Peer managers monitor connected peers via `admin_peers`
4. **Promotion**: .onion peers are promoted via `admin_addTrustedPeer`
5. **Persistence**: Trusted peers persist across restarts

## Prerequisites

### System Requirements

- **OS**: Linux, macOS, or Windows with WSL2
- **CPU**: 2+ cores
- **RAM**: 8GB minimum, 16GB recommended
- **Disk**: 20GB minimum (SSD recommended)
- **Network**: Stable internet connection

### Software Requirements

- **Docker**: 20.10 or later
- **Docker Compose**: 1.29 or later (or Docker Compose V2)

### Verify Installation

```bash
# Check Docker version
docker --version
# Expected: Docker version 20.10.0 or later

# Check Docker Compose version
docker-compose --version
# Expected: docker-compose version 1.29.0 or later
# OR
docker compose version
# Expected: Docker Compose version v2.0.0 or later

# Verify Docker is running
docker ps
# Should show running containers (or empty list if none)
```

## Installation

### 1. Clone Repository

```bash
# If you have the full repository
cd go-ethereum/deployment/docker-compose

# OR if deploying standalone
git clone https://github.com/your-org/go-ethereum.git
cd go-ethereum/deployment/docker-compose
```

### 2. Prepare Environment

```bash
# Make all scripts executable
chmod +x scripts/*.sh

# Optional: Copy and customize environment variables
cp .env.example .env
nano .env  # Edit configuration as needed
```

### 3. Build Images

```bash
# Build gethrelay images
docker-compose build

# OR pull pre-built images (if available)
docker-compose pull
```

## Configuration

### Environment Variables

The `.env` file (created from `.env.example`) contains all configuration options:

#### Key Configuration Options

```bash
# Tor Configuration
TOR_SOCKS_PORT=9050          # SOCKS5 proxy port
TOR_CONTROL_PORT=9051        # Control port

# Gethrelay Configuration
GETH_VERBOSITY=3             # 0-5 (3=info, 4=debug)
GETH_MAX_PEERS=50            # Maximum peers
GETH_NETWORK_ID=1            # 1=mainnet

# Peer Manager Configuration
PEER_CHECK_INTERVAL=30       # Check interval (seconds)
```

### Production Configuration

For production deployments, consider:

1. **Resource Limits**: Add CPU/memory limits to services
2. **Logging**: Configure log rotation and aggregation
3. **Monitoring**: Enable metrics and alerting
4. **Backup**: Configure volume backups
5. **Security**: Review security settings

Example production additions to `docker-compose.yml`:

```yaml
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
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"
```

## Deployment

### Quick Start

```bash
# Start all services
docker-compose up -d

# Watch logs (all services)
docker-compose logs -f

# Watch logs (specific service)
docker-compose logs -f gethrelay-1
```

### Development Deployment

For development with exposed ports and verbose logging:

```bash
# Start with development overrides
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Access RPC from host
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}'
```

### Staged Deployment

For controlled rollout:

```bash
# Step 1: Start Tor first
docker-compose up -d tor

# Wait for Tor to be healthy
docker-compose ps tor

# Step 2: Start first node
docker-compose up -d gethrelay-1 peer-manager-1

# Step 3: Start remaining nodes
docker-compose up -d gethrelay-2 peer-manager-2 gethrelay-3 peer-manager-3
```

## Validation

### Automated Validation

Use the provided validation script:

```bash
./scripts/validate-deployment.sh
```

This script performs 27 automated tests including:
- Container health checks
- Port accessibility
- .onion address generation
- Peer connectivity
- Error log analysis

### Manual Validation

#### 1. Check All Containers Running

```bash
docker-compose ps

# Expected output:
# NAME                STATE         STATUS
# gethrelay-1         running       healthy
# gethrelay-2         running       healthy
# gethrelay-3         running       healthy
# peer-manager-1      running
# peer-manager-2      running
# peer-manager-3      running
# tor-proxy           running       healthy
```

#### 2. Verify Tor Connectivity

```bash
# Check Tor SOCKS5 proxy
docker-compose exec tor sh -c 'nc -z localhost 9050 && echo "SOCKS5 OK"'

# Check Tor control port
docker-compose exec tor sh -c 'nc -z localhost 9051 && echo "Control OK"'
```

#### 3. Get Node .onion Addresses

```bash
for i in 1 2 3; do
  echo "Node $i:"
  docker-compose exec gethrelay-$i sh -c '
    wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'\'' \
    --header="Content-Type: application/json" \
    http://127.0.0.1:8545 | grep -o "[a-z0-9]\{56\}\.onion"
  '
done
```

#### 4. Check Peer Counts

```bash
for i in 1 2 3; do
  echo "Node $i peer count:"
  docker-compose exec gethrelay-$i sh -c '
    wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
    --header="Content-Type: application/json" \
    http://127.0.0.1:8545
  '
done
```

Expected result: Each node should have 2+ peers after 2-3 minutes.

## Monitoring

### Real-time Dashboard

Use the monitoring script for a live dashboard:

```bash
./scripts/monitor.sh
```

This displays:
- Node status and health
- Peer counts (total and trusted)
- .onion addresses
- Resource usage
- Discovery summary

### Log Monitoring

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f gethrelay-1

# Tail last 100 lines
docker-compose logs --tail=100 peer-manager-1

# Follow peer discovery
docker-compose logs -f peer-manager-1 peer-manager-2 peer-manager-3 | grep "promoted"
```

### Metrics Collection

For production monitoring, integrate with:

- **Prometheus**: Scrape metrics from gethrelay
- **Grafana**: Visualize metrics
- **Loki**: Aggregate logs

Example Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'gethrelay'
    static_configs:
      - targets: ['localhost:6060']  # If metrics enabled
```

## Maintenance

### Regular Operations

#### Update Images

```bash
# Pull latest images
docker-compose pull

# Rebuild local images
docker-compose build

# Restart with new images
docker-compose up -d
```

#### Restart Services

```bash
# Restart all services
docker-compose restart

# Restart specific service
docker-compose restart gethrelay-1

# Graceful restart (stop then start)
docker-compose stop gethrelay-1
docker-compose start gethrelay-1
```

#### View Resource Usage

```bash
# Real-time stats
docker stats

# Disk usage
docker system df

# Volume usage
docker volume ls
docker volume inspect docker-compose_geth-data-1
```

### Backup and Restore

#### Backup Node Data

```bash
# Backup node 1 data
docker run --rm \
  -v docker-compose_geth-data-1:/data \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/geth-1-$(date +%Y%m%d).tar.gz /data

# Backup all nodes
for i in 1 2 3; do
  docker run --rm \
    -v docker-compose_geth-data-$i:/data \
    -v $(pwd)/backups:/backup \
    alpine tar czf /backup/geth-$i-$(date +%Y%m%d).tar.gz /data
done
```

#### Restore Node Data

```bash
# Stop services
docker-compose down

# Restore node 1
docker run --rm \
  -v docker-compose_geth-data-1:/data \
  -v $(pwd)/backups:/backup \
  alpine tar xzf /backup/geth-1-20231113.tar.gz -C /

# Restart services
docker-compose up -d
```

### Scaling

#### Add 4th Node

1. Add to `docker-compose.yml`:

```yaml
gethrelay-4:
  # ... (copy from gethrelay-3, update names/volumes)

peer-manager-4:
  # ... (copy from peer-manager-3, update names)

volumes:
  geth-data-4:
```

2. Start new services:

```bash
docker-compose up -d gethrelay-4 peer-manager-4
```

#### Remove Node

```bash
# Stop and remove service
docker-compose stop gethrelay-3 peer-manager-3
docker-compose rm -f gethrelay-3 peer-manager-3

# Optionally remove volume
docker volume rm docker-compose_geth-data-3
```

## Troubleshooting

See [README.md](README.md#troubleshooting) for comprehensive troubleshooting guide.

### Quick Diagnostics

```bash
# Check all container status
docker-compose ps

# Check logs for errors
docker-compose logs | grep -i "error\|fatal\|panic"

# Check Tor connectivity
docker-compose exec gethrelay-1 sh -c 'nc -z tor 9050 && echo "Tor OK"'

# Check RPC
docker-compose exec gethrelay-1 sh -c 'nc -z localhost 8545 && echo "RPC OK"'

# Check peer manager activity
docker-compose logs peer-manager-1 --tail=50
```

## Security

### Production Security Checklist

- [ ] RPC bound to `127.0.0.1` only
- [ ] API modules limited to `admin,eth,net`
- [ ] Tor cookie authentication enabled
- [ ] Volume permissions set correctly
- [ ] Container images from trusted sources
- [ ] Regular security updates applied
- [ ] Firewall rules configured
- [ ] Logs monitored for suspicious activity

### Security Best Practices

1. **Network Isolation**: Use dedicated Docker network
2. **RPC Access**: Never expose to `0.0.0.0` in production
3. **API Modules**: Only enable necessary modules
4. **Tor Security**: Keep cookie authentication enabled
5. **Volume Permissions**: Set appropriate file permissions
6. **Image Security**: Regularly update base images
7. **Log Monitoring**: Monitor for security events

### Security Hardening

```yaml
# Example hardened configuration
services:
  gethrelay-1:
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
```

## Additional Resources

- [Quick Start Guide](QUICKSTART.md)
- [README.md](README.md)
- [Research Document](../../docker-compose-service-discovery-research.md)
- [Docker Documentation](https://docs.docker.com/)
- [Gethrelay Documentation](../../README.md)

## Support

For issues and questions:

1. Check [Troubleshooting](#troubleshooting) section
2. Review logs: `docker-compose logs -f`
3. Run validation: `./scripts/validate-deployment.sh`
4. Check GitHub issues
5. Contact maintainers

## License

This deployment configuration is part of the go-ethereum project.
