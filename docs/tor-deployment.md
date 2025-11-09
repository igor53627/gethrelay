# Tor Integration Deployment Guide

## Overview

This guide provides best practices for deploying gethrelay with Tor integration in production environments. It covers installation, configuration, monitoring, high availability, and security hardening.

## Production Deployment Architecture

### Recommended Architecture

```
┌─────────────────────────────────────────────────┐
│ Load Balancer / Reverse Proxy                   │
│  - HAProxy or Nginx                             │
│  - SSL termination                              │
│  - Rate limiting                                │
└────────────────┬────────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
┌───────▼────────┐ ┌──────▼─────────┐
│ Gethrelay #1   │ │ Gethrelay #2   │
│  - Clearnet    │ │  - Tor-only    │
│  - High perf   │ │  - Privacy     │
└───────┬────────┘ └──────┬─────────┘
        │                 │
┌───────▼─────────────────▼─────────┐
│ Shared Tor Daemon (HA)            │
│  - SOCKS5: 127.0.0.1:9050         │
│  - Control: 127.0.0.1:9051        │
│  - HA: Keepalived or similar      │
└───────────────────────────────────┘
```

**Key components:**
1. **Load balancer** - Distributes RPC requests
2. **Gethrelay clearnet instance** - High-performance, low-latency
3. **Gethrelay Tor instance** - Privacy-focused, censorship-resistant
4. **Shared Tor daemon** - Centralized Tor proxy with HA

## Installation

### Prerequisites

**System Requirements:**
- Linux (Ubuntu 22.04 LTS or newer recommended)
- 4+ CPU cores
- 8+ GB RAM
- 100+ GB SSD storage
- 1 Gbps network connection

**Software Dependencies:**
- Go 1.21+ (for building from source)
- Tor 0.4.8+ (latest stable version)
- systemd (for service management)

### Install Tor

**Debian/Ubuntu:**
```bash
# Add Tor repository
sudo apt-get update
sudo apt-get install -y apt-transport-https
echo "deb https://deb.torproject.org/torproject.org $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/tor.list
wget -qO- https://deb.torproject.org/torproject.org/A3C4F0F979CAA22CDBA8F512EE8CBC9E886DDD89.asc | gpg --dearmor | sudo tee /usr/share/keyrings/tor-archive-keyring.gpg >/dev/null

# Install Tor
sudo apt-get update
sudo apt-get install -y tor deb.torproject.org-keyring

# Verify installation
tor --version
```

**RHEL/CentOS:**
```bash
# Enable EPEL repository
sudo yum install -y epel-release

# Install Tor
sudo yum install -y tor

# Verify installation
tor --version
```

**Docker:**
```dockerfile
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y tor
# Configure Tor (see configuration section)
EXPOSE 9050 9051
CMD ["tor", "-f", "/etc/tor/torrc"]
```

### Install Gethrelay

**From source:**
```bash
# Clone repository
git clone https://github.com/ethereum/go-ethereum.git
cd go-ethereum

# Build gethrelay
make gethrelay

# Verify installation
./build/bin/gethrelay version
```

**Binary release:**
```bash
# Download latest release
wget https://github.com/ethereum/go-ethereum/releases/download/v1.x.x/gethrelay-linux-amd64.tar.gz

# Extract
tar -xzf gethrelay-linux-amd64.tar.gz

# Verify
./gethrelay version
```

## Configuration

### Tor Configuration

**Production `/etc/tor/torrc`:**

```
# Network settings
SOCKSPort 127.0.0.1:9050
ControlPort 127.0.0.1:9051

# Authentication
CookieAuthentication 1
CookieAuthFile /var/run/tor/control.authcookie
CookieAuthFileGroupReadable 1

# Performance tuning
NumEntryGuards 8
MaxCircuitDirtiness 600
CircuitBuildTimeout 60
LearnCircuitBuildTimeout 0

# Logging
Log notice file /var/log/tor/notices.log
Log warn file /var/log/tor/warnings.log

# Hidden service for P2P (optional)
HiddenServiceDir /var/lib/tor/gethrelay/
HiddenServicePort 30303 127.0.0.1:30303

# Security hardening
DisableAllSwap 1
HardwareAccel 1
SafeSocks 1
TestSocks 1
WarnPlaintextPorts 23,109,110,143
```

