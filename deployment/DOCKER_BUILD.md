# Gethrelay Docker Image Build and Deployment Guide

## Overview

This guide covers building, pushing, and deploying the gethrelay Docker image to GitHub Container Registry (ghcr.io).

## Image Details

- **Registry:** GitHub Container Registry (ghcr.io)
- **Image Name:** `ghcr.io/igor53627/gethrelay`
- **Supported Architectures:** linux/amd64, linux/arm64
- **Base Image:** golang:1.23-alpine (builder), alpine:latest (runtime)
- **Includes:** Tor client for privacy-preserving P2P networking

## Automated Builds

### GitHub Actions Workflows

The repository includes two GitHub Actions workflows:

#### 1. Build and Push Image (`build-gethrelay-image.yaml`)

**Triggers:**
- Push to `main` or `master` branch (when relevant files change)
- New releases/tags
- Manual workflow dispatch

**What it does:**
- Builds multi-architecture Docker image (amd64, arm64)
- Pushes to ghcr.io/igor53627/gethrelay
- Tags with:
  - `latest` (for main branch)
  - Git commit SHA (e.g., `main-abc123`)
  - Semver tags (for releases, e.g., `v1.0.0`, `1.0`, `1`)
- Generates build attestation for supply chain security
- Verifies the built image

**Usage:**
```bash
# Trigger manually via GitHub CLI
gh workflow run build-gethrelay-image.yaml

# Trigger manually via GitHub UI
# Go to Actions > Build and Push Gethrelay Image > Run workflow
```

#### 2. Full Deployment Workflow (`deploy-gethrelay.yaml`)

**Triggers:**
- New releases
- Manual workflow dispatch

**What it does:**
- Builds and pushes Docker image
- Deploys to Kubernetes cluster
- Updates all 10 deployments (3 default, 4 prefer-tor, 3 only-onion)
- Performs health checks
- Sends notifications

**Usage:**
```bash
# Trigger deployment via GitHub CLI
gh workflow run deploy-gethrelay.yaml -f environment=production

# Trigger via GitHub UI
# Go to Actions > Deploy Gethrelay to Kubernetes > Run workflow
```

## Manual Build and Push

### Prerequisites

1. **Docker installed** (with BuildKit support)
2. **GitHub Personal Access Token (PAT)** with `write:packages` permission

### Step 1: Authenticate with GitHub Container Registry

```bash
# Create a PAT at https://github.com/settings/tokens
# Required scopes: write:packages, read:packages

# Login to ghcr.io
echo "YOUR_GITHUB_PAT" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

### Step 2: Build the Image

#### Single Architecture (local testing)

```bash
# Build for your current architecture
docker build -f Dockerfile.gethrelay -t ghcr.io/igor53627/gethrelay:latest .

# Test the image locally
docker run --rm ghcr.io/igor53627/gethrelay:latest --version
```

#### Multi-Architecture (production)

```bash
# Create a buildx builder (first time only)
docker buildx create --name gethrelay-builder --use
docker buildx inspect --bootstrap

# Build for multiple architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile.gethrelay \
  -t ghcr.io/igor53627/gethrelay:latest \
  --push \
  .
```

### Step 3: Tag and Push

```bash
# Tag with version (optional)
docker tag ghcr.io/igor53627/gethrelay:latest ghcr.io/igor53627/gethrelay:v1.0.0

# Push to registry
docker push ghcr.io/igor53627/gethrelay:latest
docker push ghcr.io/igor53627/gethrelay:v1.0.0

# For multi-architecture builds, the --push flag in buildx already handles this
```

### Step 4: Verify the Push

```bash
# Pull and verify
docker pull ghcr.io/igor53627/gethrelay:latest
docker inspect ghcr.io/igor53627/gethrelay:latest

# Check available tags
gh api /user/packages/container/gethrelay/versions
```

## Making the Image Public

By default, GitHub Container Registry packages are private. To make the image public:

1. Go to https://github.com/users/igor53627/packages/container/gethrelay/settings
2. Scroll to "Danger Zone"
3. Click "Change visibility"
4. Select "Public"
5. Confirm the change

**Note:** Public images don't require authentication to pull, making Kubernetes deployment simpler.

## Kubernetes Deployment

### Option 1: Public Image (Recommended)

If the image is public, no additional configuration is needed:

```bash
# Deploy with public image
kubectl apply -f deployment/k8s/namespace.yaml
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml
```

### Option 2: Private Image with Pull Secrets

If the image remains private, create an imagePullSecret:

```bash
# Create secret in Kubernetes
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password=YOUR_GITHUB_PAT \
  --docker-email=YOUR_EMAIL \
  -n gethrelay

