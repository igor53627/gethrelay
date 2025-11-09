# Gethrelay Kubernetes Deployment Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          GitHub Repository                                   │
│                                                                              │
│  ┌────────────────────┐          ┌──────────────────────┐                  │
│  │   Source Code      │          │  Container Registry  │                  │
│  │   - gethrelay      │─────────▶│  ghcr.io/ethereum/  │                  │
│  │   - Dockerfile     │  Build   │    gethrelay:latest  │                  │
│  └────────────────────┘          └──────────────────────┘                  │
│                                             │                                │
│  ┌────────────────────────────────────────┐ │                                │
│  │    GitHub Actions Workflow             │ │                                │
│  │  .github/workflows/deploy-gethrelay.yaml│                                │
│  │                                        │ │                                │
│  │  1. Build & Push ──────────────────────┘                                │
│  │  2. Deploy to K8s                      │                                │
│  │  3. Health Check                       │                                │
│  └────────────────────────────────────────┘                                │
│                    │                                                         │
└────────────────────┼─────────────────────────────────────────────────────────┘
                     │ (uses KUBECONFIG secret)
                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Vultr Kubernetes Cluster                                │
│                vke-3c38b142-565b-4497-9762-c37fe9da1879                     │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                     Namespace: gethrelay                              │ │
│  │                                                                       │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │ │
│  │  │                    ConfigMap: gethrelay-config                  │ │ │
│  │  │   CHAIN=mainnet, MAX_PEERS=200, V5DISC=true                     │ │ │
│  │  └─────────────────────────────────────────────────────────────────┘ │ │
│  │                                                                       │ │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │ │
│  │  │  DEFAULT MODE    │  │  PREFER-TOR MODE │  │  TOR-ONLY MODE   │  │ │
│  │  │  (3 instances)   │  │  (4 instances)   │  │  (3 instances)   │  │ │
│  │  └──────────────────┘  └──────────────────┘  └──────────────────┘  │ │
│  │                                                                       │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │ │
│  │  │ Pod: gethrelay-default-1                                        │ │ │
│  │  │                                                                 │ │ │
│  │  │  ┌────────────────────────────┐  ┌──────────────────────────┐ │ │ │
│  │  │  │  Container: tor            │  │  Container: gethrelay    │ │ │ │
│  │  │  │  Image: alpine/tor:latest  │  │  Image: ghcr.io/...      │ │ │ │
│  │  │  │                            │  │                          │ │ │ │
│  │  │  │  SOCKS5 Proxy: 9050        │◀─┤  Args:                   │ │ │ │
│  │  │  │  Control Port: 9051        │  │  --chain=mainnet         │ │ │ │
│  │  │  │                            │  │  --tor-proxy=:9050       │ │ │ │
│  │  │  │  Resources:                │  │  --v5disc                │ │ │ │
│  │  │  │    CPU: 100m-500m          │  │                          │ │ │ │
│  │  │  │    Mem: 128Mi-512Mi        │  │  Resources:              │ │ │ │
│  │  │  │                            │  │    CPU: 200m-1000m       │ │ │ │
│  │  │  │  User: tor                 │  │    Mem: 256Mi-1Gi        │ │ │ │
│  │  │  └────────────────────────────┘  │                          │ │ │ │
│  │  │                                   │  User: gethrelay         │ │ │ │
│  │  │  ┌────────────────────────────┐  │  Port: 30303 (P2P)       │ │ │ │
│  │  │  │  Volume: tor-data          │  └──────────────────────────┘ │ │ │
│  │  │  │  Type: emptyDir            │                               │ │ │
│  │  │  └────────────────────────────┘                               │ │ │
│  │  └─────────────────────────────────────────────────────────────────┘ │ │
│  │          │                                                            │ │
│  │          ▼                                                            │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │ │
│  │  │ Service: gethrelay-default-1                                    │ │ │
│  │  │   Type: NodePort                                                │ │ │
│  │  │   Port: 30303 → TargetPort: 30303                               │ │ │
│  │  └─────────────────────────────────────────────────────────────────┘ │ │
│  │                                                                       │ │
│  │  (Similar structure for gethrelay-default-2, default-3,               │ │
│  │   prefer-tor-1/2/3/4, only-onion-1/2/3)                              │ │
│  │                                                                       │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐ │ │
│  │  │ Service: gethrelay (Headless)                                   │ │ │
│  │  │   ClusterIP: None                                               │ │ │
│  │  │   Purpose: Service discovery for all pods                       │ │ │
│  │  └─────────────────────────────────────────────────────────────────┘ │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                   │
                                   │ P2P Connections
                                   ▼
                        ┌──────────────────────┐
                        │  Ethereum Network    │
                        │  - Mainnet           │
                        │  - Tor .onion peers  │
                        │  - Clearnet peers    │
                        └──────────────────────┘
