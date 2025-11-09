# Gethrelay Kubernetes Deployment - Quick Start

## TL;DR

Deploy 10 gethrelay instances with varied Tor settings to Kubernetes:

```bash
# 1. Setup (one-time)
./deployment/scripts/setup-github-secrets.sh kubeconfig.yaml

# 2. Test deployment configuration
./deployment/scripts/test-deployment.sh kubeconfig.yaml

# 3. Deploy via GitHub Actions
# Go to Actions > Deploy Gethrelay to Kubernetes > Run workflow
```

## Instance Distribution

- **3 instances** - Default mode (Tor + clearnet fallback)
- **4 instances** - Prefer-Tor mode (prefers .onion addresses)
- **3 instances** - Tor-Only mode (only .onion connections)

## Manual Deployment

```bash
export KUBECONFIG=./kubeconfig.yaml

# Deploy everything
kubectl apply -f deployment/k8s/namespace.yaml
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml

# Watch deployment
kubectl get pods -n gethrelay -w
```

## Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n gethrelay

# Check deployments
kubectl get deployments -n gethrelay

# Check services
kubectl get services -n gethrelay

# View logs
kubectl logs -n gethrelay <pod-name> -c gethrelay -f
```

## Common Commands

```bash
# Get pods by mode
kubectl get pods -n gethrelay -l mode=default
kubectl get pods -n gethrelay -l mode=prefer-tor
kubectl get pods -n gethrelay -l mode=only-onion

# View logs for all instances of a mode
kubectl logs -n gethrelay -l mode=prefer-tor -c gethrelay

# Check resource usage
kubectl top pods -n gethrelay

# Delete everything
kubectl delete namespace gethrelay
```

## Troubleshooting

```bash
# Pod not starting?
kubectl describe pod <pod-name> -n gethrelay

# Check events
kubectl get events -n gethrelay --sort-by='.lastTimestamp'

# Test Tor connectivity
kubectl exec -it <pod-name> -n gethrelay -c tor -- ps aux

# View all container logs
kubectl logs <pod-name> -n gethrelay --all-containers
```

## GitHub Actions Deployment

1. **Setup Secret** (one-time)
   - Run: `./deployment/scripts/setup-github-secrets.sh`
   - Add base64 kubeconfig to GitHub Secrets as `KUBECONFIG`

2. **Deploy**
   - Go to **Actions** tab
   - Select **Deploy Gethrelay to Kubernetes**
   - Click **Run workflow**
   - Select environment and run

3. **Monitor**
   - Watch workflow execution in Actions tab
   - Check deployment logs
   - Verify health checks pass

## Files Structure

```
deployment/
├── README.md                    # Full documentation
├── QUICKSTART.md               # This file
├── k8s/
│   ├── namespace.yaml          # Namespace configuration
│   ├── deployments.yaml        # 10 deployment configurations
│   └── services.yaml           # Service configurations
├── tor/
│   └── torrc                   # Tor configuration
└── scripts/
    ├── setup-github-secrets.sh # Setup GitHub secrets
    └── test-deployment.sh      # Validate deployment

.github/workflows/
└── deploy-gethrelay.yaml       # CI/CD workflow

Dockerfile.gethrelay            # Container image definition
kubeconfig.yaml                 # Kubernetes config (gitignored)
```

## Security Checklist

- [x] kubeconfig.yaml in .gitignore
- [x] kubeconfig stored as GitHub secret (base64 encoded)
- [x] Containers run as non-root users
- [x] Resource limits configured
- [x] Tor isolation per pod
- [x] No persistent storage of secrets

## Support

For detailed documentation, see [deployment/README.md](./README.md)
