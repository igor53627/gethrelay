# Gethrelay Docker Build and Deployment Setup - Complete

## What Has Been Set Up

This document summarizes the complete Docker image build and deployment infrastructure that has been configured for the gethrelay project.

## Files Created/Modified

### 1. GitHub Actions Workflows

#### `/Users/user/pse/ethereum/go-ethereum/.github/workflows/build-gethrelay-image.yaml` (NEW)
**Purpose:** Automated Docker image building and publishing

**Features:**
- Multi-architecture builds (linux/amd64, linux/arm64)
- Automatic tagging with git SHA, branches, and semver
- Build cache optimization
- Image verification
- Supply chain attestation
- Triggered on:
  - Push to main/master (when relevant files change)
  - New releases
  - Manual workflow dispatch

**Publishes to:** `ghcr.io/igor53627/gethrelay`

#### `/Users/user/pse/ethereum/go-ethereum/.github/workflows/deploy-gethrelay.yaml` (UPDATED)
**Changes made:**
- Fixed image name from `ghcr.io/ethereum/gethrelay` to `ghcr.io/igor53627/gethrelay`
- Already includes full deployment workflow with:
  - Docker image build and push
  - Kubernetes deployment (all 10 instances)
  - Health checks
  - Deployment verification

### 2. Kubernetes Manifests

#### `/Users/user/pse/ethereum/go-ethereum/deployment/k8s/deployments.yaml` (UPDATED)
**Changes made:**
- Updated all 10 deployments to use correct image: `ghcr.io/igor53627/gethrelay:latest`
- Previously used incorrect registry: `ghcr.io/ethereum/gethrelay:latest`

**Deployment configuration:**
- 3 default mode instances
- 4 prefer-tor mode instances
- 3 tor-only mode instances
- Each with Tor sidecar container

### 3. Build Scripts

#### `/Users/user/pse/ethereum/go-ethereum/scripts/build-and-push-image.sh` (NEW)
**Purpose:** Manual Docker image building and pushing

**Features:**
- Single or multi-architecture builds
- Automatic GHCR authentication
- Multiple tag support
- Build cache control
- Image testing
- Colored output and error handling

**Usage examples:**
```bash
# Simple local build
./scripts/build-and-push-image.sh

# Multi-arch build and push
./scripts/build-and-push-image.sh -m -p -t latest

# With version tags
./scripts/build-and-push-image.sh -m -p -t v1.0.0 -a latest

# Login and push
export GITHUB_USERNAME="igor53627"
export GITHUB_TOKEN="your-pat"
./scripts/build-and-push-image.sh --login -p
```

### 4. Documentation

#### `/Users/user/pse/ethereum/go-ethereum/deployment/DOCKER_BUILD.md` (NEW)
**Comprehensive 500+ line guide covering:**
- Automated builds with GitHub Actions
- Manual build and push procedures
- Multi-architecture builds
- Authentication and permissions
- Kubernetes deployment (public and private images)
- Troubleshooting common issues
- Security considerations
- CI/CD integration
- Image tagging strategy
- Supply chain security

#### `/Users/user/pse/ethereum/go-ethereum/deployment/README.md` (UPDATED)
**Enhanced with:**
- Docker image build instructions at the top
- Quick start section for image building
- Links to detailed documentation
- Public/private image considerations
- Updated image references throughout

## How to Use

### For Immediate Deployment

#### Step 1: Build and Push Image

**Option A: Using GitHub Actions (Recommended)**
```bash
# Trigger the automated build
gh workflow run build-gethrelay-image.yaml

# Monitor the build
gh run watch

# Verify image was published
gh api /user/packages/container/gethrelay/versions
```

**Option B: Using the Build Script**
```bash
# Set credentials
export GITHUB_USERNAME="igor53627"
export GITHUB_TOKEN="ghp_your_token_here"

# Build multi-arch and push
./scripts/build-and-push-image.sh --login -m -p
```