```

## Deployment Flow

### 1. Build & Deploy Flow

```
Developer          GitHub Actions           Container Registry       Kubernetes
    │                     │                         │                    │
    │  Push/Release       │                         │                    │
    ├────────────────────▶│                         │                    │
    │                     │                         │                    │
    │                     │  Build Image            │                    │
    │                     ├────────────────────────▶│                    │
    │                     │                         │                    │
    │                     │  Push Image             │                    │
    │                     ├────────────────────────▶│                    │
    │                     │                         │                    │
    │                     │  Apply Manifests        │                    │
    │                     ├──────────────────────────────────────────────▶│
    │                     │                         │                    │
    │                     │  Pull Image             │                    │
    │                     │                         │◀───────────────────┤
    │                     │                         │                    │
    │                     │  Health Check           │                    │
    │                     ├──────────────────────────────────────────────▶│
    │                     │                         │                    │
    │                     │  Deployment Status      │                    │
    │                     │◀────────────────────────────────────────────┤
    │                     │                         │                    │
    │  Notification       │                         │                    │
    │◀────────────────────┤                         │                    │
    │                     │                         │                    │
```

### 2. Pod Startup Sequence

```
1. Pod Created
   │
   ├─▶ Init Container: None
   │
   ├─▶ Container: tor (starts in parallel)
   │   ├─ Read torrc configuration
   │   ├─ Initialize Tor daemon
   │   ├─ Establish circuits
   │   └─ Listen on SOCKS5 9050
   │
   └─▶ Container: gethrelay (starts in parallel)
       ├─ Wait for Tor proxy (optional)
       ├─ Initialize P2P stack
       ├─ Connect to bootnodes
       ├─ Discovery (v5)
       ├─ Connect to peers via Tor/clearnet
       └─ Ready to relay blocks
```

### 3. Network Traffic Flow

```
External P2P Peer
       │
       │ Clearnet/Tor
       ▼
NodePort Service (30303)
       │
       ▼
gethrelay Container
       │
       ├─▶ .onion peer?
       │   │
       │   ├─ Yes ─▶ Tor Container (SOCKS5 9050) ─▶ Tor Network ─▶ .onion Peer
       │   │
       │   └─ No  ─▶ Direct connection (if allowed by mode)
       │
       └─▶ Clearnet peer?
           │
           ├─ Default/Prefer-Tor mode ─▶ Direct connection
           │
           └─ Tor-Only mode ─▶ Reject
```

## Tor Mode Comparison

### Default Mode (3 instances)

```
Peer has .onion?     Has clearnet?      Action
─────────────────────────────────────────────────
    Yes                  Yes            Try Tor → Fallback to clearnet
    Yes                  No             Try Tor → Fail if Tor fails
    No                   Yes            Use clearnet
    No                   No             Reject
```

### Prefer-Tor Mode (4 instances)

```
Peer has .onion?     Has clearnet?      Action
─────────────────────────────────────────────────
    Yes                  Yes            Use Tor → Fallback to clearnet
    Yes                  No             Use Tor → Fail if Tor fails
    No                   Yes            Use clearnet
    No                   No             Reject
```

### Tor-Only Mode (3 instances)

```
Peer has .onion?     Has clearnet?      Action
─────────────────────────────────────────────────
    Yes                  Yes/No         Use Tor only → Fail if Tor fails
    No                   Yes            Reject (no .onion)
    No                   No             Reject
```

## Resource Distribution

### Per Pod Resources

```
┌──────────────────────────────────────────────────────┐
│                     Pod                              │
│                                                      │
│  ┌────────────────┐         ┌───────────────────┐   │
│  │ Tor Container  │         │ Gethrelay Container│   │
│  │                │         │                    │   │
│  │ Request:       │         │ Request:           │   │
│  │   100m CPU     │         │   200m CPU         │   │
│  │   128Mi RAM    │         │   256Mi RAM        │   │
│  │                │         │                    │   │
│  │ Limit:         │         │ Limit:             │   │
│  │   500m CPU     │         │   1000m CPU        │   │
│  │   512Mi RAM    │         │   1Gi RAM          │   │
│  └────────────────┘         └───────────────────┘   │
│                                                      │
│  Total Request: 300m CPU, 384Mi RAM                 │
│  Total Limit:   1500m CPU, 1.5Gi RAM                │
└──────────────────────────────────────────────────────┘
```

### Total Cluster Resources (10 pods)

```
Resource      Request    Limit      Typical Usage
─────────────────────────────────────────────────
CPU           3000m      15000m     4000-8000m
Memory        3.8Gi      15Gi       4-8Gi
Storage       0          0          ~100Mi per pod (ephemeral)
```

## Security Architecture

### Authentication & Authorization

```
┌────────────────────────────────────────────────────┐
│              GitHub Actions                        │
│  ┌──────────────────────────────────────────┐     │
│  │  GITHUB_TOKEN (automatic)                │     │
│  │  - Push to ghcr.io                       │     │
│  │  - Read repository                       │     │
│  └──────────────────────────────────────────┘     │
│  ┌──────────────────────────────────────────┐     │
│  │  KUBECONFIG (secret)                     │     │
│  │  - Base64 encoded                        │     │
│  │  - Cluster admin access                  │     │
│  └──────────────────────────────────────────┘     │
└────────────────────────────────────────────────────┘
                    │
                    ▼
