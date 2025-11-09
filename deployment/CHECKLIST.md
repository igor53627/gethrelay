# Deployment Checklist

Use this checklist to ensure proper deployment of gethrelay instances to Kubernetes.

## Pre-Deployment Checklist

### Prerequisites
- [ ] kubectl CLI installed (`kubectl version`)
- [ ] Access to Kubernetes cluster
- [ ] Kubeconfig file obtained and copied to project
- [ ] GitHub repository access with admin permissions
- [ ] Docker installed (for local testing, optional)

### Security Verification
- [ ] Kubeconfig file is in project directory (`kubeconfig.yaml`)
- [ ] Kubeconfig is listed in `.gitignore`
- [ ] Verified kubeconfig is NOT staged for commit (`git status`)
- [ ] GitHub account has package write permissions

### Configuration Review
- [ ] Reviewed `deployment/k8s/deployments.yaml`
- [ ] Verified 10 deployments configured (3 default, 4 prefer-tor, 3 only-onion)
- [ ] Reviewed resource limits appropriate for cluster
- [ ] Verified Tor configuration in `deployment/tor/torrc`
- [ ] Reviewed Dockerfile.gethrelay for security issues

## Validation Steps

### Local Validation
- [ ] Run validation script: `./deployment/scripts/test-deployment.sh kubeconfig.yaml`
- [ ] All validation checks passed ✓
- [ ] Cluster connectivity confirmed
- [ ] YAML manifests valid
- [ ] Deployment distribution correct (10 instances)

### GitHub Setup
- [ ] Run setup script: `./deployment/scripts/setup-github-secrets.sh kubeconfig.yaml`
- [ ] Copy base64 kubeconfig output
- [ ] Add to GitHub Secrets as `KUBECONFIG`
  - [ ] Navigate to: Settings > Secrets and variables > Actions
  - [ ] Click "New repository secret"
  - [ ] Name: `KUBECONFIG`
  - [ ] Value: <base64 encoded kubeconfig>
  - [ ] Click "Add secret"
- [ ] Verify secret added successfully

## Deployment Options

Choose ONE deployment method:

### Option A: GitHub Actions Deployment (Recommended)

- [ ] Navigate to repository Actions tab
- [ ] Select "Deploy Gethrelay to Kubernetes" workflow
- [ ] Click "Run workflow" button
- [ ] Select environment: `production`
- [ ] Click "Run workflow" to start
- [ ] Monitor workflow execution
- [ ] Wait for all jobs to complete
- [ ] Verify "All checks have passed" ✓

### Option B: Manual kubectl Deployment

```bash
# Set kubeconfig
export KUBECONFIG=./kubeconfig.yaml

# Verify cluster access
kubectl cluster-info

# Deploy
kubectl apply -f deployment/k8s/namespace.yaml
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml

# Monitor
kubectl get pods -n gethrelay -w
```

- [ ] Export KUBECONFIG environment variable
- [ ] Verify cluster connection
- [ ] Apply namespace manifest
- [ ] Apply deployments manifest
- [ ] Apply services manifest
- [ ] All resources created successfully

## Post-Deployment Verification

### Pod Status
```bash
kubectl get pods -n gethrelay
```

- [ ] All 10 pods in `Running` state
- [ ] No pods in `CrashLoopBackOff` or `Error` state
- [ ] Each pod shows `2/2` containers ready

### Deployment Status
```bash
kubectl get deployments -n gethrelay
```

- [ ] All 10 deployments shown
- [ ] Each deployment shows `1/1` ready
- [ ] No deployment shows `0/1` available

### Service Status
```bash
kubectl get services -n gethrelay
```

- [ ] 11 services created (1 headless + 10 NodePort)
- [ ] All services have valid endpoints
- [ ] NodePort services have port assignments

### Pod Distribution
```bash
kubectl get pods -n gethrelay -l mode=default      # Should show 3
kubectl get pods -n gethrelay -l mode=prefer-tor   # Should show 4
kubectl get pods -n gethrelay -l mode=only-onion   # Should show 3
```

- [ ] 3 pods with label `mode=default`
- [ ] 4 pods with label `mode=prefer-tor`
- [ ] 3 pods with label `mode=only-onion`

### Container Health
For each pod, check both containers are healthy:

```bash
# Pick a pod name from the list
kubectl describe pod <pod-name> -n gethrelay
```

- [ ] Both containers (tor, gethrelay) in `Running` state
- [ ] No recent restarts
- [ ] No error events in pod description

### Log Verification
```bash
# Check gethrelay logs
kubectl logs -n gethrelay <pod-name> -c gethrelay --tail=50

# Check Tor logs
kubectl logs -n gethrelay <pod-name> -c tor --tail=50
```

For each mode, check at least one pod:

**Default mode pod:**
- [ ] Gethrelay started successfully
- [ ] P2P server listening on port 30303
- [ ] Tor proxy connection established
- [ ] No critical errors in logs

**Prefer-Tor mode pod:**
- [ ] Gethrelay started with `--prefer-tor` flag
- [ ] Tor preference active in logs
- [ ] P2P connections establishing
- [ ] No critical errors in logs