**Option C: Manual Docker Commands**
```bash
# Login
echo "$GITHUB_TOKEN" | docker login ghcr.io -u igor53627 --password-stdin

# Build and push (multi-arch)
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile.gethrelay \
  -t ghcr.io/igor53627/gethrelay:latest \
  --push \
  .
```

#### Step 2: Make Image Public (One-time)

1. Go to: https://github.com/users/igor53627/packages/container/gethrelay/settings
2. Scroll to "Danger Zone"
3. Click "Change visibility"
4. Select "Public"
5. Confirm the change

This allows Kubernetes to pull the image without authentication.

#### Step 3: Deploy to Kubernetes

```bash
# Deploy all resources
kubectl apply -f deployment/k8s/namespace.yaml
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml

# Verify deployment
kubectl get pods -n gethrelay -w
```

### For Automated CI/CD

The full deployment workflow is already configured and will:

1. **On Release:**
   - Automatically build and push Docker image
   - Deploy to Kubernetes cluster
   - Run health checks
   - Report status

2. **On Manual Trigger:**
   ```bash
   gh workflow run deploy-gethrelay.yaml -f environment=production
   ```

## Troubleshooting Guide

### Issue: ImagePullBackOff in Kubernetes

**Symptom:**
```bash
kubectl get pods -n gethrelay
# Shows: ImagePullBackOff or ErrImagePull
```

**Solutions:**

1. **Verify image exists:**
   ```bash
   docker pull ghcr.io/igor53627/gethrelay:latest
   ```

2. **Make image public:**
   - Follow Step 2 above

3. **Or create imagePullSecret:**
   ```bash
   kubectl create secret docker-registry ghcr-secret \
     --docker-server=ghcr.io \
     --docker-username=igor53627 \
     --docker-password="$GITHUB_TOKEN" \
     --docker-email=your@email.com \
     -n gethrelay

   # Then add to deployments.yaml:
   # spec:
   #   template:
   #     spec:
   #       imagePullSecrets:
   #       - name: ghcr-secret
   ```

### Issue: Build Fails - Tor Config Missing

**Symptom:**
```
ERROR: failed to solve: failed to compute cache key:
"/deployment/tor/torrc" not found
```

**Solution:**
```bash
# Verify torrc exists
ls -la deployment/tor/torrc

# If missing, it should already exist, but you can recreate:
mkdir -p deployment/tor
cat > deployment/tor/torrc <<'EOF'
SocksPort 0.0.0.0:9050
ControlPort 0.0.0.0:9051
CookieAuthentication 1
DataDirectory /var/lib/tor
EOF
```

### Issue: GitHub Actions Build Fails

**Check:**
1. Workflow syntax: `.github/workflows/build-gethrelay-image.yaml`
2. Dockerfile exists: `Dockerfile.gethrelay`
3. Tor config exists: `deployment/tor/torrc`
4. Repository permissions: Settings > Actions > General > Workflow permissions

**Fix permissions if needed:**
- Go to: Settings > Actions > General
- Scroll to "Workflow permissions"
- Select "Read and write permissions"
- Save

### Issue: Multi-arch Build Fails

**Symptom:**
```
ERROR: Multiple platforms feature is currently not supported
```

**Solution:**
```bash
# Set up buildx
docker buildx create --name gethrelay-builder --use
docker buildx inspect --bootstrap

# Install QEMU for cross-platform
docker run --privileged --rm tonistiigi/binfmt --install all

# Retry build
./scripts/build-and-push-image.sh -m -p
```

## Security Checklist

- [ ] GitHub Personal Access Token has only `write:packages` permission
- [ ] Token is stored securely (not in code)
- [ ] KUBECONFIG secret is base64 encoded in GitHub Secrets
- [ ] Image is scanned for vulnerabilities regularly
- [ ] Non-root user configured in Dockerfile (already done: user gethrelay:1000)
- [ ] Resource limits set in Kubernetes deployments (already configured)
- [ ] Regular rotation of credentials

## Testing the Setup

### 1. Test Local Build

```bash
# Build locally
docker build -f Dockerfile.gethrelay -t ghcr.io/igor53627/gethrelay:test .

# Test the image
docker run --rm ghcr.io/igor53627/gethrelay:test --help
docker run --rm ghcr.io/igor53627/gethrelay:test --version
```

