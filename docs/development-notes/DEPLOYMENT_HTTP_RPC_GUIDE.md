# gethrelay HTTP RPC Deployment Guide

## Quick Start

gethrelay now supports HTTP RPC endpoints with configurable upstream proxy. This guide covers deployment configurations.

## Basic Usage

### Default Configuration (localhost:8545)
```bash
gethrelay --chain=mainnet
```
- Listens on: `localhost:8545`
- Proxies to: `https://ethereum-rpc.publicnode.com`
- APIs: `eth,net,web3`

### Custom Port
```bash
gethrelay --chain=mainnet --http.port=8080
```
- Listens on: `localhost:8080`

### External Access (Docker/K8s)
```bash
gethrelay --chain=mainnet --http.addr=0.0.0.0 --http.port=8545
```
- Listens on: `0.0.0.0:8545` (accessible from outside container)

### Custom Upstream RPC
```bash
gethrelay --chain=mainnet --rpc.upstream=https://eth-mainnet.g.alchemy.com/v2/YOUR_KEY
```
- Uses your preferred RPC provider

## Docker Deployment

### docker-compose.yml
```yaml
version: '3.8'
services:
  gethrelay:
    image: ghcr.io/username/gethrelay:latest
    ports:
      - "8545:8545"  # RPC port
      - "30303:30303"  # P2P port
    command:
      - --chain=mainnet
      - --http.addr=0.0.0.0
      - --http.port=8545
      - --http.api=eth,net,web3
      - --rpc.upstream=https://ethereum-rpc.publicnode.com
      - --maxpeers=50
```

### Dockerfile
```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o gethrelay ./cmd/gethrelay

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/gethrelay /usr/local/bin/
EXPOSE 8545 30303
ENTRYPOINT ["gethrelay"]
```

## Kubernetes Deployment

### deployment.yaml
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gethrelay
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gethrelay
  template:
    metadata:
      labels:
        app: gethrelay
    spec:
      containers:
      - name: gethrelay
        image: ghcr.io/username/gethrelay:latest
        ports:
        - containerPort: 8545
          name: rpc
        - containerPort: 30303
          name: p2p
        args:
        - --chain=mainnet
        - --http.addr=0.0.0.0
        - --http.port=8545
        - --http.api=eth,net,web3
        - --rpc.upstream=https://ethereum-rpc.publicnode.com
        - --maxpeers=50
        env:
        - name: GETHRELAY_RPC_UPSTREAM
          valueFrom:
            secretKeyRef:
              name: gethrelay-secrets
              key: upstream-rpc-url
---
apiVersion: v1
kind: Service
metadata:
  name: gethrelay-rpc
spec:
  type: LoadBalancer
  selector:
    app: gethrelay
  ports:
  - port: 8545
    targetPort: 8545
    protocol: TCP
    name: rpc
```

## Environment Variables

All flags can be set via environment variables with `GETHRELAY_` prefix:
```bash
export GETHRELAY_HTTP_ADDR=0.0.0.0
export GETHRELAY_HTTP_PORT=8545
export GETHRELAY_HTTP_API=eth,net,web3
export GETHRELAY_RPC_UPSTREAM=https://ethereum-rpc.publicnode.com
gethrelay --chain=mainnet
```

## Vultr Deployment

### Using Docker Compose
```bash
# On Vultr instance
sudo apt update && sudo apt install -y docker.io docker-compose
git clone https://github.com/username/go-ethereum.git
cd go-ethereum

# Create docker-compose.yml
cat > docker-compose.yml <<EOF
version: '3.8'
services:
  gethrelay:
    image: ghcr.io/username/gethrelay:latest
    restart: unless-stopped
    ports:
      - "8545:8545"
      - "30303:30303"
    command:
      - --chain=mainnet
      - --http.addr=0.0.0.0
      - --http.port=8545
      - --rpc.upstream=https://ethereum-rpc.publicnode.com
EOF

# Start service
sudo docker-compose up -d

# Test RPC
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

## Testing RPC Endpoints

### Check Block Number
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545
```

### Check Chain ID
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
  http://localhost:8545
```

### Get Network Version
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' \
  http://localhost:8545
```

### Send Raw Transaction
```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["0x..."],"id":1}' \
  http://localhost:8545
```

## Available HTTP Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--http` | `false` | Enable HTTP-RPC server (optional, server always runs) |
| `--http.addr` | `localhost` | HTTP-RPC server listening interface |
| `--http.port` | `8545` | HTTP-RPC server listening port |
| `--http.api` | `eth,net,web3` | Comma-separated list of APIs to expose |
| `--rpc.upstream` | `https://ethereum-rpc.publicnode.com` | Upstream RPC endpoint URL |

## Tor Integration

Combine HTTP RPC with Tor for privacy-preserving relay:
```bash
gethrelay \
  --chain=mainnet \
  --http.addr=0.0.0.0 \
  --http.port=8545 \
  --tor-proxy=127.0.0.1:9050 \
  --only-onion \
  --staticnodes=enode://...@abcd1234.onion:30303
```

## Security Recommendations

1. **Production Deployments**:
   - Use `--http.addr=127.0.0.1` for local access only
   - Use reverse proxy (nginx/caddy) with TLS for external access
   - Implement rate limiting at reverse proxy level

2. **API Exposure**:
   - Limit APIs: `--http.api=eth,net,web3` (no admin/debug)
   - Never expose admin APIs publicly

3. **Upstream RPC**:
   - Use trusted upstream providers
   - Rotate API keys regularly
   - Consider multiple upstreams for redundancy

4. **Firewall Rules**:
   ```bash
   # Allow RPC only from specific IPs
   sudo ufw allow from 10.0.0.0/8 to any port 8545
   sudo ufw deny 8545
   ```

## Monitoring

Check logs for RPC proxy status:
```bash
docker logs -f gethrelay 2>&1 | grep -i "rpc"
```

Expected output:
```
INFO [11-13|06:56:13.582] Starting JSON-RPC proxy server upstream=https://ethereum-rpc.publicnode.com addr=0.0.0.0 port=8545
```

## Troubleshooting

### RPC Not Responding
```bash
# Check if port is listening
netstat -tlnp | grep 8545

# Test locally
curl http://localhost:8545 -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### Connection Refused
- Verify `--http.addr=0.0.0.0` for external access
- Check firewall rules
- Ensure port is exposed in Docker/K8s

### Invalid Method Errors
- Check `--http.api` includes required namespace
- Verify upstream RPC supports the method
- Check upstream API key is valid

## Next Steps

1. Build Docker image with HTTP RPC support
2. Push to container registry
3. Deploy to Vultr
4. Configure DNS/load balancer
5. Set up monitoring and alerts
