# Scripts Directory

Utility scripts for development and deployment.

## Docker Build Scripts

### docker-build-local.sh

Local Docker build script for rapid iteration during development.

**Usage:**
```bash
# Quick build with auto-generated tag
./scripts/docker-build-local.sh

# Custom tag
TAG=feature-xyz ./scripts/docker-build-local.sh

# Custom registry
REGISTRY=my-registry.io TAG=v1.0.0 ./scripts/docker-build-local.sh
```

**Features:**
- Builds linux/amd64 image (matches K8s deployment)
- Auto-generates timestamp-based tags
- Interactive push prompt
- Provides kubectl deployment commands

**Environment Variables:**
- `REGISTRY` - Docker registry (default: ghcr.io)
- `IMAGE_NAME` - Image name (default: igor53627/gethrelay)
- `TAG` - Image tag (default: local-{timestamp})
- `DOCKERFILE` - Dockerfile path (default: Dockerfile.gethrelay)

**When to use:**
- Local development and testing
- Quick iterations without waiting for CI
- Testing Dockerfile changes
- Debugging build issues

For production builds, use GitHub Actions CI/CD workflow.

See [docs/DOCKER_BUILD.md](../docs/DOCKER_BUILD.md) for comprehensive documentation.