### 2. Test Image Push

```bash
# Push test image
docker push ghcr.io/igor53627/gethrelay:test

# Verify it's available
docker pull ghcr.io/igor53627/gethrelay:test
```

### 3. Test Kubernetes Deployment

```bash
# Create test namespace
kubectl create namespace gethrelay-test

# Deploy single instance for testing
kubectl create deployment gethrelay-test \
  --image=ghcr.io/igor53627/gethrelay:latest \
  -n gethrelay-test

# Check status
kubectl get pods -n gethrelay-test

# View logs
kubectl logs -n gethrelay-test -l app=gethrelay-test

# Cleanup
kubectl delete namespace gethrelay-test
```

### 4. Test GitHub Actions Workflow

```bash
# Trigger manually
gh workflow run build-gethrelay-image.yaml

# Watch the run
gh run watch

# View logs if it fails
gh run view --log-failed
```

## Next Steps

### Immediate Actions

1. **Build and publish the image:**
   ```bash
   gh workflow run build-gethrelay-image.yaml
   ```

2. **Make image public** (recommended for easier deployment)

3. **Deploy to Kubernetes:**
   ```bash
   kubectl apply -f deployment/k8s/
   ```

4. **Verify deployment:**
   ```bash
   kubectl get pods -n gethrelay -w
   kubectl logs -n gethrelay -l app=gethrelay --tail=50
   ```

### Ongoing Maintenance

1. **Monitor deployments:**
   ```bash
   kubectl get pods -n gethrelay -o wide
   kubectl top pods -n gethrelay
   ```

2. **Update images on new releases:**
   - Automatic via deploy-gethrelay.yaml workflow
   - Or manually: `kubectl set image deployment/... gethrelay=ghcr.io/igor53627/gethrelay:v1.0.0`

3. **Review logs regularly:**
   ```bash
   kubectl logs -n gethrelay -l app=gethrelay --tail=100 -f
   ```

4. **Check for vulnerabilities:**
   ```bash
   docker scout cves ghcr.io/igor53627/gethrelay:latest
   # or
   trivy image ghcr.io/igor53627/gethrelay:latest
   ```

## File Reference

All created/modified files:

```
/Users/user/pse/ethereum/go-ethereum/
├── .github/workflows/
│   ├── build-gethrelay-image.yaml      (NEW - Automated image builds)
│   └── deploy-gethrelay.yaml           (UPDATED - Fixed image reference)
├── deployment/
│   ├── DOCKER_BUILD.md                 (NEW - Comprehensive build docs)
│   ├── README.md                       (UPDATED - Added build instructions)
│   ├── k8s/
│   │   ├── deployments.yaml            (UPDATED - Fixed image references)
│   │   ├── namespace.yaml              (existing)
│   │   └── services.yaml               (existing)
│   └── tor/
│       └── torrc                       (existing)
├── scripts/
│   └── build-and-push-image.sh         (NEW - Manual build script)
├── Dockerfile.gethrelay                (existing)
└── DOCKER_DEPLOYMENT_SETUP.md          (THIS FILE - Complete summary)
```

## Support Resources

- **Detailed build documentation:** [deployment/DOCKER_BUILD.md](deployment/DOCKER_BUILD.md)
- **Deployment guide:** [deployment/README.md](deployment/README.md)
- **Build script help:** `./scripts/build-and-push-image.sh --help`
- **GitHub Actions:** https://github.com/igor53627/gethrelay/actions
- **Container registry:** https://github.com/igor53627/packages/container/gethrelay

## Summary

Everything is now set up for:
1. Building multi-architecture Docker images (amd64, arm64)
2. Publishing to GitHub Container Registry (ghcr.io/igor53627/gethrelay)
3. Automated builds via GitHub Actions
4. Manual builds via convenient script
5. Kubernetes deployment with all 10 instances
6. Comprehensive documentation and troubleshooting guides

The Kubernetes deployments are ready to use the correct image reference and will work as soon as the image is built and published.
