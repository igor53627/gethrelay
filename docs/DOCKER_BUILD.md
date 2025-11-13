# Docker Build Optimization Guide

## Overview

This guide covers the optimized Docker build workflow for the gethrelay project, including both CI/CD builds via GitHub Actions and local builds for rapid iteration.

## Build Time Improvements

### Before Optimization
- Multi-architecture builds (linux/amd64 + linux/arm64)
- Build time: ~18-20 minutes per commit
- QEMU emulation overhead for ARM builds

### After Optimization
- Single architecture build (linux/amd64)
- Build time: ~5-10 minutes per commit (2-4x faster)
- GitHub Actions cache for Docker layers
- No QEMU overhead

## GitHub Actions CI/CD Build

### Automatic Triggers

The workflow automatically builds when:
- Push to `main` or `master` branches
- Changes to relevant files:
  - `Dockerfile.gethrelay`
  - `cmd/gethrelay/**`
  - `node/**`
  - `p2p/**`
  - `go.mod`, `go.sum`
  - `deployment/tor/**`
- Release published
- Manual workflow dispatch

### Build Configuration

**File:** `.github/workflows/build-gethrelay-image.yaml`

**Key optimizations:**
- Single platform: `linux/amd64`
- GitHub Actions cache: `type=gha,mode=max`
- BuildKit inline cache: `BUILDKIT_INLINE_CACHE=1`
- No QEMU setup required (native amd64 builds)

### Image Tags

The workflow automatically creates multiple tags:
- `latest` - Latest main/master branch build
- `<branch>-<sha>` - Branch name + commit SHA
- `<branch>` - Latest commit on branch
- Semver tags for releases (`v1.0.0`, `v1.0`, `v1`)

### Registry

Images are pushed to GitHub Container Registry:
```
ghcr.io/igor53627/gethrelay:latest
ghcr.io/igor53627/gethrelay:main-a1b2c3d
```

## Local Development Builds

### Quick Start

For rapid iteration during development:

```bash
# Basic build with auto-generated tag
./scripts/docker-build-local.sh

# Custom tag
TAG=feature-xyz ./scripts/docker-build-local.sh

# Build and auto-push
./scripts/docker-build-local.sh
# (script will prompt to push)
```

### Script Features

**Location:** `scripts/docker-build-local.sh`

**Capabilities:**
- Builds linux/amd64 image (matches K8s cluster)
- Auto-generates timestamp-based tags
- Creates both timestamped tag and `local` tag
- Interactive push prompt
- Provides kubectl deployment commands

### Environment Variables

Customize the build with environment variables:

```bash
# Custom registry
REGISTRY=my-registry.io ./scripts/docker-build-local.sh

# Custom image name
IMAGE_NAME=my-org/gethrelay ./scripts/docker-build-local.sh

# Custom tag
TAG=v1.2.3-dev ./scripts/docker-build-local.sh

# Custom Dockerfile
DOCKERFILE=Dockerfile.custom ./scripts/docker-build-local.sh
```

### Manual Docker Build

For advanced control:

```bash
# Basic build
docker build -f Dockerfile.gethrelay -t ghcr.io/igor53627/gethrelay:local .

# With BuildKit cache
docker build \
  --file Dockerfile.gethrelay \
  --tag ghcr.io/igor53627/gethrelay:dev \
  --platform linux/amd64 \
  --build-arg BUILDKIT_INLINE_CACHE=1 \
  .

# Push to registry
docker push ghcr.io/igor53627/gethrelay:dev
```

## When to Use Each Workflow

### Use CI/CD Builds When:
- Deploying to production
- Creating releases
- Sharing with team
- Need consistent, reproducible builds
- Collaborating on features

### Use Local Builds When:
- Rapid iteration during development
- Testing Dockerfile changes
- Debugging build issues
- Need immediate feedback (no CI wait)
- Experimenting with configurations

## Build Cache Optimization

### GitHub Actions Cache

The workflow uses GitHub Actions cache to speed up builds:

```yaml
cache-from: type=gha
cache-to: type=gha,mode=max
```

**Cache behavior:**
- Caches all Docker layers (mode=max)
- Separate cache per branch
- Automatic cleanup of old cache entries
- Shared across workflow runs

### Local Docker Cache

Docker automatically caches layers locally:

```bash
# View cached images
docker images ghcr.io/igor53627/gethrelay

# Clear local cache if needed
docker builder prune
docker system prune -a
```

