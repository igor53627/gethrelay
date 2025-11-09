# Gethrelay Kubernetes Deployment - Documentation Index

Complete index of all deployment documentation and resources.

## Quick Navigation

| Document | Purpose | Audience |
|----------|---------|----------|
| [QUICKSTART.md](./QUICKSTART.md) | TL;DR deployment steps | Everyone |
| [CHECKLIST.md](./CHECKLIST.md) | Step-by-step deployment checklist | Operators |
| [README.md](./README.md) | Complete documentation | DevOps/Developers |
| [DEPLOYMENT_SUMMARY.md](./DEPLOYMENT_SUMMARY.md) | Architecture and design overview | Technical leads |
| [INDEX.md](./INDEX.md) | This file - documentation index | Everyone |

## Documentation Structure

### Getting Started (Start Here!)

1. **[QUICKSTART.md](./QUICKSTART.md)** - Quick Start Guide
   - TL;DR deployment commands
   - Instance distribution overview
   - Common commands reference
   - Quick troubleshooting

2. **[CHECKLIST.md](./CHECKLIST.md)** - Deployment Checklist
   - Pre-deployment verification
   - Step-by-step deployment process
   - Post-deployment validation
   - Success criteria
   - Rollback procedures

### Detailed Documentation

3. **[README.md](./README.md)** - Complete Deployment Guide
   - Full architecture overview
   - Prerequisites and setup
   - Tor configuration modes explained
   - CI/CD workflow details
   - Monitoring and maintenance
   - Troubleshooting guide
   - Performance tuning
   - Security considerations

4. **[DEPLOYMENT_SUMMARY.md](./DEPLOYMENT_SUMMARY.md)** - Architecture Summary
   - What was created (all files)
   - Design decisions explained
   - Security measures
   - Next steps
   - Resource expectations

## File Structure

### Kubernetes Manifests (`k8s/`)

```
k8s/
├── namespace.yaml          # Namespace: gethrelay
├── deployments.yaml        # 10 Deployment resources
└── services.yaml          # 11 Service resources
```

**Purpose:** Kubernetes resource definitions for deploying gethrelay instances.

**Usage:**
```bash
kubectl apply -f deployment/k8s/
```

### Scripts (`scripts/`)

```
scripts/
├── setup-github-secrets.sh    # GitHub Secrets setup helper
└── test-deployment.sh        # Deployment validation script
```

**setup-github-secrets.sh**
- Base64 encodes kubeconfig
- Provides GitHub Secrets setup instructions
- Validates kubeconfig
- Optional clipboard copy

**Usage:**
```bash
./deployment/scripts/setup-github-secrets.sh kubeconfig.yaml
```

**test-deployment.sh**
- Validates all prerequisites
- Tests YAML manifests
- Verifies cluster connectivity
- Checks deployment distribution
- Runs dry-run tests

**Usage:**
```bash
./deployment/scripts/test-deployment.sh kubeconfig.yaml
```

### Configuration (`tor/`)

```
tor/
└── torrc                  # Tor daemon configuration
```

**Purpose:** Tor proxy configuration for privacy-preserving P2P networking.

**Configuration:**
- SOCKS5 proxy: port 9050
- Control port: 9051
- Cookie authentication
- Performance tuned

## Container Image

**Dockerfile.gethrelay** (project root)
- Multi-stage build (build + runtime)
- Alpine-based minimal image
- Includes Tor daemon
- Non-root user security
- Multi-architecture support (amd64/arm64)

**Build:**
```bash
docker build -f Dockerfile.gethrelay -t gethrelay:local .
```

## CI/CD Pipeline

**`.github/workflows/deploy-gethrelay.yaml`**

Three-job workflow:
1. **build-and-push** - Builds and publishes container image
2. **deploy-to-kubernetes** - Deploys to cluster
3. **health-check** - Validates deployment

**Triggers:**
- Automatic on release publish
- Manual workflow dispatch

**Secrets Required:**
- `KUBECONFIG` - Base64 encoded kubeconfig

## Deployment Configurations

### Instance Distribution

| Mode | Instances | Configuration |
|------|-----------|---------------|
| Default | 3 | `--tor-proxy=127.0.0.1:9050` |
| Prefer-Tor | 4 | `--tor-proxy=127.0.0.1:9050 --prefer-tor` |
| Tor-Only | 3 | `--tor-proxy=127.0.0.1:9050 --only-onion` |
| **Total** | **10** | |

### Tor Mode Behaviors

**Default Mode** (3 instances)
- Connects to .onion addresses via Tor when available
- Falls back to clearnet on Tor connection failure
- Uses clearnet for peers without .onion addresses
- **Use case:** Balanced privacy and connectivity

**Prefer-Tor Mode** (4 instances)
- Prefers .onion addresses when both .onion and clearnet available
- Falls back to clearnet on Tor connection failure
- Uses clearnet for peers without .onion addresses
- **Use case:** Enhanced privacy with connectivity fallback

**Tor-Only Mode** (3 instances)
- Only connects to peers with .onion addresses
- Rejects peers without .onion addresses
- No clearnet fallback
- **Use case:** Maximum privacy, fewer peers expected

### Resource Allocation

**Per Pod:**
- Tor container: 100m-500m CPU, 128Mi-512Mi memory
- Gethrelay container: 200m-1000m CPU, 256Mi-1Gi memory

**Total (10 pods):**
- CPU: 3-8 cores
- Memory: 3-8 GB

## Common Workflows

### First Time Deployment

