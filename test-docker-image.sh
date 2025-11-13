#!/bin/bash
# Script to test the Docker image locally before pushing

set -e

COMMIT_SHA="7270745c6"
BRANCH="tor-enr-integration"
IMAGE_TAG="ghcr.io/igor53627/gethrelay:${BRANCH}-${COMMIT_SHA}"

echo "Testing Docker image: ${IMAGE_TAG}"
echo ""

echo "1. Checking if image exists..."
docker images "${IMAGE_TAG}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
echo ""

echo "2. Checking image metadata and labels..."
docker inspect "${IMAGE_TAG}" --format '{{json .Config.Labels}}' | jq '.'
echo ""

echo "3. Checking if gethrelay binary is present..."
docker run --rm --entrypoint /bin/sh "${IMAGE_TAG}" -c "ls -lh /usr/local/bin/gethrelay"
echo ""

echo "4. Checking Tor installation..."
docker run --rm --entrypoint /bin/sh "${IMAGE_TAG}" -c "which tor && tor --version | head -1"
echo ""

echo "5. Checking Tor configuration file..."
docker run --rm --entrypoint /bin/sh "${IMAGE_TAG}" -c "ls -la /etc/tor/ && cat /etc/tor/torrc"
echo ""

echo "6. Verifying user configuration..."
docker run --rm --entrypoint /bin/sh "${IMAGE_TAG}" -c "id && whoami"
echo ""

echo "All checks passed! Image is ready to be pushed."