### Dockerfile Optimization

The Dockerfile is already optimized for caching:

1. **Go module download** (changes infrequently)
   ```dockerfile
   COPY go.mod go.sum ./
   RUN go mod download
   ```

2. **Source code copy** (changes frequently)
   ```dockerfile
   COPY . .
   RUN go build -o /gethrelay ./cmd/gethrelay
   ```

This ensures Go dependencies are cached and only rebuilt when `go.mod`/`go.sum` change.

## Architecture Decision

### Why linux/amd64 Only?

**Kubernetes Deployment Analysis:**
- Checked K8s deployments in `deployment/k8s/deployments.yaml`
- No architecture constraints specified
- Standard cloud K8s clusters use linux/amd64
- GitHub Actions runners are linux/amd64

**Benefits:**
- 2-4x faster builds (no cross-compilation)
- No QEMU emulation overhead
- Simpler workflow (fewer moving parts)
- Matches target deployment environment

**Adding ARM Support:**
If ARM builds are needed in the future:

```yaml
platforms: linux/amd64,linux/arm64
```

Re-add QEMU setup:
```yaml
- name: Set up QEMU
  uses: docker/setup-qemu-action@v3
```

## Testing Images

### Local Testing

```bash
# Check version
docker run --rm ghcr.io/igor53627/gethrelay:local --version

# Run with Tor proxy
docker run --rm \
  -p 30303:30303 \
  ghcr.io/igor53627/gethrelay:local \
  --chain=mainnet \
  --maxpeers=200 \
  --tor-proxy=host.docker.internal:9050
```

### Kubernetes Testing

```bash
# Update deployment with new image
kubectl set image deployment/gethrelay-default-1 \
  gethrelay=ghcr.io/igor53627/gethrelay:local \
  -n gethrelay

# Check deployment status
kubectl rollout status deployment/gethrelay-default-1 -n gethrelay

# View logs
kubectl logs -f deployment/gethrelay-default-1 -n gethrelay -c gethrelay
```

## Troubleshooting

### Build Fails in CI

1. **Check GitHub Actions logs** - View detailed build output
2. **Test locally** - Run `./scripts/docker-build-local.sh`
3. **Clear cache** - Re-run workflow to rebuild from scratch
4. **Check Dockerfile** - Verify Dockerfile.gethrelay is valid

### Local Build Issues

```bash
# Docker not running
sudo systemctl start docker  # Linux
open -a Docker              # macOS

# Permission denied
sudo usermod -aG docker $USER  # Linux
# Then logout and login

# Out of disk space
docker system prune -a --volumes

# Cache issues
docker builder prune -a
```

### Registry Authentication

```bash
# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Or use personal access token
docker login ghcr.io
# Username: your-github-username
# Password: ghp_your_personal_access_token
```

## Performance Benchmarks

### Build Time Comparison

| Configuration | First Build | Cached Build | Notes |
|--------------|-------------|--------------|-------|
| Multi-arch (amd64+arm64) | ~18-20 min | ~15 min | QEMU overhead |
| Single-arch (amd64) | ~8-10 min | ~5-7 min | Native build |
| Local build | ~5-8 min | ~3-5 min | Local cache |

### Cache Hit Rates

- **Go module cache:** ~90% (changes infrequently)
- **Alpine base layers:** ~100% (very stable)
- **Source code layers:** ~30-50% (changes frequently)

## Best Practices

### Development Workflow

1. **Develop locally** - Use local Docker builds
2. **Test in K8s** - Deploy to dev namespace
3. **Commit changes** - Push to feature branch
4. **CI builds** - Automatic build on push
5. **Merge to main** - Production image with `latest` tag

### Image Tagging Strategy

- **Local development:** `local`, `local-{timestamp}`
- **Feature branches:** `feature-xyz-{sha}`
- **Main branch:** `latest`, `main-{sha}`
- **Releases:** `v1.0.0`, `v1.0`, `v1`

### Cache Management

- Let Docker handle local cache automatically
- GitHub Actions cache is managed automatically
- Only manually prune when disk space is critical

## Additional Resources

- [Docker BuildKit Documentation](https://docs.docker.com/build/buildkit/)
- [GitHub Actions Cache](https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
- [Go Multi-Stage Builds](https://docs.docker.com/language/golang/build-images/)