1. Read [QUICKSTART.md](./QUICKSTART.md)
2. Follow [CHECKLIST.md](./CHECKLIST.md)
3. Run `./deployment/scripts/setup-github-secrets.sh`
4. Deploy via GitHub Actions or kubectl

### Updating Deployment

1. Update manifests in `deployment/k8s/`
2. Test with `./deployment/scripts/test-deployment.sh`
3. Commit and push changes
4. Deploy via GitHub Actions

### Troubleshooting

1. Check [QUICKSTART.md](./QUICKSTART.md) troubleshooting section
2. Review [README.md](./README.md) detailed troubleshooting
3. Use validation script: `./deployment/scripts/test-deployment.sh`
4. Check pod logs: `kubectl logs -n gethrelay <pod>`

### Monitoring

1. Pod status: `kubectl get pods -n gethrelay`
2. Resource usage: `kubectl top pods -n gethrelay`
3. Logs: `kubectl logs -n gethrelay <pod> -c gethrelay -f`
4. Events: `kubectl get events -n gethrelay`

## Key Commands Reference

### Deployment

```bash
# Deploy all resources
kubectl apply -f deployment/k8s/

# Deploy individual resources
kubectl apply -f deployment/k8s/namespace.yaml
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml
```

### Monitoring

```bash
# Watch pods
kubectl get pods -n gethrelay -w

# Check deployments
kubectl get deployments -n gethrelay

# View logs
kubectl logs -n gethrelay <pod-name> -c gethrelay -f

# Check by mode
kubectl get pods -n gethrelay -l mode=default
kubectl get pods -n gethrelay -l mode=prefer-tor
kubectl get pods -n gethrelay -l mode=only-onion
```

### Troubleshooting

```bash
# Describe pod
kubectl describe pod <pod-name> -n gethrelay

# Check events
kubectl get events -n gethrelay --sort-by='.lastTimestamp'

# Resource usage
kubectl top pods -n gethrelay

# Execute in pod
kubectl exec -it <pod-name> -n gethrelay -c gethrelay -- sh
```

### Cleanup

```bash
# Delete all resources
kubectl delete namespace gethrelay

# Delete specific deployment
kubectl delete deployment <deployment-name> -n gethrelay
```

## Security Files

### .gitignore (updated)

Added entries:
```
# Kubernetes config
kubeconfig.yaml
**/kubeconfig*.yaml
```

**Critical:** Prevents accidental commit of cluster credentials.

**Verification:**
```bash
git check-ignore -v kubeconfig.yaml
# Should output: .gitignore:62:**/kubeconfig*.yaml	kubeconfig.yaml
```

## Support Resources

### Documentation
- This index
- Individual guide files
- Inline comments in manifests
- Script help output

### Validation Tools
- `test-deployment.sh` - Comprehensive validation
- `setup-github-secrets.sh` - Secret configuration
- `kubectl` dry-run commands

### Debugging
- Pod logs: `kubectl logs`
- Pod describe: `kubectl describe pod`
- Events: `kubectl get events`
- Resource usage: `kubectl top`

## Best Practices

1. **Always validate before deploying**
   ```bash
   ./deployment/scripts/test-deployment.sh kubeconfig.yaml
   ```

2. **Never commit secrets**
   - kubeconfig.yaml is gitignored
   - Use GitHub Secrets for CI/CD
   - Rotate credentials regularly

3. **Monitor after deployment**
   ```bash
   kubectl get pods -n gethrelay -w
   ```

4. **Check logs for errors**
   ```bash
   kubectl logs -n gethrelay <pod> -c gethrelay
   ```

5. **Use CI/CD for production**
   - GitHub Actions workflow
   - Automated testing
   - Rollback capability

## FAQ

**Q: How do I deploy for the first time?**
A: Follow [QUICKSTART.md](./QUICKSTART.md) or [CHECKLIST.md](./CHECKLIST.md).

**Q: What if a pod fails to start?**
A: Check logs with `kubectl logs <pod> -n gethrelay -c gethrelay` and see [README.md](./README.md) troubleshooting section.

**Q: How do I update the deployment?**
A: Modify manifests, test with validation script, then deploy via GitHub Actions or kubectl.

**Q: Can I scale the deployments?**
A: Not recommended for P2P nodes. Each deployment should remain at 1 replica.

**Q: How do I change Tor settings?**
A: Edit `deployment/tor/torrc` or deployment args in `deployment/k8s/deployments.yaml`.

**Q: What if GitHub Actions fails?**
A: Check workflow logs, verify KUBECONFIG secret, test kubeconfig locally.

**Q: How do I rollback?**
A: Use `kubectl rollout undo` or re-run previous GitHub Actions workflow.

## Version History

- **v1.0.0** - Initial deployment infrastructure
  - 10 gethrelay instances
  - 3 Tor configuration modes
  - GitHub Actions CI/CD
  - Complete documentation

## Contributing

When updating deployment infrastructure:

1. Update relevant documentation
2. Test changes with validation script
3. Update this index if adding new files
4. Follow security best practices
5. Update DEPLOYMENT_SUMMARY.md with changes

## License

Same as go-ethereum project (GNU LGPL v3).

---

**Quick Links:**
- [QUICKSTART.md](./QUICKSTART.md) - Start here!
- [CHECKLIST.md](./CHECKLIST.md) - Deployment steps
- [README.md](./README.md) - Full documentation
- [DEPLOYMENT_SUMMARY.md](./DEPLOYMENT_SUMMARY.md) - Architecture
