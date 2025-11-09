#!/bin/bash
#
# Build and Push Gethrelay Docker Image
# Usage: ./scripts/build-and-push-image.sh [OPTIONS]
#

set -euo pipefail

# Configuration
REGISTRY="ghcr.io"
IMAGE_NAME="igor53627/gethrelay"
DOCKERFILE="Dockerfile.gethrelay"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
BUILD_MULTI_ARCH=false
PUSH_IMAGE=false
TAG="latest"
BUILD_ARGS=""

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat << EOF
Build and Push Gethrelay Docker Image

Usage: $0 [OPTIONS]

Options:
    -m, --multi-arch     Build for multiple architectures (amd64, arm64)
    -p, --push          Push image to registry after build
    -t, --tag TAG       Tag for the image (default: latest)
    -a, --also-tag TAG  Additional tag for the image (can be used multiple times)
    --no-cache          Build without using cache
    --login             Login to GitHub Container Registry first
    -h, --help          Show this help message

Examples:
    # Build locally for testing
    $0

    # Build and push latest tag
    $0 -p -t latest

    # Build multi-arch and push with version tag
    $0 -m -p -t v1.0.0 -a latest

    # Build with custom tag
    $0 -t feature-tor-integration

    # Login and push
    $0 --login -p

Environment Variables:
    GITHUB_TOKEN        GitHub Personal Access Token for authentication
    GITHUB_USERNAME     GitHub username for authentication

EOF
}

# Parse arguments
ADDITIONAL_TAGS=()
while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--multi-arch)
            BUILD_MULTI_ARCH=true
            shift
            ;;
        -p|--push)
            PUSH_IMAGE=true
            shift
            ;;
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        -a|--also-tag)
            ADDITIONAL_TAGS+=("$2")
            shift 2
            ;;
        --no-cache)
            BUILD_ARGS="$BUILD_ARGS --no-cache"
            shift
            ;;
        --login)
            if [ -z "${GITHUB_TOKEN:-}" ] || [ -z "${GITHUB_USERNAME:-}" ]; then
                log_error "GITHUB_TOKEN and GITHUB_USERNAME must be set for login"
                exit 1
            fi
            log_info "Logging in to $REGISTRY..."
            echo "$GITHUB_TOKEN" | docker login "$REGISTRY" -u "$GITHUB_USERNAME" --password-stdin
            log_success "Logged in successfully"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Verify Docker is installed
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

# Verify Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
    log_error "Dockerfile not found: $DOCKERFILE"
    exit 1
fi

# Verify Tor config exists
if [ ! -f "deployment/tor/torrc" ]; then
    log_error "Tor configuration not found: deployment/tor/torrc"
    log_info "Creating default Tor configuration..."
    mkdir -p deployment/tor
    cat > deployment/tor/torrc <<'EOF'
SocksPort 0.0.0.0:9050
ControlPort 0.0.0.0:9051
CookieAuthentication 1
DataDirectory /var/lib/tor
EOF
    log_success "Created default Tor configuration"
fi

# Build full image tag
FULL_IMAGE_TAG="$REGISTRY/$IMAGE_NAME:$TAG"

log_info "Building gethrelay Docker image..."
log_info "Image: $FULL_IMAGE_TAG"
log_info "Multi-arch: $BUILD_MULTI_ARCH"
log_info "Push: $PUSH_IMAGE"

if [ "$BUILD_MULTI_ARCH" = true ]; then
    # Multi-architecture build
    log_info "Building for multiple architectures (amd64, arm64)..."

    # Check if buildx is available
    if ! docker buildx version &> /dev/null; then
        log_error "Docker buildx is not available"
        exit 1
    fi

    # Create builder if it doesn't exist
    if ! docker buildx inspect gethrelay-builder &> /dev/null; then
        log_info "Creating buildx builder..."
        docker buildx create --name gethrelay-builder --use
        docker buildx inspect --bootstrap
    else
        log_info "Using existing buildx builder..."
        docker buildx use gethrelay-builder
    fi

    # Build command
    BUILD_CMD="docker buildx build"
    BUILD_CMD="$BUILD_CMD --platform linux/amd64,linux/arm64"
    BUILD_CMD="$BUILD_CMD -f $DOCKERFILE"
    BUILD_CMD="$BUILD_CMD -t $FULL_IMAGE_TAG"

    # Add additional tags
    for EXTRA_TAG in "${ADDITIONAL_TAGS[@]}"; do
        BUILD_CMD="$BUILD_CMD -t $REGISTRY/$IMAGE_NAME:$EXTRA_TAG"
    done

    BUILD_CMD="$BUILD_CMD $BUILD_ARGS"

    if [ "$PUSH_IMAGE" = true ]; then
        BUILD_CMD="$BUILD_CMD --push"
    else
        BUILD_CMD="$BUILD_CMD --load"
        log_warning "Multi-arch builds without --push will only load the native architecture"
    fi

    BUILD_CMD="$BUILD_CMD ."

    log_info "Running: $BUILD_CMD"
    eval $BUILD_CMD

else
    # Single architecture build
    log_info "Building for current architecture..."

    BUILD_CMD="docker build"
    BUILD_CMD="$BUILD_CMD -f $DOCKERFILE"
    BUILD_CMD="$BUILD_CMD -t $FULL_IMAGE_TAG"

    # Add additional tags
    for EXTRA_TAG in "${ADDITIONAL_TAGS[@]}"; do
        BUILD_CMD="$BUILD_CMD -t $REGISTRY/$IMAGE_NAME:$EXTRA_TAG"
    done

    BUILD_CMD="$BUILD_CMD $BUILD_ARGS"
    BUILD_CMD="$BUILD_CMD ."

    log_info "Running: $BUILD_CMD"
    eval $BUILD_CMD
fi

log_success "Build completed successfully"

# Push if requested (for single-arch builds)
if [ "$PUSH_IMAGE" = true ] && [ "$BUILD_MULTI_ARCH" = false ]; then
    log_info "Pushing image to registry..."
    docker push "$FULL_IMAGE_TAG"

    # Push additional tags
    for EXTRA_TAG in "${ADDITIONAL_TAGS[@]}"; do
        EXTRA_FULL_TAG="$REGISTRY/$IMAGE_NAME:$EXTRA_TAG"
        log_info "Pushing additional tag: $EXTRA_FULL_TAG"
        docker push "$EXTRA_FULL_TAG"
    done

    log_success "Image pushed successfully"
fi

# Display image information
log_info "Image details:"
docker images "$REGISTRY/$IMAGE_NAME" --filter "reference=$FULL_IMAGE_TAG"

# Test the image
log_info "Testing the image..."
if docker run --rm "$FULL_IMAGE_TAG" --version 2>/dev/null; then
    log_success "Image test passed"
else
    log_warning "Image test failed or --version not supported"
fi

log_success "All done!"
echo ""
log_info "Next steps:"
if [ "$PUSH_IMAGE" = false ]; then
    echo "  - Test locally: docker run --rm $FULL_IMAGE_TAG --help"
    echo "  - Push to registry: docker push $FULL_IMAGE_TAG"
fi
echo "  - Deploy to Kubernetes: kubectl apply -f deployment/k8s/"
echo "  - Check deployment: kubectl get pods -n gethrelay"
