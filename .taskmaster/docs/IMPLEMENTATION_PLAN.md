# Gethrelay Tor Metrics Monitoring - Implementation Task List

**Based on Research**: Prometheus + Grafana Stack (Recommended Solution)  
**Estimated Timeline**: 4 weeks (50 hours total)  
**Tech Stack**: kube-prometheus-stack, Prometheus, Grafana, Alertmanager

---

## ✅ **PHASE 1: Deploy Monitoring Stack** (Week 1 - 8 hours)

### Task 1.1: Prepare Infrastructure
- [ ] Verify Kubernetes cluster resources (4-6 cores, 10-20GB RAM, 120GB storage available)
- [ ] Confirm storage class supports SSD-backed dynamic provisioning
- [ ] Install Helm 3.x if not present
- [ ] Verify kubectl access with appropriate permissions

### Task 1.2: Deploy kube-prometheus-stack
```bash
# Add Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Create monitoring namespace
kubectl create namespace monitoring

# Deploy stack
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values deployment/monitoring/values-prometheus.yaml \
  --wait
```

### Task 1.3: Configure Grafana Access
- [ ] Port-forward Grafana: `kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80`
- [ ] Login with admin credentials (from Helm values)
- [ ] Verify Prometheus data source is configured
- [ ] Import community Geth dashboards (IDs: 13877, 18463, 15750)

### Task 1.4: Verify Prometheus Scraping
- [ ] Access Prometheus UI: `kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090`
- [ ] Check targets page for node-exporter and kube-state-metrics
- [ ] Verify metrics are being collected

**Deliverables**: 
- Prometheus + Grafana + Alertmanager running in `monitoring` namespace
- Grafana accessible with imported Geth dashboards
- Basic Kubernetes metrics visible

---

## ✅ **PHASE 2: Enable Gethrelay Metrics** (Week 1-2 - 4 hours)

### Task 2.1: Update Gethrelay StatefulSets
**File**: `deployment/k8s/deployments.yaml`

Add metrics flags to all StatefulSets:
```yaml
args:
  - --metrics
  - --metrics.addr=0.0.0.0
  - --metrics.port=6060
  # ... existing args
ports:
  - name: metrics
    containerPort: 6060
    protocol: TCP
```

### Task 2.2: Create Metrics Services
**File**: `deployment/k8s/metrics-services.yaml`

```bash
kubectl apply -f deployment/k8s/metrics-services.yaml
```

Create headless services for each deployment mode (clearnet, prefer-tor, only-onion)

### Task 2.3: Create ServiceMonitor
**File**: `deployment/k8s/servicemonitor.yaml`

```bash
kubectl apply -f deployment/k8s/servicemonitor.yaml
```

### Task 2.4: Verify Metrics Export
```bash
# Test metrics endpoint
kubectl port-forward -n gethrelay gethrelay-clearnet-0 6060:6060
curl http://localhost:6060/debug/metrics/prometheus

# Check Prometheus targets
# Visit http://localhost:9090/targets - should see all 10 gethrelay pods
```

**Deliverables**:
- All gethrelay pods expose `/debug/metrics/prometheus`
- Prometheus scraping all 10 targets successfully
- Standard geth metrics visible (p2p_peers, p2p_ingress, p2p_egress)

---

## ✅ **PHASE 3: Implement Tor Metrics** (Week 2 - 16 hours)

### Task 3.1: Create Tor Metrics Package
**File**: `p2p/tor_metrics.go` (NEW)

```go
package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	torDialAttempts = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "p2p",
		Subsystem: "dials",
		Name:      "tor_total",
		Help:      "Total Tor dial attempts",
	})
	
	torDialSuccesses = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "p2p",
		Subsystem: "dials",
		Name:      "tor_success_total",
		Help:      "Successful Tor dials",
	})
	
	peersByNetworkType = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "p2p",
			Name:      "peers_by_network",
			Help:      "Peers by network type",
		},
		[]string{"network_type"},
	)
	
	trafficByNetworkType = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "p2p",
			Name:      "traffic_bytes_total",
			Help:      "Traffic by type and direction",
		},
		[]string{"network_type", "direction"},
	)
}

func IsOnionAddress(addr string) bool {
	return strings.HasSuffix(addr, ".onion")
}
```

### Task 3.2: Instrument Dial Scheduler
**File**: `p2p/dial.go`

Modify `dialTask.run()` to track Tor dials:
```go
func (t *dialTask) run(d *dialScheduler) {
	// ... existing code ...
	
	addr, _ := t.dest().TCPEndpoint()
	isTor := IsOnionAddress(addr.String())
	
	conn, err := t.dial(d, t.dest())
	if err != nil {
		if isTor {
			torDialAttempts.Inc()
		}
		return
	}
	
	if isTor {
		torDialAttempts.Inc()
		torDialSuccesses.Inc()
	}
	// ... rest of method
}
```

### Task 3.3: Track Traffic by Type
**File**: `p2p/metrics.go`

