#!/bin/bash
# Script to push the Docker image to GitHub Container Registry
# Usage: ./push-docker-image.sh YOUR_GITHUB_PAT

set -e

GITHUB_TOKEN="${1:-}"
COMMIT_SHA="7270745c6"
BRANCH="tor-enr-integration"
IMAGE_TAG="ghcr.io/igor53627/gethrelay:${BRANCH}-${COMMIT_SHA}"
IMAGE_TAG_LATEST="ghcr.io/igor53627/gethrelay:${BRANCH}-latest"

if [ -z "$GITHUB_TOKEN" ]; then
    echo "Error: GitHub Personal Access Token required"
    echo ""
    echo "Usage: ./push-docker-image.sh YOUR_GITHUB_PAT"
    echo ""
    echo "To create a PAT with write:packages permission:"
    echo "1. Go to https://github.com/settings/tokens/new"
    echo "2. Give it a name like 'GHCR Push Token'"
    echo "3. Select these scopes:"
    echo "   - write:packages"
    echo "   - read:packages"
    echo "4. Generate token and copy it"
    echo "5. Run: ./push-docker-image.sh YOUR_TOKEN_HERE"
    echo ""
    exit 1
fi

echo "Authenticating with GitHub Container Registry..."
echo "$GITHUB_TOKEN" | docker login ghcr.io -u igor53627 --password-stdin

echo ""
echo "Pushing image: ${IMAGE_TAG}"
docker push "${IMAGE_TAG}"

echo ""
echo "Tagging and pushing latest..."
docker tag "${IMAGE_TAG}" "${IMAGE_TAG_LATEST}"
docker push "${IMAGE_TAG_LATEST}"

echo ""
echo "Success! Images pushed:"
echo "  - ${IMAGE_TAG}"
echo "  - ${IMAGE_TAG_LATEST}"
echo ""
echo "Update your docker-compose.yml to use:"
echo "  image: ${IMAGE_TAG}"
