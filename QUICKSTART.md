# Gethrelay Docker & Kubernetes Quick Start

## 1. Build and Push Docker Image

### Option A: Automated (Recommended)
```bash
gh workflow run build-gethrelay-image.yaml
gh run watch
```

### Option B: Manual Script
```bash
export GITHUB_USERNAME="igor53627"
export GITHUB_TOKEN="your_github_pat"
./scripts/build-and-push-image.sh --login -m -p
```

### Option C: Docker CLI
```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u igor53627 --password-stdin
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile.gethrelay \
  -t ghcr.io/igor53627/gethrelay:latest \
  --push .
```

## 2. Make Image Public (One-time)

1. Visit: https://github.com/users/igor53627/packages/container/gethrelay/settings
2. Change visibility to "Public"

## 3. Deploy to Kubernetes

```bash
kubectl apply -f deployment/k8s/namespace.yaml
kubectl apply -f deployment/k8s/deployments.yaml
kubectl apply -f deployment/k8s/services.yaml
```

## 4. Verify Deployment

```bash
kubectl get pods -n gethrelay -w
kubectl logs -n gethrelay -l app=gethrelay --tail=50
```

## Troubleshooting

### ImagePullBackOff?
- Make image public (see step 2)
- Or create imagePullSecret: See [DOCKER_BUILD.md](deployment/DOCKER_BUILD.md#kubernetes-deployment)

### Build fails?
- Check Dockerfile exists: `Dockerfile.gethrelay`
- Check Tor config: `deployment/tor/torrc`

## Documentation

- **Complete setup:** [DOCKER_DEPLOYMENT_SETUP.md](DOCKER_DEPLOYMENT_SETUP.md)
- **Build details:** [deployment/DOCKER_BUILD.md](deployment/DOCKER_BUILD.md)
- **Deployment guide:** [deployment/README.md](deployment/README.md)
- **Build script help:** `./scripts/build-and-push-image.sh --help`
