#!/bin/bash
# Local Docker build script for gethrelay
# Use this for rapid iteration and testing before CI/CD builds

set -e

# Configuration
REGISTRY="${REGISTRY:-ghcr.io}"
IMAGE_NAME="${IMAGE_NAME:-igor53627/gethrelay}"
TAG="${TAG:-local-$(date +%s)}"
DOCKERFILE="${DOCKERFILE:-Dockerfile.gethrelay}"

# Color output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Local Docker Build for gethrelay ===${NC}"
echo -e "${BLUE}Registry: ${NC}${REGISTRY}"
echo -e "${BLUE}Image:    ${NC}${IMAGE_NAME}"
echo -e "${BLUE}Tag:      ${NC}${TAG}"
echo -e "${BLUE}File:     ${NC}${DOCKERFILE}"
echo ""

# Check if docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${YELLOW}Error: Docker is not running${NC}"
    exit 1
fi

# Build the image
echo -e "${GREEN}Building image...${NC}"
docker build \
    --file "${DOCKERFILE}" \
    --tag "${REGISTRY}/${IMAGE_NAME}:${TAG}" \
    --tag "${REGISTRY}/${IMAGE_NAME}:local" \
    --platform linux/amd64 \
    --build-arg BUILDKIT_INLINE_CACHE=1 \
    .

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo -e "${BLUE}Image tags:${NC}"
echo "  ${REGISTRY}/${IMAGE_NAME}:${TAG}"
echo "  ${REGISTRY}/${IMAGE_NAME}:local"
echo ""

# Ask if user wants to push
read -p "Push to registry? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${GREEN}Pushing images to registry...${NC}"
    docker push "${REGISTRY}/${IMAGE_NAME}:${TAG}"
    docker push "${REGISTRY}/${IMAGE_NAME}:local"
    echo -e "${GREEN}Push complete!${NC}"
else
    echo -e "${YELLOW}Skipping push. To push later, run:${NC}"
    echo "  docker push ${REGISTRY}/${IMAGE_NAME}:${TAG}"
    echo "  docker push ${REGISTRY}/${IMAGE_NAME}:local"
fi

echo ""
echo -e "${GREEN}To test the image locally:${NC}"
echo "  docker run --rm ${REGISTRY}/${IMAGE_NAME}:${TAG} --version"
echo ""
echo -e "${GREEN}To use in Kubernetes:${NC}"
echo "  kubectl set image deployment/gethrelay-default-1 gethrelay=${REGISTRY}/${IMAGE_NAME}:${TAG} -n gethrelay"