**Key settings explained:**

- `SOCKSPort 127.0.0.1:9050` - SOCKS5 proxy for gethrelay
- `ControlPort 127.0.0.1:9051` - Control port for hidden service creation
- `CookieAuthentication 1` - Secure authentication
- `NumEntryGuards 8` - More entry guards for reliability
- `MaxCircuitDirtiness 600` - Keep circuits for 10 minutes (balance privacy/performance)
- `DisableAllSwap 1` - Prevent memory swapping (security)

**Apply configuration:**
```bash
sudo systemctl restart tor
sudo systemctl enable tor

# Verify Tor is running
sudo systemctl status tor
netstat -an | grep 9050
```

### Gethrelay Configuration

**Systemd service file:** `/etc/systemd/system/gethrelay.service`

```ini
[Unit]
Description=Gethrelay Ethereum P2P Relay Node
After=network.target tor.service
Requires=tor.service

[Service]
Type=simple
User=gethrelay
Group=gethrelay
ExecStart=/usr/local/bin/gethrelay \
  --port=30303 \
  --maxpeers=200 \
  --networkid=1 \
  --tor-proxy=127.0.0.1:9050 \
  --prefer-tor \
  --bootnodes="<bootnodes>" \
  --rpc.upstream=https://ethereum-rpc.publicnode.com \
  --datadir=/var/lib/gethrelay
Restart=always
RestartSec=10
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gethrelay

[Install]
WantedBy=multi-user.target
```

**Create gethrelay user:**
```bash
sudo useradd -r -s /bin/false gethrelay
sudo mkdir -p /var/lib/gethrelay
sudo chown gethrelay:gethrelay /var/lib/gethrelay

# Add gethrelay user to tor group (for cookie access)
sudo usermod -a -G debian-tor gethrelay
```

**Start gethrelay:**
```bash
sudo systemctl daemon-reload
sudo systemctl start gethrelay
sudo systemctl enable gethrelay

# Verify
sudo systemctl status gethrelay
sudo journalctl -u gethrelay -f
```

### Configuration Modes for Production

#### Mode 1: High Performance (Prefer Tor)

**Use case:** General production deployment

```bash
gethrelay \
  --tor-proxy=127.0.0.1:9050 \
  --prefer-tor \
  --maxpeers=200 \
  --port=30303
```

**Characteristics:**
- Tor when available, clearnet fallback
- Good balance of privacy and performance
- Suitable for most deployments

#### Mode 2: Privacy-First (Tor-Only)

**Use case:** Privacy-critical deployments, censored networks

```bash
gethrelay \
  --tor-proxy=127.0.0.1:9050 \
  --only-onion \
  --maxpeers=100 \
  --port=30303 \
  --bootnodes="<tor-only-bootnodes>"
```

**Characteristics:**
- Maximum privacy
- Slower performance
- Smaller peer pool
- Requires Tor-enabled bootnodes

#### Mode 3: Hybrid Deployment

**Use case:** Large-scale deployments with separate instances

**Clearnet instance:**
```bash
gethrelay \
  --port=30303 \
  --maxpeers=200 \
  --datadir=/var/lib/gethrelay-clearnet
```

**Tor instance:**
```bash
gethrelay \
  --tor-proxy=127.0.0.1:9050 \
  --only-onion \
  --port=30304 \
  --maxpeers=100 \
  --datadir=/var/lib/gethrelay-tor
```

**Characteristics:**
- Separate identities (no linkability)
- Optimized for each use case
- Higher resource usage

## Monitoring

### Health Checks

**Basic health check script:** `/usr/local/bin/gethrelay-health.sh`

