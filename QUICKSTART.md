# Gethrelay Quick Start

Get your gethrelay node running in 5 minutes with Docker Compose.

## Quick Deployment

### Prerequisites
- Docker 20.10+
- Docker Compose 1.29+
- 8GB RAM minimum
- 20GB disk space

### 1. One-Line Start

```bash
cd deployment/docker-compose && chmod +x scripts/*.sh && docker-compose up -d
```

### 2. Verify Deployment

```bash
# Check all containers are running
docker-compose ps

# Watch logs
docker-compose logs -f
```

### 3. Check Peer Connections

```bash
# Check peer count
docker-compose exec gethrelay-1 sh -c '
  wget -q -O - --post-data='\''{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'\'' \
  --header="Content-Type: application/json" \
  http://127.0.0.1:8545
'
```

## Alternative Deployment Methods

### Local Binary Build

```bash
# Build gethrelay
make gethrelay

# Run locally
./gethrelay --chain mainnet
```

### Docker Image Build

```bash
# Build image
docker build -f Dockerfile -t gethrelay:latest .

# Run container
docker run -d \
  -p 30303:30303 \
  -p 8545:8545 \
  gethrelay:latest --chain mainnet
```

## Documentation

- **Complete Setup**: [deployment/docker-compose/README.md](deployment/docker-compose/README.md)
- **5-Minute Guide**: [deployment/docker-compose/QUICKSTART.md](deployment/docker-compose/QUICKSTART.md)
- **Full Documentation**: [cmd/gethrelay/README.md](cmd/gethrelay/README.md)
- **Build Scripts**: [scripts/README.md](scripts/README.md)

## Troubleshooting

### Docker Compose Issues?
See [deployment/docker-compose/README.md](deployment/docker-compose/README.md#troubleshooting)

### Build Fails?
- Check Go version: `go version` (requires 1.24+)
- Check dependencies: `go mod download`

### Need Help?
- Review [cmd/gethrelay/README.md](cmd/gethrelay/README.md)
- Check issues: https://github.com/igor53627/gethrelay/issues
