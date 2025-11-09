#!/bin/bash
# Setup GitHub Secrets for Kubernetes Deployment
# This script helps encode the kubeconfig for GitHub Actions secrets

set -e

KUBECONFIG_FILE="${1:-kubeconfig.yaml}"

echo "==================================="
echo "GitHub Secrets Setup for Gethrelay"
echo "==================================="
echo ""

# Check if kubeconfig exists
if [ ! -f "$KUBECONFIG_FILE" ]; then
    echo "ERROR: Kubeconfig file not found: $KUBECONFIG_FILE"
    echo ""
    echo "Usage: $0 [kubeconfig-file]"
    echo "Example: $0 kubeconfig.yaml"
    exit 1
fi

echo "Found kubeconfig: $KUBECONFIG_FILE"
echo ""

# Validate kubeconfig
echo "Validating kubeconfig..."
if ! kubectl cluster-info --kubeconfig="$KUBECONFIG_FILE" &>/dev/null; then
    echo "WARNING: Could not connect to cluster with provided kubeconfig"
    echo "This might be expected if you're on a different network"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "Kubeconfig is valid!"
    echo ""
fi

# Base64 encode kubeconfig
echo "Encoding kubeconfig to base64..."
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    ENCODED=$(cat "$KUBECONFIG_FILE" | base64)
else
    # Linux
    ENCODED=$(cat "$KUBECONFIG_FILE" | base64 -w 0)
fi

echo ""
echo "==================================="
echo "GitHub Secret Configuration"
echo "==================================="
echo ""
echo "1. Go to your GitHub repository"
echo "2. Navigate to: Settings > Secrets and variables > Actions"
echo "3. Click 'New repository secret'"
echo "4. Name: KUBECONFIG"
echo "5. Value: Copy the base64 encoded string below"
echo ""
echo "==================================="
echo "Base64 Encoded Kubeconfig:"
echo "==================================="
echo ""
echo "$ENCODED"
echo ""
echo "==================================="

# Optionally copy to clipboard
if command -v pbcopy &> /dev/null; then
    echo ""
    read -p "Copy to clipboard (macOS)? (y/N) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "$ENCODED" | pbcopy
        echo "Copied to clipboard!"
    fi
elif command -v xclip &> /dev/null; then
    echo ""
    read -p "Copy to clipboard (Linux)? (y/N) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "$ENCODED" | xclip -selection clipboard
        echo "Copied to clipboard!"
    fi
fi

echo ""
echo "==================================="
echo "Verification Steps"
echo "==================================="
echo ""
echo "After adding the secret to GitHub, you can verify with:"
echo ""
echo "# Manually trigger the deployment workflow:"
echo "# 1. Go to Actions tab"
echo "# 2. Select 'Deploy Gethrelay to Kubernetes'"
echo "# 3. Click 'Run workflow'"
echo ""
echo "Or test locally:"
echo "export KUBECONFIG=$KUBECONFIG_FILE"
echo "kubectl apply -f deployment/k8s/namespace.yaml"
echo "kubectl apply -f deployment/k8s/deployments.yaml"
echo "kubectl get pods -n gethrelay"
echo ""
echo "==================================="
echo "Security Reminder"
echo "==================================="
echo ""
echo "- NEVER commit kubeconfig.yaml to git"
echo "- The file is already in .gitignore"
echo "- Rotate kubeconfig credentials regularly"
echo "- Use RBAC to limit permissions"
echo ""