```bash
#!/bin/bash

# Check Tor SOCKS5 proxy
if ! curl --socks5 127.0.0.1:9050 -s https://check.torproject.org | grep -q "Congratulations"; then
  echo "ERROR: Tor SOCKS5 proxy not working"
  exit 1
fi

# Check gethrelay is running
if ! systemctl is-active --quiet gethrelay; then
  echo "ERROR: Gethrelay service not running"
  exit 1
fi

# Check peer count
PEER_COUNT=$(gethrelay admin.peers 2>/dev/null | jq length)
if [ "$PEER_COUNT" -lt 5 ]; then
  echo "WARNING: Low peer count ($PEER_COUNT)"
  exit 1
fi

echo "OK: All health checks passed"
exit 0
```

**Run periodically:**
```bash
# Add to crontab
*/5 * * * * /usr/local/bin/gethrelay-health.sh || /usr/local/bin/alert.sh "Gethrelay health check failed"
```

### Metrics Collection

**Prometheus exporter for Tor:**

```bash
# Install tor-prometheus-exporter
pip3 install prometheus-tor-exporter

# Run exporter
tor-prometheus-exporter --tor-control-address 127.0.0.1:9051
```

**Key metrics to monitor:**

1. **Tor metrics:**
   - Circuit build success rate
   - Active circuits
   - SOCKS5 connection count
   - Bandwidth usage

2. **Gethrelay metrics:**
   - Peer count (total, Tor, clearnet)
   - Connection establishment latency
   - Fallback events (Tor → clearnet)
   - Data throughput

3. **System metrics:**
   - CPU usage
   - Memory usage
   - Network I/O
   - Disk I/O

**Example Prometheus scrape config:**

```yaml
scrape_configs:
  - job_name: 'tor'
    static_configs:
      - targets: ['localhost:9099']

  - job_name: 'gethrelay'
    static_configs:
      - targets: ['localhost:6060']
```

### Logging

**Centralized logging with rsyslog:**

**`/etc/rsyslog.d/gethrelay.conf`:**
```
if $programname == 'gethrelay' then /var/log/gethrelay/gethrelay.log
& stop

if $programname == 'tor' then /var/log/tor/tor.log
& stop
```

**Log rotation:** `/etc/logrotate.d/gethrelay`

```
/var/log/gethrelay/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 gethrelay gethrelay
    sharedscripts
    postrotate
        systemctl reload gethrelay
    endscript
}
```

### Alerting

**Example alert rules (Prometheus Alertmanager):**

```yaml
groups:
  - name: gethrelay
    rules:
      - alert: TorProxyDown
        expr: up{job="tor"} == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Tor proxy is down"

      - alert: LowPeerCount
        expr: gethrelay_peer_count < 10
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Gethrelay has low peer count ({{ $value }})"

      - alert: HighFallbackRate
        expr: rate(gethrelay_tor_fallback_count[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High Tor fallback rate ({{ $value }}/s)"
```

## High Availability

### Tor HA Configuration

**Option 1: Multiple Tor instances with load balancing**

Run multiple Tor daemons on different ports:

```bash
# Tor instance 1
SOCKSPort 127.0.0.1:9050

# Tor instance 2
SOCKSPort 127.0.0.1:9051

# Tor instance 3
SOCKSPort 127.0.0.1:9052
```

Use HAProxy to load balance:

```
frontend tor_proxy
    bind 127.0.0.1:9050
    mode tcp
    default_backend tor_pool

backend tor_pool
    mode tcp
    balance roundrobin
    server tor1 127.0.0.1:9150 check
    server tor2 127.0.0.1:9151 check
    server tor3 127.0.0.1:9152 check
```

**Option 2: Tor daemon failover with Keepalived**

**Master node:**
```
vrrp_instance TOR_PROXY {
    state MASTER
    interface eth0
    virtual_router_id 51
    priority 100
    virtual_ipaddress {
        10.0.0.100/24
    }
}
```

**Backup node:**
```
vrrp_instance TOR_PROXY {
    state BACKUP
    interface eth0
    virtual_router_id 51
    priority 99
    virtual_ipaddress {
        10.0.0.100/24
    }
}
```

Configure gethrelay to use VIP:
```bash
gethrelay --tor-proxy=10.0.0.100:9050
```

### Gethrelay HA Configuration

**Option 1: Active-active with different identities**

Run multiple gethrelay instances with separate data directories:

