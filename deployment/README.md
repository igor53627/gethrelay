# Gethrelay Deployment Guide

This guide covers building the Docker image, publishing to GitHub Container Registry, and deploying 10 gethrelay instances with varied Tor settings to a Kubernetes cluster with automated CI/CD using GitHub Actions.

For detailed Docker build documentation, see [DOCKER_BUILD.md](DOCKER_BUILD.md).

## Overview

The deployment consists of:
- **10 gethrelay instances** distributed across three Tor configuration modes:
  - **3 instances** - Default mode (Tor with clearnet fallback)
  - **4 instances** - Prefer Tor mode (prefers .onion addresses)
  - **3 instances** - Tor-Only mode (only connects to .onion peers)

Each instance runs in its own pod with:
- Sidecar Tor proxy container
- Dedicated P2P networking
- Resource limits and requests
- Prometheus metrics exposure

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                      │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │         Namespace: gethrelay                       │ │
│  │                                                    │ │
│  │  ┌──────────────┐  ┌──────────────┐              │ │
│  │  │   Pod 1-3    │  │   Pod 4-7    │              │ │
│  │  │  (Default)   │  │ (Prefer-Tor) │              │ │
│  │  │              │  │              │              │ │
│  │  │ ┌─────────┐  │  │ ┌─────────┐  │              │ │
│  │  │ │Gethrelay│  │  │ │Gethrelay│  │              │ │
│  │  │ │         │  │  │ │         │  │              │ │
│  │  │ └─────────┘  │  │ └─────────┘  │              │ │
│  │  │ ┌─────────┐  │  │ ┌─────────┐  │              │ │
│  │  │ │   Tor   │  │  │ │   Tor   │  │              │ │
│  │  │ └─────────┘  │  │ └─────────┘  │              │ │
│  │  └──────────────┘  └──────────────┘              │ │
│  │                                                    │ │
│  │  ┌──────────────┐                                 │ │
│  │  │   Pod 8-10   │                                 │ │
│  │  │ (Tor-Only)   │                                 │ │
│  │  │              │                                 │ │
│  │  │ ┌─────────┐  │                                 │ │
│  │  │ │Gethrelay│  │                                 │ │
│  │  │ │         │  │                                 │ │
│  │  │ └─────────┘  │                                 │ │
│  │  │ ┌─────────┐  │                                 │ │
│  │  │ │   Tor   │  │                                 │ │
│  │  │ └─────────┘  │                                 │ │
│  │  └──────────────┘                                 │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Build and Publish Docker Image

The image needs to be built and published to GitHub Container Registry before deployment.

#### Option A: Automated Build (Recommended)

Trigger the GitHub Actions workflow:

```bash
# Via GitHub CLI
gh workflow run build-gethrelay-image.yaml

# Or via GitHub UI
# Navigate to: Actions > Build and Push Gethrelay Image > Run workflow
```

#### Option B: Manual Build

```bash
# Using the provided script
export GITHUB_USERNAME="igor53627"
export GITHUB_TOKEN="your-github-pat"
./scripts/build-and-push-image.sh --login -m -p

# Or using Docker directly
echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GITHUB_USERNAME" --password-stdin
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile.gethrelay \
  -t ghcr.io/igor53627/gethrelay:latest \
  --push \
  .
```

#### Make Image Public (One-time Setup)

To avoid needing imagePullSecrets in Kubernetes:

1. Visit: https://github.com/users/igor53627/packages/container/gethrelay/settings
2. Scroll to "Danger Zone" > "Change visibility"
3. Select "Public" and confirm

See [DOCKER_BUILD.md](DOCKER_BUILD.md) for detailed documentation.

## Prerequisites

1. **Kubernetes Cluster Access**
   - Kubeconfig file with appropriate permissions
   - kubectl CLI tool installed

2. **GitHub Repository Secrets** (for CI/CD)
   - `KUBECONFIG` - Base64 encoded kubeconfig file

3. **Container Registry Access** (for manual builds)
   - GitHub Personal Access Token with `write:packages` permission
   - Docker with BuildKit and buildx support

## Initial Setup

### 1. Configure Kubeconfig Secret

The kubeconfig file should NEVER be committed to the repository. Instead, store it as a GitHub secret:

```bash
# Base64 encode your kubeconfig
cat kubeconfig.yaml | base64 | pbcopy  # macOS
cat kubeconfig.yaml | base64 -w 0      # Linux

# Add to GitHub:
# Settings -> Secrets and variables -> Actions -> New repository secret
# Name: KUBECONFIG
# Value: <paste base64 encoded kubeconfig>
```