**Tor-Only mode pod:**
- [ ] Gethrelay started with `--only-onion` flag
- [ ] Only-onion mode active in logs
- [ ] Tor circuits established
- [ ] No critical errors in logs

### Tor Circuit Verification
```bash
kubectl logs -n gethrelay <pod-name> -c tor --tail=100 | grep -i "circuit"
```

- [ ] Tor circuits building successfully
- [ ] No repeated circuit failures
- [ ] SOCKS proxy responding

### Resource Usage
```bash
kubectl top pods -n gethrelay
```

- [ ] CPU usage within limits (< 1000m per pod)
- [ ] Memory usage within limits (< 1Gi per pod)
- [ ] No pods showing high resource pressure

### Events Check
```bash
kubectl get events -n gethrelay --sort-by='.lastTimestamp' | tail -20
```

- [ ] No error events in recent events
- [ ] No warning events requiring action
- [ ] Successful pod creation events visible

### Network Connectivity
```bash
# Get NodePort assignments
kubectl get services -n gethrelay -o wide

# Test from inside cluster
kubectl exec -it <pod-name> -n gethrelay -c gethrelay -- sh
# Inside pod: netstat -tlnp
```

- [ ] Port 30303 listening in each pod
- [ ] Tor SOCKS proxy (9050) accessible
- [ ] No network policy blocking connections

## Monitoring Setup (Optional but Recommended)

### Metrics Collection
- [ ] Consider enabling Prometheus monitoring
- [ ] Add pod annotations for scraping
- [ ] Set up Grafana dashboards
- [ ] Configure alerting rules

### Log Aggregation
- [ ] Consider setting up log aggregation (ELK, Loki)
- [ ] Configure log retention policies
- [ ] Set up log-based alerts

## Cleanup Verification (if needed)

If deployment fails and cleanup is needed:

```bash
# Delete everything
kubectl delete namespace gethrelay

# Verify cleanup
kubectl get namespace gethrelay  # Should return "not found"
```

- [ ] Namespace deleted successfully
- [ ] All pods terminated
- [ ] All services removed
- [ ] No orphaned resources

## Rollback Plan

If issues occur after deployment:

### Via GitHub Actions
- [ ] Go to Actions > Deploy Gethrelay to Kubernetes
- [ ] Find previous successful workflow run
- [ ] Click "Re-run jobs"

### Via kubectl
```bash
# Rollback specific deployment
kubectl rollout undo deployment/gethrelay-default-1 -n gethrelay

# Check rollout status
kubectl rollout status deployment/gethrelay-default-1 -n gethrelay
```

- [ ] Identify problematic deployment
- [ ] Execute rollback command
- [ ] Verify rollback successful
- [ ] Check pod status after rollback

## Success Criteria

Deployment is successful when ALL of the following are true:

- [x] All pre-deployment checks passed
- [x] All validation steps successful
- [x] GitHub secret configured (if using Actions)
- [x] Deployment method executed without errors
- [x] All 10 pods in Running state (2/2 containers)
- [x] All deployments show 1/1 ready
- [x] All services created and have endpoints
- [x] Pod distribution correct (3/4/3)
- [x] No critical errors in any pod logs
- [x] Tor circuits established in all pods
- [x] Resource usage within limits
- [x] No error events in cluster
- [x] P2P networking functional

## Common Issues and Solutions

### Issue: Pods stuck in Pending
**Solution:**
- Check node resources: `kubectl describe node`
- Check pod events: `kubectl describe pod <pod-name> -n gethrelay`
- Verify resource requests are achievable

### Issue: Pods in CrashLoopBackOff
**Solution:**
- Check logs: `kubectl logs <pod-name> -n gethrelay -c gethrelay`
- Check Tor logs: `kubectl logs <pod-name> -n gethrelay -c tor`
- Verify image pull succeeded
- Check configuration errors

### Issue: Image pull failures
**Solution:**
- Verify image exists: Check GitHub Container Registry
- Check image name/tag in deployment.yaml
- Verify GITHUB_TOKEN permissions
- Try manual pull: `docker pull ghcr.io/ethereum/gethrelay:latest`

### Issue: Namespace already exists
**Solution:**
- Check existing resources: `kubectl get all -n gethrelay`
- Delete if needed: `kubectl delete namespace gethrelay`
- Wait for termination to complete
- Retry deployment

### Issue: GitHub Actions workflow fails
**Solution:**
- Check workflow logs in Actions tab
- Verify KUBECONFIG secret is correct
- Test kubeconfig locally
- Check cluster accessibility
- Verify GITHUB_TOKEN permissions

## Documentation References

- Full deployment guide: `deployment/README.md`
- Quick start guide: `deployment/QUICKSTART.md`
- Deployment summary: `deployment/DEPLOYMENT_SUMMARY.md`
- Test script: `deployment/scripts/test-deployment.sh`
- Setup script: `deployment/scripts/setup-github-secrets.sh`

## Sign-Off

Deployment completed and verified by: ________________

Date: ________________

Issues encountered (if any): ________________________________

Notes: ________________________________

---

**Remember:**
- NEVER commit kubeconfig.yaml to git
- Rotate secrets regularly
- Monitor resource usage
- Keep documentation updated
