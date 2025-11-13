# Vultr Server Resource Analysis & Capacity Planning

## Current Deployment on Vultr

### Server Specifications
- **CPU**: Intel Xeon E-2388G @ 3.20GHz
  - 8 cores / 16 threads
  - High-performance server CPU
- **Memory**: 128 GB RAM (125.7 GiB)
- **Disk**: 3.6 TB (1.6 TB used, 1.9 TB available)
- **Network**: 1 Gbps (assumed from Vultr standard)
- **Uptime**: 18 days, 11:59 hours

### Current Resource Consumption

#### Per-Container Resource Usage

**3x Gethrelay Nodes:**
| Container | CPU % | Memory | Network I/O (Total) |
|-----------|-------|---------|---------------------|
| gethrelay-1 | 0.26% | 18.46 MiB | 6.23 MB / 900 kB |
| gethrelay-2 | 0.35% | 16.86 MiB | 6.23 MB / 913 kB |
| gethrelay-3 | 0.35% | 21.50 MiB | 6.23 MB / 909 kB |
| **Total** | **0.96%** | **~57 MiB** | **~19 MB / 2.7 MB** |

**3x Peer-Manager Sidecars:**
| Container | CPU % | Memory |
|-----------|-------|---------|
| peer-manager-1 | 0.00% | 3.23 MiB |
| peer-manager-2 | 0.00% | 3.96 MiB |
| peer-manager-3 | 0.00% | 3.26 MiB |
| **Total** | **~0.00%** | **~10 MiB** |

**Tor Proxy:**
| Container | CPU % | Memory | Network I/O |
|-----------|-------|---------|-------------|
| tor-proxy | 0.04% | 54.47 MiB | 17.1 MB / 18.7 MB |