### 2. Verify Local Kubeconfig

```bash
# Test cluster access
export KUBECONFIG=./kubeconfig.yaml
kubectl cluster-info
kubectl get nodes
```

### 2. Deploy to Kubernetes

For the initial deployment or testing:

```bash
# Create namespace
kubectl apply -f deployment/k8s/namespace.yaml

# Deploy all instances
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml

# Verify deployment
kubectl get deployments -n gethrelay
kubectl get pods -n gethrelay
kubectl get services -n gethrelay
```

**Note:** If the image is private, you'll need to create an imagePullSecret. See [DOCKER_BUILD.md](DOCKER_BUILD.md#kubernetes-deployment) for instructions.

## Tor Configuration Modes

### Default Mode (3 instances)
```yaml
args:
  - --tor-proxy=127.0.0.1:9050
```
- Connects to .onion addresses via Tor when available
- Falls back to clearnet on Tor failures
- Uses clearnet for peers without .onion addresses

### Prefer Tor Mode (4 instances)
```yaml
args:
  - --tor-proxy=127.0.0.1:9050
  - --prefer-tor
```
- Prefers .onion addresses when both available
- Falls back to clearnet on Tor failures
- Maximizes privacy while maintaining connectivity

### Tor-Only Mode (3 instances)
```yaml
args:
  - --tor-proxy=127.0.0.1:9050
  - --only-onion
```
- Only connects to peers with .onion addresses
- Rejects clearnet-only peers
- Maximum privacy, may have fewer peers

## CI/CD Workflow

### Automatic Deployment on Release

The GitHub Actions workflow automatically deploys when:
1. A new release is published
2. Manual workflow dispatch is triggered

### Workflow Steps

1. **Build and Push**
   - Builds gethrelay Docker image
   - Pushes to GitHub Container Registry
   - Tags with version and commit SHA

2. **Deploy to Kubernetes**
   - Applies namespace configuration
   - Updates deployments with new image
   - Creates/updates services
   - Waits for rollout completion

3. **Health Check**
   - Verifies all pods are running
   - Checks logs for errors
   - Reports deployment statistics

4. **Notification**
   - Reports success or failure
   - Uploads deployment manifests as artifacts

### Manual Deployment via GitHub Actions

1. Go to **Actions** tab in GitHub
2. Select **Deploy Gethrelay to Kubernetes** workflow
3. Click **Run workflow**
4. Select environment (production/staging)
5. Click **Run workflow** button

## Monitoring and Maintenance

### Check Deployment Status

```bash
# Get all deployments
kubectl get deployments -n gethrelay

# Get deployment by mode
kubectl get deployments -n gethrelay -l mode=default
kubectl get deployments -n gethrelay -l mode=prefer-tor
kubectl get deployments -n gethrelay -l mode=only-onion

# Get pods
kubectl get pods -n gethrelay -o wide

# Get services
kubectl get services -n gethrelay
```

### View Logs

```bash
# View logs for specific pod
kubectl logs -n gethrelay <pod-name> -c gethrelay

# View Tor logs
kubectl logs -n gethrelay <pod-name> -c tor

# Follow logs in real-time
kubectl logs -n gethrelay <pod-name> -c gethrelay -f

# View logs for all pods with specific label
kubectl logs -n gethrelay -l mode=prefer-tor -c gethrelay
```

### Resource Usage

```bash
# Check resource usage
kubectl top pods -n gethrelay

# Describe specific deployment
kubectl describe deployment gethrelay-prefer-tor-1 -n gethrelay

# Get events
kubectl get events -n gethrelay --sort-by='.lastTimestamp'
```

### Scaling

```bash
# Scale a specific deployment (not recommended for P2P nodes)
kubectl scale deployment gethrelay-default-1 -n gethrelay --replicas=2

# View deployment replicas
kubectl get deployments -n gethrelay -o wide
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl describe pod <pod-name> -n gethrelay

# Check events
kubectl get events -n gethrelay

# Check if image pull succeeded
kubectl get pods -n gethrelay -o jsonpath='{.items[*].status.containerStatuses[*].state}'
```

### Network Issues

