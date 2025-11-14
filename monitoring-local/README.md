# Local Grafana Monitoring for Remote Gethrelay Nodes

This setup allows you to run Grafana + Prometheus **locally** on your Mac and collect metrics from your **remote** gethrelay nodes running on `108.61.166.134`.

## Architecture

```
[Remote: 108.61.166.134]               [Local: Your Mac]
┌─────────────────────┐                ┌──────────────────┐
│ gethrelay-1 :6060   │───SSH Tunnel──>│ localhost:16060  │
│ gethrelay-2 :6060   │───SSH Tunnel──>│ localhost:26060  │
│ gethrelay-3 :6060   │───SSH Tunnel──>│ localhost:36060  │
└─────────────────────┘                └────────┬─────────┘
                                                 │
                                        ┌────────▼─────────┐
                                        │  Prometheus      │
                                        │  :9090           │
                                        └────────┬─────────┘
                                                 │
                                        ┌────────▼─────────┐
                                        │  Grafana         │
                                        │  :3000           │
                                        └──────────────────┘
```

## Prerequisites

- Docker Desktop running on your Mac
- SSH access to remote nodes (geth-onion-dev)
- Metrics enabled on remote gethrelay nodes

## Setup Instructions

### Step 1: Enable Metrics on Remote Nodes

SSH into the remote server and update the docker-compose configuration:

```bash
ssh geth-onion-dev
cd /root/gethrelay-docker
```

Edit `docker-compose.yml` and add these flags to each gethrelay service:

```yaml
services:
  gethrelay-1:
    command:
      - --metrics
      - --metrics.addr=0.0.0.0
      - --metrics.port=6060
      # ... existing flags ...

  gethrelay-2:
    command:
      - --metrics
      - --metrics.addr=0.0.0.0
      - --metrics.port=6060
      # ... existing flags ...

  gethrelay-3:
    command:
      - --metrics
      - --metrics.addr=0.0.0.0
      - --metrics.port=6060
      # ... existing flags ...
```

Restart the containers:

```bash
docker-compose down
docker-compose up -d
```

Verify metrics are exposed:

```bash
docker exec gethrelay-1 wget -q -O- http://localhost:6060/debug/metrics/prometheus | head
```

You should see output like:
```
# TYPE p2p_dials_tor_total counter
p2p_dials_tor_total 12
# TYPE p2p_peers_network_tor gauge
p2p_peers_network_tor 2
...
```

### Step 2: Create SSH Tunnels

From your **local Mac**, create SSH tunnels to forward remote metrics ports to localhost:

```bash
# Terminal 1: Tunnel for gethrelay-1
ssh -N -L 16060:172.20.0.11:6060 geth-onion-dev

# Terminal 2: Tunnel for gethrelay-2
ssh -N -L 26060:172.20.0.12:6060 geth-onion-dev

# Terminal 3: Tunnel for gethrelay-3
ssh -N -L 36060:172.20.0.13:6060 geth-onion-dev
```

**Or use a single command with background tunnels:**

```bash
ssh -f -N -L 16060:172.20.0.11:6060 geth-onion-dev
ssh -f -N -L 26060:172.20.0.12:6060 geth-onion-dev
ssh -f -N -L 36060:172.20.0.13:6060 geth-onion-dev
```

Verify tunnels are working:

```bash
curl -s http://localhost:16060/debug/metrics/prometheus | head
curl -s http://localhost:26060/debug/metrics/prometheus | head
curl -s http://localhost:36060/debug/metrics/prometheus | head
```

### Step 3: Start Local Monitoring Stack

From the `monitoring-local` directory:

```bash
cd /Users/user/pse/ethereum/go-ethereum/monitoring-local
docker-compose up -d
```

Check containers are running:

```bash
docker-compose ps
```

Expected output:
```
NAME                IMAGE                      STATUS
grafana-local       grafana/grafana:latest     Up
prometheus-local    prom/prometheus:latest     Up
```

### Step 4: Access Dashboards

**Prometheus UI**: http://localhost:9090
- Check targets: http://localhost:9090/targets
- All 3 gethrelay targets should show `UP`

**Grafana**: http://localhost:3000
- Username: `admin`
- Password: `admin`
- Pre-loaded dashboard: "Gethrelay Tor Metrics"

## Dashboard Features

The included Grafana dashboard shows:

1. **Peer Distribution by Network Type** - Tor vs Clearnet peers
2. **Peer Count Over Time** - Time series of peer connections
3. **Tor Dial Success Rate** - Percentage of successful Tor connections
4. **Tor Dial Rate** - Connection attempts per second
5. **Total Traffic by Network Type** - Bandwidth usage comparison
6. **Tor Traffic Rate** - Real-time Tor bytes/sec
7. **Clearnet Traffic Rate** - Real-time clearnet bytes/sec
8. **Traffic Distribution %** - Ingress traffic breakdown
9. **Peer Network Distribution** - Pie chart visualization

## Available Metrics

```
p2p/dials/tor/total           # Total Tor dial attempts
p2p/dials/tor/success         # Successful Tor connections
p2p/peers/network/tor         # Current Tor peer count
p2p/peers/network/clearnet    # Current clearnet peer count
p2p/traffic/tor/ingress       # Inbound Tor traffic (bytes)
p2p/traffic/tor/egress        # Outbound Tor traffic (bytes)
p2p/traffic/clearnet/ingress  # Inbound clearnet traffic (bytes)
p2p/traffic/clearnet/egress   # Outbound clearnet traffic (bytes)
```

## Troubleshooting

### Prometheus shows targets as DOWN

Check SSH tunnels are running:
```bash
ps aux | grep ssh | grep -E '16060|26060|36060'
```

If not running, recreate tunnels (see Step 2).

### No metrics data in Grafana

1. Check Prometheus is scraping: http://localhost:9090/targets
2. Verify metrics on remote nodes:
   ```bash
   ssh geth-onion-dev "docker exec gethrelay-1 wget -q -O- http://localhost:6060/debug/metrics/prometheus | head"
   ```
3. Check Grafana datasource: Grafana → Configuration → Data Sources → Prometheus

### Dashboard not loading

1. Restart Grafana:
   ```bash
   docker-compose restart grafana
   ```
2. Check dashboard provisioning:
   ```bash
   docker exec grafana-local ls -la /etc/grafana/provisioning/dashboards/
   ```

## Stopping the Monitoring Stack

```bash
cd /Users/user/pse/ethereum/go-ethereum/monitoring-local
docker-compose down
```

Kill SSH tunnels:
```bash
pkill -f 'ssh.*16060'
pkill -f 'ssh.*26060'
pkill -f 'ssh.*36060'
```

## Production Deployment

For production, consider:
- Deploy Prometheus + Grafana on the same server as gethrelay
- Use service discovery instead of static targets
- Enable authentication and HTTPS
- Set up alerting rules
- Configure retention policies

See the `feature/monitoring-prometheus-grafana` branch for Kubernetes deployment examples.

---

**Created**: 2025-11-13
**Remote Nodes**: 108.61.166.134 (geth-onion-dev)
**Dashboard Source**: `feature/monitoring-prometheus-grafana` branch
