# Docker Build Caching Optimization

This document explains the caching strategies implemented to reduce Docker build times in GitHub Actions.

## Optimizations Implemented

### 1. **BuildKit Cache Mounts in Dockerfile**

The Dockerfile now uses BuildKit cache mounts to persist Go module downloads and build artifacts across builds:

- **Go Module Cache** (`/go/pkg/mod`): Caches downloaded Go dependencies
- **Go Build Cache** (`/root/.cache/go-build`): Caches compiled packages

**Impact**: Reduces `go mod download` from ~30-60s to ~5-10s on cache hits.

### 2. **Granular GitHub Actions Cache Keys**

The workflow generates more specific cache keys based on file content hashes:

- Separate cache keys for:
  - `go.mod` and `go.sum` (dependency layer)
  - `Dockerfile.gethrelay` (builder layer)
  - General fallback caches

**Impact**: Only invalidates specific layers when relevant files change, not the entire build.

### 3. **Hive Binary Caching**

The Hive binary (built from source) is now cached using GitHub Actions cache:

- Cache key based on workflow file hash
- Falls back to building from source if cache miss
- Saves ~30-60 seconds per build

### 4. **Multi-Stage Cache Scopes**

Docker layer caching uses multiple scopes with fallback:

- **Specific caches**: Hash-based scopes for exact matches
- **General caches**: Fallback scopes for partial matches
- **Mode**: `max` mode to preserve all intermediate layers

**Impact**: Better cache hit rates even when dependency versions change slightly.

## Cache Strategy Breakdown

### Stage 1: Base Layer (rarely changes)
- Go base image with build tools
- Cached by Docker layer cache
- Invalidated only when `GO_VERSION` changes

### Stage 2: Dependencies Layer (changes when go.mod/go.sum changes)
- Uses BuildKit cache mount for `/go/pkg/mod`
- GitHub Actions cache key based on `go.mod` + `go.sum` hash
- Cache hits: ~5-10s vs ~30-60s for fresh downloads

### Stage 3: Builder Layer (changes when source code changes)
- Uses BuildKit cache mount for build cache
- GitHub Actions cache key based on Dockerfile hash
- Only rebuilds when source files or Dockerfile change

### Stage 4: Runtime Layer (minimal, fast)
- Alpine base with minimal packages
- Fast to rebuild (usually <5s)

## Expected Build Times

### First Build (cold cache)
- Base layer: ~30s
- Dependencies: ~45s
- Build: ~60-90s
- Runtime: ~5s
- **Total: ~2-3 minutes**

### Subsequent Builds (warm cache, no changes)
- Base layer: ~2s (pulled from cache)
- Dependencies: ~5s (cache mount hit)
- Build: ~10s (cache mount hit)
- Runtime: ~2s
- **Total: ~20-30 seconds**

### Builds with go.mod changes only
- Base layer: ~2s (cached)
- Dependencies: ~45s (fresh download)
- Build: ~10s (cache mount hit)
- Runtime: ~2s
- **Total: ~1 minute**

### Builds with source code changes only
- Base layer: ~2s (cached)
- Dependencies: ~5s (cache mount hit)
- Build: ~60-90s (compile changed code)
- Runtime: ~2s
- **Total: ~1.5 minutes**

## Monitoring Cache Effectiveness

GitHub Actions shows cache hit/miss in the build logs:

- Look for messages like: `CACHED [stage 1/4] FROM golang:...`
- Cache statistics appear in the build output
- Check the "Cache" section in GitHub Actions UI

## Cache Storage Limits

GitHub Actions cache:
- **Maximum size**: 10 GB per repository
- **Eviction**: Least recently used (LRU)
- **Retention**: 7 days of no access

If cache is evicted, builds will fall back to fresh downloads but still benefit from BuildKit cache mounts within a single build.

## Troubleshooting

### Cache not working?

1. **Check BuildKit is enabled**: Ensure `DOCKER_BUILDKIT: 1` in env
2. **Verify cache mounts**: Look for `--mount=type=cache` in Dockerfile
3. **Check cache keys**: Verify hash generation in workflow logs
4. **Clear cache**: Delete specific cache entries in GitHub Actions settings if needed

### Cache too large?

If cache is approaching the 10GB limit:
- Reduce cache retention periods
- Use more specific cache keys
- Consider using Docker registry for base images instead of caching them

## Future Improvements

Potential additional optimizations:

1. **Registry Cache**: Push intermediate images to container registry instead of just GHA cache
2. **Pre-built Base Images**: Use custom base images with dependencies pre-installed
3. **Parallel Builds**: Build multiple targets in parallel if needed
4. **Conditional Builds**: Skip image build if only documentation changed