```bash
# Instance 1 (clearnet-focused)
gethrelay --datadir=/var/lib/gethrelay-1 --port=30303 --tor-proxy=127.0.0.1:9050

# Instance 2 (Tor-focused)
gethrelay --datadir=/var/lib/gethrelay-2 --port=30304 --tor-proxy=127.0.0.1:9050 --prefer-tor
```

**Option 2: Active-passive with shared storage**

Use shared storage (NFS, Ceph) for data directory:

**Active node:**
```bash
gethrelay --datadir=/mnt/shared/gethrelay --port=30303
```

**Passive node (standby):**
- Wait for active node failure
- Mount shared storage
- Start gethrelay with same datadir

**Note:** Requires fencing to prevent split-brain.

## Security Hardening

### Firewall Configuration

**iptables rules:**

```bash
# Allow P2P port (inbound)
sudo iptables -A INPUT -p tcp --dport 30303 -j ACCEPT

# Allow established connections
sudo iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Allow localhost (for Tor SOCKS5)
sudo iptables -A INPUT -i lo -j ACCEPT

# Drop everything else
sudo iptables -A INPUT -j DROP

# Save rules
sudo iptables-save | sudo tee /etc/iptables/rules.v4
```

**ufw (simplified firewall):**

```bash
sudo ufw allow 30303/tcp
sudo ufw enable
```

### AppArmor/SELinux

**AppArmor profile for Tor:** `/etc/apparmor.d/usr.bin.tor`

```
#include <tunables/global>

/usr/bin/tor {
  #include <abstractions/base>
  #include <abstractions/nameservice>

  /var/lib/tor/** rw,
  /var/log/tor/* w,
  /var/run/tor/* rw,
  /etc/tor/* r,

  deny /proc/sys/kernel/** w,
  deny /sys/** w,
}
```

**Enable AppArmor profile:**
```bash
sudo apparmor_parser -r /etc/apparmor.d/usr.bin.tor
```

### Secret Management

**Tor cookie authentication:**

```bash
# Ensure cookie is only readable by gethrelay user
sudo chown debian-tor:debian-tor /var/run/tor/control.authcookie
sudo chmod 640 /var/run/tor/control.authcookie
sudo usermod -a -G debian-tor gethrelay
```

**Hidden service keys:**

```bash
# Protect hidden service keys
sudo chown -R debian-tor:debian-tor /var/lib/tor/gethrelay/
sudo chmod 700 /var/lib/tor/gethrelay/
sudo chmod 600 /var/lib/tor/gethrelay/hs_ed25519_secret_key
```

### Network Segmentation

**Separate network zones:**

```
┌─────────────────────┐
│ Public Internet     │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│ DMZ (Load Balancer) │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│ App Zone (Gethrelay)│
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│ Service Zone (Tor)  │
└─────────────────────┘
```

**Benefits:**
- Limit attack surface
- Contain breaches
- Enforce least privilege

## Disaster Recovery

### Backup Strategy

**What to backup:**

1. **Gethrelay data directory**
   - Node key (identity)
   - Configuration files
   - ENR data

2. **Tor hidden service keys**
   - `hs_ed25519_secret_key`
   - `hs_ed25519_public_key`
   - `hostname` file

**Backup script:** `/usr/local/bin/gethrelay-backup.sh`

```bash
#!/bin/bash

BACKUP_DIR="/backup/gethrelay/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# Backup gethrelay data
tar -czf "$BACKUP_DIR/gethrelay-data.tar.gz" /var/lib/gethrelay/

# Backup Tor hidden service
tar -czf "$BACKUP_DIR/tor-hidden-service.tar.gz" /var/lib/tor/gethrelay/

# Encrypt backups
gpg --encrypt --recipient backup@example.com "$BACKUP_DIR/gethrelay-data.tar.gz"
gpg --encrypt --recipient backup@example.com "$BACKUP_DIR/tor-hidden-service.tar.gz"

# Remove unencrypted backups
rm "$BACKUP_DIR"/*.tar.gz

# Retention: keep last 30 days
find /backup/gethrelay/ -type d -mtime +30 -exec rm -rf {} +
```

**Schedule backups:**
```bash
# Daily at 2 AM
0 2 * * * /usr/local/bin/gethrelay-backup.sh
```

