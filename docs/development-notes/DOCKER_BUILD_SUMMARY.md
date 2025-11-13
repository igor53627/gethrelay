# Docker Build Summary - Tor ENR Integration

## Build Information

**Date**: 2025-11-12
**Branch**: tor-enr-integration
**Commit SHA (short)**: 7270745c6
**Commit SHA (full)**: 7270745c6e80b1efb8c772a1f34dee7c01a4b4b7

## Docker Image Details

**Image Tag**: `ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6`
**Also Tagged**: `ghcr.io/igor53627/gethrelay:tor-enr-integration-latest`
**Size**: 84.1 MB
**Architecture**: linux/amd64
**Base Image**: Alpine Linux (latest)

## Build Status

✅ **BUILD SUCCESSFUL** - Image built locally and ready to push

### What's Included

1. **Go Ethereum (gethrelay)**: Custom relay binary with Tor integration
2. **Tor Package**: Tor daemon installed from Alpine packages
3. **Tor Configuration**: Custom torrc file located at `/etc/tor/torrc`
4. **TorDialer Code**: All Tor dialer integration code compiled into the binary
5. **Security**: Non-root user (gethrelay:gethrelay, uid:gid 1000:1000)

### Dockerfile Used

`Dockerfile.gethrelay` (root level) - includes:
- Multi-stage build for optimal size
- Build dependencies: gcc, musl-dev, linux-headers, git, make
- Runtime: Alpine + ca-certificates + tor
- Proper user permissions for tor and gethrelay

## Verification

The image has been built and loaded into local Docker. You can verify it with:

```bash
# Check image exists
docker images ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6

# Run automated tests
./test-docker-image.sh
```

## Pushing to GitHub Container Registry

### Prerequisites

You need a GitHub Personal Access Token (PAT) with `write:packages` permission.

### Create a PAT

1. Go to: https://github.com/settings/tokens/new
2. Token name: "GHCR Push Token" (or your preference)
3. Expiration: Select based on your security requirements
4. Scopes: Select both:
   - ✅ `write:packages` (Upload packages to GitHub Package Registry)
   - ✅ `read:packages` (Download packages from GitHub Package Registry)
5. Click "Generate token"
6. **Copy the token immediately** (you won't be able to see it again)

### Push the Image

```bash
# Method 1: Using the helper script (recommended)
./push-docker-image.sh YOUR_GITHUB_PAT

# Method 2: Manual push
echo "YOUR_GITHUB_PAT" | docker login ghcr.io -u igor53627 --password-stdin
docker push ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6
docker tag ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6 ghcr.io/igor53627/gethrelay:tor-enr-integration-latest
docker push ghcr.io/igor53627/gethrelay:tor-enr-integration-latest
```

## Deployment to VM

### Update docker-compose.yml

Use this exact image tag in your docker-compose.yml file on the VM (45.77.155.64):

```yaml
services:
  gethrelay:
    image: ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6
    # ... rest of your configuration
```

### Pull on VM

After pushing, SSH to your VM and pull the image:

```bash
ssh root@45.77.155.64
docker pull ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6
docker-compose up -d
```

## Build Features

### Labels and Metadata

The image includes OCI-compliant labels:
- `org.opencontainers.image.source`: https://github.com/igor53627/gethrelay
- `org.opencontainers.image.revision`: 7270745c6e80b1efb8c772a1f34dee7c01a4b4b7
- `org.opencontainers.image.version`: 7270745c6
- `org.opencontainers.image.title`: gethrelay
- `org.opencontainers.image.description`: Go Ethereum relay with Tor integration
- `org.opencontainers.image.created`: 2025-11-12T13:31:20.834136324Z

### Tor Configuration

The included `/etc/tor/torrc` file has:
- SOCKS5 proxy on port 9050
- Control port on 9051
- Cookie authentication enabled
- Optimized circuit settings

## Troubleshooting

### If push fails

1. Verify PAT has `write:packages` scope
2. Ensure you're logged in: `docker login ghcr.io -u igor53627`
3. Check token hasn't expired
4. Verify repository permissions on GitHub

### If image is missing locally

Re-build with:
```bash
docker buildx build \
  --platform linux/amd64 \
  --file Dockerfile.gethrelay \
  --tag ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6 \
  --load \
  .
```

## Multi-Platform Support

Currently built for: linux/amd64

To build for multiple platforms (requires push to registry):
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --file Dockerfile.gethrelay \
  --tag ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6 \
  --push \
  .
```

## Next Steps

1. ✅ Docker image built successfully
2. ⏳ Create GitHub PAT with write:packages permission
3. ⏳ Run `./push-docker-image.sh YOUR_PAT` to push image
4. ⏳ Update docker-compose.yml on VM
5. ⏳ Deploy to VM at 45.77.155.64

---

**Final Image Tag for Deployment:**
```
ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6
```