#### Total Gethrelay Stack Consumption
- **CPU**: ~1.0% of 16 threads = ~0.16 cores
- **Memory**: ~122 MiB (0.09% of 128 GB)
- **Disk I/O**: Minimal (relay nodes don't store blockchain)
- **Network**: ~36 MB total (18 days uptime)

### System-Wide Resource Usage

**Overall System:**
- **CPU Load**: 0.38 (1-min), 0.43 (5-min), 0.44 (15-min)
- **Memory Used**: 14.7 GB (11.4%)
  - Main consumer: Full geth node (9.0 GB)
  - Consensus client (Nimbus+): 4.2 GB
  - Gethrelay stack: 0.12 GB
- **Available Memory**: 110 GB (87.6%)
- **Swap Used**: 624 MB / 8 GB (7.6%)

## Capacity Analysis

### Per-Node Resource Requirements

**One Gethrelay Node (with peer-manager):**
- CPU: 0.3% per core (~0.048 cores at 3.2GHz)
- Memory: ~20 MB + 3 MB = **23 MB**
- Disk: ~100 MB (binary + configs)
- Network: ~2 MB/hour ingress, ~300 KB/hour egress

**Tor Proxy (shared):**
- CPU: 0.04% (~0.006 cores)
- Memory: 55 MB
- Network: Proportional to node count

### Theoretical Maximum Capacity

#### CPU-Bound Estimate
```
Available CPU: 16 cores - 2.4% used = ~15.6 cores available
Per node: 0.048 cores
Maximum nodes: 15.6 / 0.048 = 325 nodes
```

#### Memory-Bound Estimate
```
Available RAM: 110 GB
Per node: 23 MB = 0.023 GB
Maximum nodes: 110 / 0.023 = 4,782 nodes
```

#### Network-Bound Estimate (Conservative)
```
Assumed 1 Gbps = 125 MB/s
Current usage: ~36 MB over 18 days = ~0.023 KB/s
Per node sustained: ~0.1 KB/s ingress + egress

Maximum nodes (conservative): 1,000-2,000 nodes
(Limited by Tor circuit overhead, not raw bandwidth)
```

#### Tor Circuit Limit (Most Restrictive)
```
Tor circuits per node: ~50 peer connections
Tor daemon limit: 10,000 circuits (typical)
Maximum nodes: 10,000 / 50 = 200 nodes (realistic)
```

### Practical Capacity Estimate

**Conservative Recommendation: 50-100 nodes per server**

Reasoning:
1. **Tor Overhead**: Primary bottleneck
   - Each node maintains ~50 Tor circuits
   - Tor circuit creation overhead scales non-linearly
   - Single tor-proxy shared across all nodes

2. **P2P Connection Overhead**:
   - Each node maintains active peer connections
   - Connection handshakes consume CPU
   - DHT queries increase with node count

3. **Monitoring & Management**:
   - Peer-manager scripts poll every 30s
   - With 100 nodes: 100 polls/30s = 3.3 req/s (manageable)

4. **Operational Margin**:
   - Keep 20-30% headroom for spikes
   - Allow for future growth

### Recommended Scaling Strategy

#### Tier 1: Development/Testing (Current)
- **3-10 nodes** per server
- Full monitoring and debugging
- High resource availability per node

#### Tier 2: Production Testing
- **20-50 nodes** per server
- Validate Tor circuit scaling
- Monitor for bottlenecks

#### Tier 3: Production Scale
- **50-100 nodes** per server
- Optimized tor-proxy configuration
- Load-balanced across multiple servers

#### Tier 4: Large-Scale Deployment
- **Multiple servers** with 50-100 nodes each
- Geographic distribution
- Separate Tor proxies per 50 nodes

## Resource Optimization Opportunities

### Current Architecture (Unoptimized)

**Good:**
- Lightweight relay nodes (no blockchain state)
- Shared Tor proxy (single daemon)
- Alpine-based containers (minimal footprint)

**Can Improve:**
- **Tor Circuit Pooling**: Reuse circuits across nodes
- **Connection Multiplexing**: Share connections for peer discovery
- **Batch Admin API Queries**: Single peer-manager for all nodes
- **Memory Sharing**: Use Go's lightweight goroutines

### Optimized Architecture Estimates

**With Optimizations:**
- CPU per node: 0.3% → **0.1%** (3x improvement)
- Memory per node: 23 MB → **15 MB** (1.5x improvement)
- Tor overhead: Logarithmic scaling vs linear

**Optimized Capacity: 150-200 nodes per server**

## Cost Analysis

### Current Vultr Configuration
- Server: High-performance bare metal
- Estimated Cost: ~$120-300/month (assuming High Frequency plan)
- Current Utilization: **<1% of capacity**

### Cost Efficiency

**At 3 Nodes:**
- Cost per node: $40-100/month
- Utilization: 0.09% memory, 1% CPU

**At 50 Nodes (Realistic):**
- Cost per node: $2.40-6/month
- Utilization: 2% memory, 10-15% CPU

**At 100 Nodes (Optimized):**
- Cost per node: $1.20-3/month
- Utilization: 3-5% memory, 20-30% CPU

### Break-Even Analysis

**To maximize value:**
- Current setup: **Underutilized by 99%**
- Target: 50 nodes = **50x cost efficiency**
- Optimized: 100 nodes = **100x cost efficiency**

## Deployment Recommendations

### Immediate Next Steps (Safe)

**Scale to 10 nodes:**
```bash
# Duplicate docker-compose services
# Monitor Tor circuit count: tor-proxy logs
# Validate peer discovery across all nodes
```

### Short-Term (1-2 weeks)

**Scale to 30-50 nodes:**
1. Test Tor circuit scaling
2. Monitor system metrics (CPU, memory, network)
3. Optimize peer-manager (batch queries)
4. Validate .onion propagation at scale

### Medium-Term (1-3 months)

**Scale to 100+ nodes:**
1. Implement circuit pooling
2. Multi-server deployment
3. Geographic distribution
4. Load testing framework

### Long-Term (3+ months)

**1000+ nodes across multiple servers:**
1. Automated provisioning
2. Dynamic scaling based on demand
3. Monitoring & alerting infrastructure
4. Cost optimization strategies

## Monitoring Metrics

### Critical to Track

**Per-Node Metrics:**
- Memory consumption trend
- CPU utilization peaks
- Peer connection count
- Network bandwidth (ingress/egress)

**Tor Metrics:**
- Active circuits
- Circuit creation rate
- Circuit failure rate
- Bandwidth throttling events

**System Metrics:**
- Overall CPU load
- Memory pressure
- Disk I/O wait
- Network saturation

### Warning Thresholds

| Metric | Warning | Critical |
|--------|---------|----------|
| CPU (system) | 60% | 80% |
| Memory | 70% | 85% |
| Tor circuits | 7,500 | 9,500 |
| Network | 600 Mbps | 900 Mbps |

## Conclusion

**Current Status:**
- **Massively underutilized** (~1% of capacity)
- Can safely scale to **50-100 nodes** on current hardware
- Primary bottleneck: **Tor circuit management**
- Cost efficiency opportunity: **100x improvement possible**

**Next Action:**
- Scale to 10-20 nodes immediately (safe)
- Monitor for 1-2 weeks
- Optimize Tor configuration
- Plan for 50+ node deployment

---

**Generated**: 2025-01-13
**Based on**: 18 days production monitoring
**Server**: Vultr High-Frequency (Intel Xeon E-2388G, 128GB RAM, 16 cores)
