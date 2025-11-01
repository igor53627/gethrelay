# Local Testing Guide for GitHub Actions Workflow

This guide shows you how to test the GitHub Actions workflow locally before pushing changes.

## Method 1: Using the Test Script (Recommended)

The simplest way to test the workflow setup locally:

```bash
cd cmd/gethrelay
./test-workflow-local.sh
```

This script will:
1. Run unit tests
2. Build the Docker image
3. Set up Hive
4. Configure the gethrelay client
5. Show you how to run Hive tests manually

## Method 2: Using `act` (Full Workflow Simulation)

`act` simulates the entire GitHub Actions environment. However, some actions may need secrets or special setup.

### Basic Usage

```bash
# List available workflows
act -l

# Run a specific workflow
act -W .github/workflows/gethrelay-tests.yml

# Run a specific job
act -j unit-tests

# Run with more verbosity
act -v

# Dry run (see what would happen)
act --dryrun
```

### Limitations

- Some GitHub Actions (like Docker Buildx) may not work perfectly in `act`
- Secrets need to be configured (use `act --secret`)
- Some network-related actions might behave differently

### Running with Secrets (for Telegram notifications)

```bash
act --secret TELEGRAM_BOT_TOKEN="your-token" \
    --secret TELEGRAM_CHAT_ID="your-chat-id"
```

### Testing Specific Jobs

```bash
# Test just the unit tests
act -j unit-tests

# Test just the build
act -j build-image

# Skip certain jobs
act --skip-job hive-tests
```

## Method 3: Manual Step-by-Step Testing

If you want to test specific parts manually:

### 1. Unit Tests
```bash
cd cmd/gethrelay
go test -v -race -cover .
```

### 2. Build Docker Image
```bash
docker build -f cmd/gethrelay/Dockerfile.gethrelay \
    --build-arg GO_VERSION=1.24 \
    -t ethereum/gethrelay:latest \
    .
```

### 3. Set Up Hive Client
```bash
# Clone Hive
git clone --depth=1 https://github.com/ethereum/hive.git /tmp/hive-local

# Build Hive
cd /tmp/hive-local && go build -o hive .

# Set up gethrelay client (see test-workflow-local.sh for details)
# Then test:
cd /tmp/hive-local
./hive --sim=ethereum/rpc --client=gethrelay:local --loglevel=5
```

## Troubleshooting

### `act` can't find Docker
Make sure Docker is running:
```bash
docker info
```

### Client discovery fails
Check that:
- The client directory exists in `/tmp/hive-local-test/clients/gethrelay/`
- `hive.yaml` is present and valid
- `Dockerfile.local` is present
- All scripts are adapted for `gethrelay` (not `geth`)

### Docker build fails
Ensure:
- You're in the repository root
- All required build args are provided
- Docker has enough resources allocated

## Quick Test Checklist

- [ ] Unit tests pass
- [ ] Docker image builds successfully
- [ ] Hive can discover the client (`gethrelay:local`)
- [ ] Client scripts work (gethrelay.sh, enode.sh)
- [ ] Dockerfile.local builds correctly

## Notes

- The local test script uses `/tmp/hive-local-test` to avoid conflicts
- `act` will use its own Docker containers (separate from your local Docker)
- Some GitHub Actions features may not work identically locally vs. CI
- Always test the full workflow in CI after local testing