Enhance `meteredConn` to distinguish Tor traffic:
```go
type meteredConn struct {
	net.Conn
	isTor bool
}

func (c *meteredConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	ingressTrafficMeter.Mark(int64(n))
	
	networkType := "clearnet"
	if c.isTor {
		networkType = "tor"
	}
	trafficByNetworkType.WithLabelValues(networkType, "ingress").Add(float64(n))
	
	return n, err
}
```

### Task 3.4: Track Peer Counts
**File**: `p2p/server.go`

Add periodic peer type counting:
```go
func (srv *Server) updatePeerMetrics() {
	var torPeers, clearnetPeers int
	
	srv.peerOp(func(peers map[enode.ID]*Peer) {
		for _, p := range peers {
			addr := p.RemoteAddr().String()
			if strings.Contains(addr, ".onion") {
				torPeers++
			} else {
				clearnetPeers++
			}
		}
	})
	
	peersByNetworkType.WithLabelValues("tor").Set(float64(torPeers))
	peersByNetworkType.WithLabelValues("clearnet").Set(float64(clearnetPeers))
}
```

### Task 3.5: Build and Deploy
```bash
# Build updated gethrelay
make gethrelay

# Build Docker image
docker build -t ghcr.io/igor53627/gethrelay:tor-metrics-v1 .

# Push to registry
docker push ghcr.io/igor53627/gethrelay:tor-metrics-v1

# Update StatefulSets
kubectl set image statefulset -n gethrelay --all \
  gethrelay=ghcr.io/igor53627/gethrelay:tor-metrics-v1

# Verify new metrics
kubectl port-forward -n gethrelay gethrelay-clearnet-0 6060:6060
curl http://localhost:6060/debug/metrics/prometheus | grep -E "p2p_(peers_by_network|dials_tor|traffic_bytes)"
```

**Deliverables**:
- New metrics visible: `p2p_peers_by_network{network_type="tor|clearnet"}`
- Tor dial metrics: `p2p_dials_tor_total`, `p2p_dials_tor_success_total`
- Traffic metrics: `p2p_traffic_bytes_total{network_type="tor|clearnet",direction="ingress|egress"}`

---

## ✅ **PHASE 4: Create Dashboards** (Week 3 - 8 hours)

### Task 4.1: Create Tor Overview Dashboard
**File**: `deployment/monitoring/dashboard-tor-overview.json`

Panels to create:
1. **Peer Distribution** (Time Series)
   ```promql
   p2p_peers_by_network{namespace="gethrelay"}
   ```

2. **Tor Dial Success Rate** (Gauge)
   ```promql
   rate(p2p_dials_tor_success_total[5m]) / rate(p2p_dials_tor_total[5m]) * 100
   ```

3. **Traffic Breakdown** (Time Series - Stacked)
   ```promql
   rate(p2p_traffic_bytes_total{namespace="gethrelay"}[5m])
   ```

4. **Deployment Mode Distribution** (Pie Chart)
   ```promql
   count by (mode) (up{job="gethrelay"})
   ```

5. **Active Tor Peers by Pod** (Bar Gauge)
   ```promql
   p2p_peers_by_network{namespace="gethrelay",network_type="tor"}
   ```

### Task 4.2: Import Dashboard to Grafana
```bash
kubectl create configmap grafana-dashboard-tor \
  --from-file=tor-overview.json=deployment/monitoring/dashboard-tor-overview.json \
  -n monitoring

kubectl label configmap grafana-dashboard-tor grafana_dashboard=1 -n monitoring
```

### Task 4.3: Create Mode-Specific Dashboards
- Create filtered views for only-onion, prefer-tor, and clearnet modes
- Add variables for pod selection
- Configure auto-refresh (10s interval)

**Deliverables**:
- Custom Tor metrics dashboard in Grafana
- Real-time visualization of peer distribution and traffic
- Deployment mode comparison views

---

## ✅ **PHASE 5: Configure Alerting** (Week 3-4 - 6 hours)

### Task 5.1: Create PrometheusRule
**File**: `deployment/k8s/prometheus-rules.yaml`

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: gethrelay-tor-alerts
  namespace: gethrelay
spec:
  groups:
    - name: tor-alerts
      interval: 30s
      rules:
        - alert: GethrelayLowPeerCount
          expr: p2p_peers{namespace="gethrelay"} < 10
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Low peer count on {{ $labels.pod }}"
            description: "Only {{ $value }} peers connected"
        
        - alert: GethrelayNoTorPeers
          expr: p2p_peers_by_network{namespace="gethrelay",mode=~"prefer-tor|only-onion",network_type="tor"} == 0
          for: 10m
          labels:
            severity: critical
          annotations:
            summary: "No Tor peers on {{ $labels.pod }}"
        
        - alert: GethrelayOnionModeClearnetLeak
          expr: p2p_peers_by_network{mode="only-onion",network_type="clearnet"} > 0
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "SECURITY: Clearnet peers in only-onion mode!"
            description: "Pod {{ $labels.pod }} has {{ $value }} clearnet peers"