┌────────────────────────────────────────────────────┐
│           Kubernetes Cluster                       │
│  ┌──────────────────────────────────────────┐     │
│  │  RBAC (via kubeconfig)                   │     │
│  │  - Namespace: gethrelay                  │     │
│  │  - Resources: pods, deployments, services│     │
│  └──────────────────────────────────────────┘     │
│  ┌──────────────────────────────────────────┐     │
│  │  Pod Security                            │     │
│  │  - Non-root users (tor, gethrelay)       │     │
│  │  - No privileged containers              │     │
│  │  - Resource limits enforced              │     │
│  │  - No host network/PID/IPC               │     │
│  └──────────────────────────────────────────┘     │
└────────────────────────────────────────────────────┘
```

### Data Flow Security

```
GitHub Secrets ──(TLS)──▶ GitHub Actions ──(TLS)──▶ K8s API
                              │
                              ├──(TLS)──▶ Container Registry
                              │
                              └──(TLS)──▶ Cluster Nodes
                                            │
                                            ▼
                                   ┌─────────────────┐
                                   │  Pod Network    │
                                   │  - Tor: SOCKS5  │
                                   │  - P2P: 30303   │
                                   └─────────────────┘
```

## Monitoring & Observability

### Recommended Monitoring Points

```
┌─────────────────────────────────────────────────────────┐
│                   Monitoring Stack                      │
│                                                         │
│  ┌────────────────────────────────────────────┐        │
│  │         Cluster Metrics (kubectl)          │        │
│  │  - Pod status (Running/Failed)             │        │
│  │  - Resource usage (CPU/Memory)             │        │
│  │  - Events (Errors/Warnings)                │        │
│  └────────────────────────────────────────────┘        │
│                                                         │
│  ┌────────────────────────────────────────────┐        │
│  │       Application Logs (kubectl logs)      │        │
│  │  - Gethrelay: P2P connections, errors      │        │
│  │  - Tor: Circuit status, SOCKS errors       │        │
│  └────────────────────────────────────────────┘        │
│                                                         │
│  ┌────────────────────────────────────────────┐        │
│  │     Optional: Prometheus/Grafana           │        │
│  │  - gethrelay metrics (port 6060)           │        │
│  │  - Custom dashboards                       │        │
│  │  - Alerting rules                          │        │
│  └────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────┘
```

## Failure Scenarios & Recovery

### Pod Failure

```
Pod Crashes
    │
    ├─▶ Kubernetes detects (liveness probe or exit code)
    │
    ├─▶ Restart policy: Always (default)
    │
    ├─▶ Exponential backoff if repeated failures
    │
    └─▶ CrashLoopBackOff after multiple failures
            │
            └─▶ Manual intervention required
                - Check logs: kubectl logs <pod>
                - Check events: kubectl describe pod <pod>
```

### Deployment Update

```
New Image Available
    │
    ├─▶ Update deployment image tag
    │
    ├─▶ Kubernetes initiates rolling update
    │   ├─ Create new pod with new image
    │   ├─ Wait for pod to be Ready
    │   ├─ Terminate old pod
    │   └─ Repeat for all replicas
    │
    └─▶ Rollback available if issues
        - kubectl rollout undo deployment/<name>
```

### Tor Circuit Failure

```
Tor Circuit Fails
    │
    ├─▶ Default/Prefer-Tor mode
    │   └─▶ Fallback to clearnet (if available)
    │
    └─▶ Tor-Only mode
        └─▶ Connection fails
            └─▶ Peer discovery continues
                └─▶ Find other .onion peers
```

## Scalability Considerations

### Horizontal Scaling (Not Recommended for P2P)

```
❌ DON'T: Scale replicas > 1 per deployment
   - P2P nodes have unique identities
   - Multiple replicas = duplicate node IDs
   - Wastes resources

✓ DO: Add more deployments with different configs
   - Each deployment = 1 replica
   - Unique node identity per pod
   - Better configuration variety
```

### Cluster Scaling

```
Add Nodes to Cluster
    │
    ├─▶ Kubernetes scheduler distributes pods
    │
    ├─▶ Better fault tolerance
    │
    └─▶ More capacity for additional deployments
```

## File References

- **Kubernetes Manifests**: `deployment/k8s/`
- **Tor Configuration**: `deployment/tor/torrc`
- **Container Build**: `Dockerfile.gethrelay`
- **CI/CD Pipeline**: `.github/workflows/deploy-gethrelay.yaml`
- **Documentation**: `deployment/README.md`

---

**Generated:** 2025-11-09
**Version:** 1.0.0
**Project:** go-ethereum/gethrelay