### Recovery Procedures

**Scenario 1: Tor daemon failure**

```bash
# Restore Tor service
sudo systemctl restart tor

# Gethrelay will automatically reconnect
# Check logs for recovery
sudo journalctl -u gethrelay -f
```

**Scenario 2: Gethrelay crash**

```bash
# Restart gethrelay service
sudo systemctl restart gethrelay

# Verify peer connections restored
gethrelay admin.peers
```

**Scenario 3: Data corruption**

```bash
# Stop services
sudo systemctl stop gethrelay tor

# Restore from backup
sudo tar -xzf /backup/gethrelay/20250109/gethrelay-data.tar.gz -C /
sudo tar -xzf /backup/gethrelay/20250109/tor-hidden-service.tar.gz -C /

# Fix permissions
sudo chown -R gethrelay:gethrelay /var/lib/gethrelay/
sudo chown -R debian-tor:debian-tor /var/lib/tor/gethrelay/

# Start services
sudo systemctl start tor
sudo systemctl start gethrelay
```

## Performance Tuning

### Tor Daemon Tuning

**`/etc/tor/torrc` optimizations:**

```
# Increase entry guards for reliability
NumEntryGuards 8

# Reduce circuit build timeout (trade privacy for speed)
CircuitBuildTimeout 30
LearnCircuitBuildTimeout 0

# Increase bandwidth limits
RelayBandwidthRate 10 MBytes
RelayBandwidthBurst 20 MBytes

# Optimize CPU usage
HardwareAccel 1
NumCPUs 4
```

### Gethrelay Tuning

**Increase peer limits:**
```bash
gethrelay --maxpeers=500 --tor-proxy=127.0.0.1:9050
```

**Use prefer-tor for performance:**
```bash
gethrelay --prefer-tor --tor-proxy=127.0.0.1:9050
```

### System Tuning

**Increase file descriptor limits:**

```bash
# /etc/security/limits.conf
gethrelay soft nofile 65535
gethrelay hard nofile 65535
```

**TCP tuning:**

```bash
# /etc/sysctl.conf
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 1200
net.core.somaxconn = 1024
net.ipv4.tcp_max_syn_backlog = 2048
```

Apply:
```bash
sudo sysctl -p
```

## Troubleshooting Production Issues

See [tor-troubleshooting.md](./tor-troubleshooting.md) for detailed troubleshooting.

**Common production issues:**

1. **Tor proxy overload** - Use multiple Tor instances with load balancing
2. **Hidden service descriptor fetch failures** - Increase descriptor timeout
3. **Circuit build failures** - Use more entry guards, reduce build timeout
4. **High latency** - Use `--prefer-tor` instead of `--only-onion`
5. **Low peer count** - Ensure bootnodes have .onion addresses

## Compliance and Regulations

### Legal Considerations

**Tor usage may be:**
- Illegal in some jurisdictions (China, Iran, etc.)
- Monitored by intelligence agencies
- Subject to export controls

**Recommendations:**
- Consult legal counsel before deployment
- Understand local regulations
- Document compliance measures

### Data Retention

**Logs to retain:**
- Connection logs (peer IPs, timestamps)
- Error logs (troubleshooting)

**Logs to avoid:**
- Tor circuit details (privacy)
- .onion addresses (user privacy)

**Retention period:** 30-90 days (balance compliance and privacy)

## Summary

**Deployment checklist:**

- [ ] Install Tor daemon (latest stable version)
- [ ] Configure Tor with production settings
- [ ] Install gethrelay (from source or binary)
- [ ] Create gethrelay systemd service
- [ ] Configure firewall rules
- [ ] Set up monitoring (Prometheus, logs)
- [ ] Configure alerting (PagerDuty, email)
- [ ] Implement backup strategy
- [ ] Document recovery procedures
- [ ] Test failover scenarios
- [ ] Security hardening (AppArmor, SELinux)
- [ ] Performance tuning
- [ ] Legal compliance review

---

**Version:** 1.0
**Last Updated:** 2025-11-09
**Status:** Production-ready
**Tested on:** Ubuntu 22.04 LTS, Debian 12, RHEL 9
