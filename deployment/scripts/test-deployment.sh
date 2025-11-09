#!/bin/bash
# Test Deployment Script for Gethrelay Kubernetes Deployment
# Validates deployment configuration before applying to cluster

set -e

KUBECONFIG_FILE="${1:-kubeconfig.yaml}"

echo "========================================"
echo "Gethrelay Deployment Validation"
echo "========================================"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warning() {
    echo -e "${YELLOW}!${NC} $1"
}

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v kubectl &> /dev/null; then
    error "kubectl not found. Please install kubectl."
    exit 1
fi
success "kubectl found"

if ! command -v docker &> /dev/null; then
    warning "docker not found. Image building will not work locally."
else
    success "docker found"
fi

echo ""

# Check kubeconfig
if [ ! -f "$KUBECONFIG_FILE" ]; then
    error "Kubeconfig not found: $KUBECONFIG_FILE"
    exit 1
fi
success "Kubeconfig found: $KUBECONFIG_FILE"

# Check gitignore
if grep -q "kubeconfig.yaml" .gitignore; then
    success "kubeconfig.yaml is in .gitignore"
else
    error "kubeconfig.yaml NOT in .gitignore - SECURITY RISK!"
    exit 1
fi

echo ""

# Validate YAML files
echo "Validating Kubernetes manifests..."

for file in deployment/k8s/*.yaml; do
    if kubectl apply --dry-run=client -f "$file" --kubeconfig="$KUBECONFIG_FILE" &>/dev/null; then
        success "Valid: $(basename $file)"
    else
        error "Invalid: $(basename $file)"
        kubectl apply --dry-run=client -f "$file" --kubeconfig="$KUBECONFIG_FILE"
        exit 1
    fi
done

echo ""

# Check deployment distribution
echo "Checking deployment distribution..."

# Count deployments by kind (each deployment has exactly one metadata.name)
DEFAULT_COUNT=$(grep -c "name: gethrelay-default-" deployment/k8s/deployments.yaml || true)
PREFER_TOR_COUNT=$(grep -c "name: gethrelay-prefer-tor-" deployment/k8s/deployments.yaml || true)
ONLY_ONION_COUNT=$(grep -c "name: gethrelay-only-onion-" deployment/k8s/deployments.yaml || true)
TOTAL=$((DEFAULT_COUNT + PREFER_TOR_COUNT + ONLY_ONION_COUNT))

echo "  Default mode: $DEFAULT_COUNT instances"
echo "  Prefer-Tor mode: $PREFER_TOR_COUNT instances"
echo "  Tor-Only mode: $ONLY_ONION_COUNT instances"
echo "  Total: $TOTAL instances"

if [ "$TOTAL" -eq 10 ]; then
    success "10 instances configured correctly"
else
    error "Expected 10 instances, found $TOTAL"
    exit 1
fi

echo ""

# Test cluster connectivity
echo "Testing cluster connectivity..."

export KUBECONFIG="$KUBECONFIG_FILE"

if kubectl cluster-info &>/dev/null; then
    success "Connected to cluster"
    kubectl cluster-info
else
    error "Cannot connect to cluster"
    exit 1
fi

echo ""

# Check namespace
echo "Checking namespace..."

if kubectl get namespace gethrelay &>/dev/null; then
    warning "Namespace 'gethrelay' already exists"
else
    echo "  Namespace 'gethrelay' does not exist (will be created on deploy)"
fi

echo ""

# Validate Dockerfile
echo "Validating Dockerfile..."

if [ -f "Dockerfile.gethrelay" ]; then
    success "Dockerfile.gethrelay found"

    # Check for security issues
    if grep -q "USER gethrelay" Dockerfile.gethrelay; then
        success "Dockerfile uses non-root user"
    else
        warning "Dockerfile may not use non-root user"
    fi

    if grep -q "tor" Dockerfile.gethrelay; then
        success "Dockerfile includes Tor"
    else
        error "Dockerfile missing Tor installation"
        exit 1
    fi
else
    error "Dockerfile.gethrelay not found"
    exit 1
fi

echo ""

# Check GitHub Actions workflow
echo "Validating GitHub Actions workflow..."

if [ -f ".github/workflows/deploy-gethrelay.yaml" ]; then
    success "GitHub Actions workflow found"

    if grep -q "KUBECONFIG" .github/workflows/deploy-gethrelay.yaml; then
        success "Workflow uses KUBECONFIG secret"
    else
        error "Workflow missing KUBECONFIG secret reference"
        exit 1
    fi
else
    error "GitHub Actions workflow not found"
    exit 1
fi

echo ""

# Test deployment (dry-run)
echo "Testing deployment (dry-run)..."

# Create namespace for server-side dry-run validation
NAMESPACE_EXISTS=false
if kubectl get namespace gethrelay &>/dev/null; then
    NAMESPACE_EXISTS=true
    echo "  Using existing namespace for validation..."
else
    echo "  Creating temporary namespace for validation..."
    kubectl apply -f deployment/k8s/namespace.yaml &>/dev/null
fi

echo "  Creating deployments..."
if kubectl apply -f deployment/k8s/deployments.yaml --dry-run=server &>/dev/null; then
    success "Deployments configuration valid"
else
    error "Deployments configuration invalid"
    kubectl apply -f deployment/k8s/deployments.yaml --dry-run=server
    # Cleanup namespace if we created it
    if [ "$NAMESPACE_EXISTS" = false ]; then
        kubectl delete namespace gethrelay &>/dev/null || true
    fi
    exit 1
fi

echo "  Creating services..."
if kubectl apply -f deployment/k8s/services.yaml --dry-run=server &>/dev/null; then
    success "Services configuration valid"
else
    error "Services configuration invalid"
    # Cleanup namespace if we created it
    if [ "$NAMESPACE_EXISTS" = false ]; then
        kubectl delete namespace gethrelay &>/dev/null || true
    fi
    exit 1
fi

# Cleanup temporary namespace if we created it
if [ "$NAMESPACE_EXISTS" = false ]; then
    echo "  Cleaning up temporary namespace..."
    kubectl delete namespace gethrelay &>/dev/null || true
fi

echo ""

# Summary
echo "========================================"
echo "Validation Summary"
echo "========================================"
echo ""
success "All validation checks passed!"
echo ""
echo "Deployment configuration:"
echo "  - 10 gethrelay instances"
echo "  - 3 Default mode (Tor with clearnet fallback)"
echo "  - 4 Prefer-Tor mode"
echo "  - 3 Tor-Only mode"
echo ""
echo "Next steps:"
echo "  1. Setup GitHub secret: ./deployment/scripts/setup-github-secrets.sh"
echo "  2. Test local deployment: kubectl apply -f deployment/k8s/"
echo "  3. Monitor deployment: kubectl get pods -n gethrelay -w"
echo ""
echo "Or trigger GitHub Actions deployment:"
echo "  1. Go to Actions tab in GitHub"
echo "  2. Select 'Deploy Gethrelay to Kubernetes'"
echo "  3. Click 'Run workflow'"
echo ""
