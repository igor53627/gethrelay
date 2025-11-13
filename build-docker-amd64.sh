#!/bin/bash
# Build Docker image explicitly for linux/amd64 platform

set -e

COMMIT_SHA="7270745c6e80b1efb8c772a1f34dee7c01a4b4b7"
IMAGE_TAG="ghcr.io/igor53627/gethrelay:tor-enr-integration-7270745c6"
LATEST_TAG="ghcr.io/igor53627/gethrelay:tor-enr-integration-latest"

echo "Building Docker image for linux/amd64..."
echo "Commit: $COMMIT_SHA"
echo "Tags: $IMAGE_TAG, $LATEST_TAG"
echo ""

docker buildx build \
  --platform linux/amd64 \
  --build-arg COMMIT="$COMMIT_SHA" \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILDNUM=1 \
  -t "$IMAGE_TAG" \
  -t "$LATEST_TAG" \
  --load \
  .

echo ""
echo "Build complete! Verifying architecture..."
docker inspect "$IMAGE_TAG" | grep -A 2 "Architecture"
