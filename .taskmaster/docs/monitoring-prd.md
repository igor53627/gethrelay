# Gethrelay Tor Metrics Monitoring System - Implementation PRD

## Overview
Implement comprehensive monitoring and observability for gethrelay's Tor-integrated Ethereum P2P relay infrastructure using Prometheus + Grafana stack.

## Objectives
1. Deploy production-ready monitoring infrastructure (Prometheus + Grafana)
2. Instrument gethrelay code with Tor-specific metrics
3. Create custom dashboards for Tor metrics visualization
4. Configure alerting for critical monitoring scenarios
5. Enable operational visibility across 3 deployment modes (clearnet, prefer-tor, only-onion)

## Technical Requirements

### Phase 1: Monitoring Stack Deployment (Week 1)
- Deploy kube-prometheus-stack via Helm to `monitoring` namespace
- Configure Prometheus with 15-day retention, 50GB storage per replica
- Configure Grafana with TLS ingress and authentication
- Set up Alertmanager with notification channels (Slack, email)
- Import community Geth dashboards (IDs: 13877, 18463)

### Phase 2: Gethrelay Metrics Instrumentation (Week 2)
- Enable built-in Prometheus endpoint in all gethrelay pods (`:6060/debug/metrics/prometheus`)
- Create Kubernetes Services for metrics endpoints
- Create ServiceMonitor CRD for auto-discovery
- Implement custom Tor metrics in gethrelay code:
  - `p2p_peers_tor` / `p2p_peers_clearnet` - Peer type gauges
  - `p2p_dials_tor_total` / `p2p_dials_tor_success_total` - Tor dial counters
  - `p2p_ingress_tor` / `p2p_egress_tor` - Tor traffic meters
  - `p2p_dials_tor_error_circuit` - Tor-specific error tracking

### Phase 3: Custom Dashboards (Week 3)
- Create "Gethrelay Tor Metrics Overview" dashboard with panels:
  - Peer connections by type (Tor vs clearnet)
  - Tor dial success rate
  - Traffic breakdown by network type
  - Deployment mode distribution
  - Tor circuit failures
- Configure dashboard auto-refresh (10s interval)
- Create dashboard templates for each deployment mode

### Phase 4: Alerting Configuration (Week 4)
- Create PrometheusRule for Tor-specific alerts:
  - Low peer count (<10 peers for 5 min)
  - No Tor peers in prefer-tor/only-onion mode (10 min)
  - High Tor dial failure rate (>50% for 5 min)
  - Clearnet leak in only-onion mode (CRITICAL)
  - Pod down (2 min)
- Configure Alertmanager routing to Slack/email
- Create runbook documentation for each alert

## Success Criteria
- [ ] All 10 gethrelay pods expose Prometheus metrics
- [ ] Prometheus successfully scrapes all targets
- [ ] Grafana dashboards display live Tor metrics
- [ ] Custom Tor metrics accurately track .onion vs clearnet traffic
- [ ] Alerts fire correctly for test scenarios
- [ ] Zero clearnet peers detected in only-onion pods
- [ ] Dashboard shows peer counts matching RPC queries

## Resources Required
- Kubernetes cluster resources: 4-6 cores, 10-20GB RAM, 120GB storage
- Helm 3.x installation
- kubectl access with cluster-admin permissions
- Slack webhook URL for alerts
- TLS certificate for Grafana ingress

## Timeline
- Week 1: Monitoring stack deployment
- Week 2: Metrics instrumentation and code changes
- Week 3: Dashboard creation
- Week 4: Alerting configuration and production rollout

## Technical Stack
- **Monitoring**: Prometheus + Grafana (via kube-prometheus-stack)
- **Instrumentation**: Prometheus Go client library
- **Storage**: Kubernetes PersistentVolumes (SSD-backed)
- **Query Language**: PromQL
- **Visualization**: Grafana 9.x+