# Update deployments.yaml to use the secret (add to each deployment spec)
# spec:
#   template:
#     spec:
#       imagePullSecrets:
#       - name: ghcr-secret
#       containers:
#       - name: gethrelay
#         ...
```

### Verify Deployment

```bash
# Check pod status
kubectl get pods -n gethrelay

# Check for image pull errors
kubectl describe pods -n gethrelay | grep -A5 "Events:"

# View logs
kubectl logs -n gethrelay -l app=gethrelay --tail=50
```

## Troubleshooting

### ImagePullBackOff / ErrImagePull

**Cause:** Image doesn't exist or authentication failed

**Solutions:**
1. Verify image exists: `docker pull ghcr.io/igor53627/gethrelay:latest`
2. Check image visibility: https://github.com/users/igor53627/packages/container/gethrelay
3. Make image public (see "Making the Image Public" above)
4. Or create imagePullSecret (see "Private Image with Pull Secrets" above)

### Build Fails in GitHub Actions

**Cause:** Missing Tor configuration file

**Solution:**
```bash
# Ensure torrc exists
ls -la deployment/tor/torrc

# If missing, create it:
mkdir -p deployment/tor
cat > deployment/tor/torrc <<'EOF'
SocksPort 0.0.0.0:9050
ControlPort 0.0.0.0:9051
CookieAuthentication 1
DataDirectory /var/lib/tor
EOF
```

### Multi-arch Build Fails

**Cause:** Buildx not configured or QEMU not installed

**Solution:**
```bash
# Install QEMU
docker run --privileged --rm tonistiigi/binfmt --install all

# Create and use buildx builder
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap
```

### Authentication Errors

**Cause:** Token expired or insufficient permissions

**Solution:**
```bash
# Generate new PAT with write:packages scope
# https://github.com/settings/tokens

# Re-authenticate
echo "NEW_GITHUB_PAT" | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

## Image Tagging Strategy

### Automated Tags (GitHub Actions)

- `latest` - Always points to the latest main branch build
- `main-<sha>` - Specific commit from main branch (e.g., `main-abc1234`)
- `v1.0.0` - Semantic version tag (from releases)
- `1.0` - Major.minor version (from releases)
- `1` - Major version (from releases)

### Manual Tags (Local Builds)

```bash
# Development builds
docker tag ghcr.io/igor53627/gethrelay:latest ghcr.io/igor53627/gethrelay:dev

# Feature branches
docker tag ghcr.io/igor53627/gethrelay:latest ghcr.io/igor53627/gethrelay:feature-tor-integration

# Release candidates
docker tag ghcr.io/igor53627/gethrelay:latest ghcr.io/igor53627/gethrelay:v1.0.0-rc1
```

## Security Considerations

### Supply Chain Security

The automated workflow includes:
- Build attestation generation
- Provenance tracking
- SHA256 digest pinning

### Scanning for Vulnerabilities

```bash
# Scan image with Docker Scout (if available)
docker scout cves ghcr.io/igor53627/gethrelay:latest

# Or use Trivy
trivy image ghcr.io/igor53627/gethrelay:latest
```

### Best Practices

1. **Use specific tags** in production (not `latest`)
2. **Pin base images** by digest in Dockerfile
3. **Scan regularly** for vulnerabilities
4. **Rotate credentials** periodically
5. **Use read-only** filesystems where possible
6. **Run as non-root** user (already configured)

## CI/CD Integration

### GitHub Actions Secrets Required

For the deployment workflow to work, configure these secrets:

```bash
# Repository Settings > Secrets and variables > Actions

KUBECONFIG - Base64-encoded Kubernetes config file
```

To get the base64-encoded kubeconfig:

```bash
# On macOS/Linux
cat ~/.kube/config | base64

# On macOS (no line wrapping)
cat ~/.kube/config | base64 | tr -d '\n'
```

## Quick Reference

### Common Commands

```bash
# Build locally
docker build -f Dockerfile.gethrelay -t ghcr.io/igor53627/gethrelay:latest .

# Test locally
docker run --rm ghcr.io/igor53627/gethrelay:latest --help

# Push to registry
docker push ghcr.io/igor53627/gethrelay:latest

# Pull from registry
docker pull ghcr.io/igor53627/gethrelay:latest

# Deploy to Kubernetes
kubectl apply -f deployment/k8s/

# Check deployment status
kubectl get pods -n gethrelay -w

# View logs
kubectl logs -n gethrelay -l app=gethrelay --tail=100 -f
```

## Additional Resources

- [GitHub Container Registry Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [Kubernetes Image Pull Secrets](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
- [GitHub Actions Workflows](https://docs.github.com/en/actions/using-workflows)