```bash
# Check service endpoints
kubectl get endpoints -n gethrelay

# Test Tor connectivity from pod
kubectl exec -it <pod-name> -n gethrelay -c gethrelay -- sh
# Inside pod:
# netstat -tlnp | grep 9050  # Check Tor SOCKS proxy

# Check if Tor is running
kubectl exec -it <pod-name> -n gethrelay -c tor -- ps aux
```

### Configuration Issues

```bash
# View ConfigMap
kubectl get configmap gethrelay-config -n gethrelay -o yaml

# Edit ConfigMap
kubectl edit configmap gethrelay-config -n gethrelay

# Restart deployments after ConfigMap change
kubectl rollout restart deployment -n gethrelay
```

### Tor Connection Issues

```bash
# Check Tor logs
kubectl logs <pod-name> -n gethrelay -c tor

# Check Tor circuit status
kubectl exec -it <pod-name> -n gethrelay -c tor -- \
  cat /var/lib/tor/notices.log
```

## Security Considerations

1. **Kubeconfig Protection**
   - Never commit kubeconfig to repository
   - Store in GitHub Secrets with base64 encoding
   - Rotate kubeconfig credentials regularly

2. **Container Security**
   - Runs as non-root user (gethrelay:1000)
   - Separate Tor user for Tor daemon
   - Resource limits enforced

3. **Network Security**
   - NodePort services for external P2P access
   - Tor proxy isolates clearnet traffic
   - No privileged containers

4. **Secret Management**
   - Use Kubernetes Secrets for sensitive data
   - Tor cookies stored in ephemeral volumes
   - No persistent storage of private keys

## Updating Deployments

### Update Image Version

```bash
# Update to specific version
kubectl set image deployment/gethrelay-default-1 \
  gethrelay=ghcr.io/igor53627/gethrelay:v1.2.3 \
  -n gethrelay

# Update all deployments
for deploy in $(kubectl get deployments -n gethrelay -o name); do
  kubectl set image $deploy \
    gethrelay=ghcr.io/igor53627/gethrelay:v1.2.3 \
    -n gethrelay
done
```

### Update Configuration

```bash
# Edit ConfigMap
kubectl edit configmap gethrelay-config -n gethrelay

# Trigger rolling restart
kubectl rollout restart deployment -n gethrelay
```

### Rolling Update

```bash
# Check rollout status
kubectl rollout status deployment/gethrelay-default-1 -n gethrelay

# View rollout history
kubectl rollout history deployment/gethrelay-default-1 -n gethrelay

# Rollback if needed
kubectl rollout undo deployment/gethrelay-default-1 -n gethrelay
```

## Cleanup

### Remove Specific Deployment

```bash
kubectl delete deployment gethrelay-default-1 -n gethrelay
kubectl delete service gethrelay-default-1 -n gethrelay
```

### Remove All Deployments

```bash
kubectl delete namespace gethrelay
```

### Preserve Namespace, Remove Deployments

```bash
kubectl delete -f deployment/k8s/deployments.yaml
kubectl delete -f deployment/k8s/services.yaml
```

## Performance Tuning

### Adjust Resource Limits

Edit `deployment/k8s/deployments.yaml`:

```yaml
resources:
  requests:
    cpu: 200m      # Increase for better performance
    memory: 256Mi  # Increase for more peers
  limits:
    cpu: 1000m
    memory: 1Gi
```

### Tor Performance

Edit `deployment/tor/torrc`:

```
# Increase circuit limits
MaxCircuitDirtiness 30
CircuitBuildTimeout 20
NumEntryGuards 8
```

### P2P Performance

Update ConfigMap in `deployment/k8s/deployments.yaml`:

```yaml
data:
  MAX_PEERS: "500"  # Increase max peers
```

## Metrics and Monitoring

### Prometheus Integration

Add Prometheus annotations to deployments:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "6060"
    prometheus.io/path: "/debug/metrics/prometheus"
```

### Grafana Dashboards

Useful metrics to monitor:
- `p2p_peers` - Number of connected peers
- `p2p_ingress` - Incoming network traffic
- `p2p_egress` - Outgoing network traffic
- Tor circuit count and status

## Support

For issues or questions:
1. Check pod logs: `kubectl logs -n gethrelay <pod-name>`
2. Review events: `kubectl get events -n gethrelay`
3. Check deployment status: `kubectl describe deployment <name> -n gethrelay`
4. Review GitHub Actions workflow logs

## References

- [Gethrelay Documentation](../cmd/gethrelay/)
- [Tor Configuration](https://2019.www.torproject.org/docs/tor-manual.html)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