```

### Task 5.2: Configure Alertmanager
Update Helm values with notification channels:
```yaml
alertmanager:
  config:
    receivers:
      - name: 'slack'
        slack_configs:
          - api_url: 'SLACK_WEBHOOK_URL'
            channel: '#gethrelay-alerts'
      
      - name: 'email'
        email_configs:
          - to: 'ops@example.com'
            from: 'alertmanager@example.com'
```

### Task 5.3: Test Alerts
```bash
# Trigger test alert
kubectl scale statefulset -n gethrelay gethrelay-only-onion-1 --replicas=0

# Verify alert fires in Prometheus
# Check Alertmanager receives alert
# Confirm notification sent to Slack/email

# Restore
kubectl scale statefulset -n gethrelay gethrelay-only-onion-1 --replicas=1
```

**Deliverables**:
- PrometheusRule deployed with Tor-specific alerts
- Alertmanager routing configured
- Test alerts verified working

---

## ✅ **PHASE 6: Production Rollout** (Week 4 - 8 hours)

### Task 6.1: Security Hardening
- [ ] Create NetworkPolicy to restrict metrics endpoint access
- [ ] Enable HTTPS for Grafana (TLS certificate)
- [ ] Change Grafana admin password
- [ ] Encrypt Alertmanager webhook secrets

### Task 6.2: Performance Tuning
- [ ] Review Prometheus retention (15 days default)
- [ ] Optimize scrape intervals if needed
- [ ] Check storage usage and adjust PV sizes

### Task 6.3: Documentation
- [ ] Create metrics reference document
- [ ] Write alert runbooks
- [ ] Document dashboard usage
- [ ] Create operational procedures

### Task 6.4: Monitoring the Monitoring
- [ ] Create alerts for Prometheus down
- [ ] Monitor Prometheus memory usage
- [ ] Set up backup for Grafana dashboards
- [ ] Configure Prometheus federation (optional)

### Task 6.5: Final Validation
```bash
# Verify all pods scraped
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
# Check /targets - should show 10/10 gethrelay pods UP

# Verify metrics accuracy
# Compare dashboard peer counts with RPC queries:
kubectl exec -n gethrelay gethrelay-only-onion-1-0 -c gethrelay -- \
  sh -c 'wget -qO- --post-data='"'"'{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'"'"' \
  --header='"'"'Content-Type: application/json'"'"' http://localhost:8545'

# Verify Tor metrics show non-zero values for only-onion pods
# Verify clearnet leak alert doesn't fire for only-onion pods
```

**Deliverables**:
- Production-ready monitoring stack
- Security hardening complete
- Documentation published
- All validation checks passed

---

## 📋 **Quick Reference**

### Key Metrics
```promql
# Total peers
p2p_peers{namespace="gethrelay"}

# Tor peers only
p2p_peers_by_network{namespace="gethrelay",network_type="tor"}

# Tor dial success rate
rate(p2p_dials_tor_success_total[5m]) / rate(p2p_dials_tor_total[5m])

# Tor bandwidth (bytes/sec)
rate(p2p_traffic_bytes_total{namespace="gethrelay",network_type="tor"}[5m])

# Peer distribution by mode
sum by (mode) (p2p_peers{namespace="gethrelay"})
```

### Useful Commands
```bash
# Check all monitoring pods
kubectl get pods -n monitoring

# Grafana access
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80

# Prometheus access
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090

# Check gethrelay metrics
kubectl port-forward -n gethrelay gethrelay-clearnet-0 6060:6060
curl http://localhost:6060/debug/metrics/prometheus

# View alerts
kubectl get prometheusrule -n gethrelay

# Check Alertmanager
kubectl port-forward -n monitoring svc/kube-prometheus-stack-alertmanager 9093:9093
```

### File Locations
- Helm values: `deployment/monitoring/values-prometheus.yaml`
- Metrics services: `deployment/k8s/metrics-services.yaml`
- ServiceMonitor: `deployment/k8s/servicemonitor.yaml`
- PrometheusRule: `deployment/k8s/prometheus-rules.yaml`
- Dashboard JSON: `deployment/monitoring/dashboard-tor-overview.json`
- Implementation docs: `.taskmaster/docs/monitoring-prd.md`
- Research report: (see research agent output above)

---

## 🎯 Success Criteria Checklist

- [ ] All 10 gethrelay pods expose Prometheus metrics
- [ ] Prometheus successfully scrapes all targets (10/10 UP)
- [ ] Grafana dashboards display live Tor metrics
- [ ] Custom Tor metrics accurately track .onion vs clearnet
- [ ] Alerts fire correctly for test scenarios
- [ ] Zero clearnet peers in only-onion pods (verified)
- [ ] Dashboard peer counts match RPC queries
- [ ] Alertmanager sends notifications to Slack/email
- [ ] Documentation complete and accessible
- [ ] Team trained on dashboard usage

---

**Total Estimated Time**: 50 hours over 4 weeks  
**Priority**: High  
**Dependencies**: Kubernetes cluster, Helm, kubectl access, Slack webhook (optional)

