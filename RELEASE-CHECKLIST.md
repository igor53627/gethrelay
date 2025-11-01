# Release Preparation Checklist

## Pre-Release Steps

### 1. Verify Code Quality
- [ ] All tests pass: `make gethrelay-test`
- [ ] Code builds successfully: `make gethrelay`
- [ ] Docker image builds: `make gethrelay-docker`
- [ ] No linter errors
- [ ] Documentation is complete

### 2. Review Changes
- [ ] Review all modified files
- [ ] Verify removed commands are intentional
- [ ] Check README.md reflects gethrelay accurately
- [ ] Verify license headers

### 3. Git Preparation
- [ ] Run `./prepare-release.sh` to prepare commit
- [ ] Review commit message
- [ ] Verify all intended changes are staged

### 4. Repository Setup
- [ ] Create new repository on GitHub/GitLab/etc.
- [ ] Update git remote: `git remote set-url origin <your-repo-url>`
- [ ] Verify remote: `git remote -v`

### 5. Initial Commit
- [ ] Create initial commit with prepared message
- [ ] Tag initial release (optional): `git tag v0.1.0`

### 6. Push to Repository
- [ ] Push initial commit: `git push -u origin <branch>`
- [ ] Push tags (if created): `git push --tags`
- [ ] Set default branch (e.g., `main` or `master`)

### 7. Repository Configuration
- [ ] Add repository description
- [ ] Set repository topics/tags
- [ ] Configure GitHub Actions (if using)
- [ ] Add LICENSE file (if needed)
- [ ] Configure branch protection rules

### 8. Post-Release
- [ ] Verify GitHub Actions run successfully
- [ ] Test Docker image pull
- [ ] Update documentation links
- [ ] Announce release (if applicable)

## Quick Commands

```bash
# Prepare release
./prepare-release.sh

# Or manually:
git add -A
git commit -F .gitmessage

# Update remote
git remote set-url origin https://github.com/username/gethrelay.git

# Push
git push -u origin main
```

## Repository Recommendations

### Repository Name
- `gethrelay` or `ethereum-relay` or similar

### Description
- "Lightweight Ethereum P2P relay node with JSON-RPC proxy"

### Topics/Tags
- `ethereum`
- `p2p`
- `relay`
- `go`
- `blockchain`
- `rpc-proxy`

### License
Ensure LICENSE/COPYING files are included. Default is:
- Library: LGPL-3.0
- Binaries: GPL-3.0

