# Docker Compose Deployment - File Index

Complete reference guide to all files in the Docker Compose deployment.

## Quick Navigation

- **New User?** Start with [QUICKSTART.md](QUICKSTART.md)
- **Need Details?** Read [README.md](README.md)
- **Production Deploy?** See [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md)
- **Implementation Info?** Check [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)

## Core Deployment Files

### docker-compose.yml
**Purpose**: Production Docker Compose configuration
**Contains**:
- Shared Tor daemon (1 instance)
- 3 gethrelay nodes with DHT discovery
- 3 peer manager sidecars
- Volume definitions
- Network configuration
- Health checks and restart policies

**Usage**:
```bash
docker-compose up -d
```

### docker-compose.dev.yml
**Purpose**: Development environment overrides
**Contains**:
- Exposed ports for external access
- Increased logging verbosity
- pprof profiling enabled
- Development API modules
- More frequent peer checks

**Usage**:
```bash
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

### .env.example
**Purpose**: Environment variable template
**Contains**:
- Tor configuration (ports, logging)
- Gethrelay settings (verbosity, peers, network)
- Peer manager configuration
- RPC settings
- Development options

**Usage**:
```bash
cp .env.example .env
nano .env  # Edit as needed
```

## Scripts Directory

### scripts/peer-manager.sh
**Purpose**: Sidecar for automatic peer discovery and promotion
**Features**:
- Monitors connected peers via admin_peers RPC
- Detects .onion peers from DHT discovery
- Promotes peers via admin_addTrustedPeer
- Tracks seen peers to avoid duplicates
- Configurable check interval (default: 30s)
- Comprehensive error handling

**How It Works**:
1. Waits for gethrelay RPC to be ready
2. Polls admin_peers every 30 seconds
3. Extracts .onion peers
4. Promotes new peers to trusted status
5. Logs all actions

### scripts/healthcheck.sh
**Purpose**: Container health verification
**Checks**:
- P2P port (30303) listening
- RPC endpoint (8545) accessible
- .onion address generated
- Peer count query

**Usage**:
```bash
# Manual health check
docker-compose exec gethrelay-1 /scripts/healthcheck.sh
```

### scripts/validate-deployment.sh
**Purpose**: Automated deployment validation
**Tests** (27 total):
1. Docker Compose installation
2. All containers running (7 expected)
3. Tor container health
4. Tor SOCKS5 proxy accessible
5. Tor control port accessible
6-8. Gethrelay node health (3 nodes)
9-11. P2P port listening (3 nodes)
12-14. RPC endpoint accessible (3 nodes)
15-17. .onion address generation (3 nodes)
18-20. Peer managers running (3 sidecars)
21-23. Peer connectivity (3 nodes)
24. Docker volumes exist
25. Docker network exists
26. Error log analysis
27. Trusted peers validation

**Usage**:
```bash
./scripts/validate-deployment.sh
```

**Output**:
- Green checkmarks (✓) for passed tests
- Red X marks (✗) for failed tests
- Summary with pass/fail counts

### scripts/monitor.sh
**Purpose**: Real-time monitoring dashboard
**Displays**:
- Tor service status
- Node status and health (3 nodes)
- Current peer counts (total + trusted)
- .onion addresses
- Peer manager activity
- Resource usage (CPU, memory, network)
- Discovery summary
- Cluster health status

**Usage**:
```bash
./scripts/monitor.sh
# Press Ctrl+C to exit
```

**Refresh Rate**: Every 5 seconds

## Documentation Files

### README.md
**Purpose**: Comprehensive documentation
**Sections**:
- Architecture overview
- Quick start guide
- Configuration instructions
- Monitoring and debugging
- Management operations
- Troubleshooting guide (detailed)
- Security considerations
- Performance tuning
- Scaling instructions
- Backup and recovery

**Length**: ~500 lines
**Target Audience**: All users

### QUICKSTART.md
**Purpose**: Get running in 5 minutes
**Sections**:
- One-line start command
- Verification steps (2-3 minutes)
- Watch discovery in action
- Verify peer connections
- Get .onion addresses
- Common commands
- Troubleshooting basics
- Success criteria

**Length**: ~150 lines
**Target Audience**: New users

### DEPLOYMENT_GUIDE.md
**Purpose**: Production deployment procedures
**Sections**:
- Architecture deep dive
- Prerequisites (system + software)
- Installation steps
- Configuration options
- Deployment strategies
- Validation procedures
- Monitoring setup
- Maintenance operations
- Troubleshooting
- Security checklist

**Length**: ~400 lines
**Target Audience**: DevOps/SRE teams

### IMPLEMENTATION_SUMMARY.md
**Purpose**: Technical implementation details
**Sections**:
- Executive summary
- Deliverables list
- Technical implementation details
- Key design decisions
- Testing and validation
- Production readiness checklist
- Comparison with K8s
- File structure
- Usage examples
- Known limitations
- Future enhancements

**Length**: ~300 lines
**Target Audience**: Developers/architects

### INDEX.md
**Purpose**: Navigation and file reference (this file)
**Sections**:
- Quick navigation
- Core deployment files
- Scripts directory
- Documentation files
- File relationships
- Common workflows

## File Relationships

```
docker-compose.yml
├── Uses: .env (optional)
├── Builds: ../../Dockerfile
├── Volumes: scripts/ (read-only)
└── References: docker-compose.dev.yml (optional)

docker-compose.dev.yml
└── Extends: docker-compose.yml

peer-manager.sh
├── Called by: peer-manager-X containers
├── Accesses: gethrelay RPC via localhost
└── Logs: Docker logs

healthcheck.sh
├── Called by: healthcheck in docker-compose.yml
└── Tests: P2P, RPC, .onion generation

validate-deployment.sh
├── Uses: docker-compose CLI
└── Tests: All components

monitor.sh
├── Uses: docker-compose CLI
└── Displays: Real-time stats
```

## Common Workflows

### Initial Deployment
1. Read [QUICKSTART.md](QUICKSTART.md)
2. Run: `docker-compose up -d`
3. Validate: `./scripts/validate-deployment.sh`
4. Monitor: `./scripts/monitor.sh`

### Development
1. Read [docker-compose.dev.yml](docker-compose.dev.yml)
2. Run: `docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d`
3. Access: RPC on http://localhost:8545
4. Profile: pprof on http://localhost:6060

### Troubleshooting
1. Check [README.md#troubleshooting](README.md#troubleshooting)
2. View logs: `docker-compose logs -f`
3. Run validation: `./scripts/validate-deployment.sh`
4. Check health: `docker-compose ps`

### Production Deployment
1. Read [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md)
2. Review security: [DEPLOYMENT_GUIDE.md#security](DEPLOYMENT_GUIDE.md#security)
3. Configure: Copy and edit `.env.example`
4. Deploy: Follow staged deployment procedure
5. Validate: Run automated tests
6. Monitor: Set up monitoring dashboard

### Maintenance
1. Update: `docker-compose pull && docker-compose up -d`
2. Backup: See [README.md#backup-and-recovery](README.md#backup-and-recovery)
3. Scale: Add nodes following [README.md#scaling](README.md#scaling)
4. Monitor: Use `./scripts/monitor.sh`

## File Sizes

```
docker-compose.yml          ~6 KB
docker-compose.dev.yml      ~2 KB
.env.example                ~2 KB
README.md                   ~25 KB
QUICKSTART.md               ~3 KB
DEPLOYMENT_GUIDE.md         ~20 KB
IMPLEMENTATION_SUMMARY.md   ~15 KB
INDEX.md                    ~8 KB (this file)

scripts/peer-manager.sh         ~4 KB
scripts/healthcheck.sh          ~4 KB
scripts/validate-deployment.sh  ~8 KB
scripts/monitor.sh              ~7 KB
```

**Total**: ~104 KB of deployment configuration and documentation

## Dependencies

### External Dependencies
- Docker 20.10+
- Docker Compose 1.29+
- Linux/macOS/Windows with WSL2

### Container Dependencies
- `alpine/tor:latest` (Tor daemon)
- `alpine:latest` (peer manager sidecars)
- Custom gethrelay image (built from ../../Dockerfile)

### Tool Dependencies (in containers)
- `netcat-openbsd` (health checks)
- `wget` (RPC calls)
- `curl` (peer manager)
- `jq` (JSON parsing in peer manager)

## Version History

### v1.0.0 (2025-11-13)
- Initial implementation
- Hybrid DHT + Admin API pattern (Approach 7)
- 3 nodes with peer manager sidecars
- Comprehensive documentation
- Automated validation
- Real-time monitoring

## Related Files

- [../../Dockerfile](../../Dockerfile) - Gethrelay container image
- [../../docker-compose-service-discovery-research.md](../../docker-compose-service-discovery-research.md) - Research document
- [../../README.md](../../README.md) - Main gethrelay documentation

## Support Resources

- **Issues**: Check logs with `docker-compose logs -f`
- **Validation**: Run `./scripts/validate-deployment.sh`
- **Monitoring**: Use `./scripts/monitor.sh`
- **Documentation**: Start with [README.md](README.md)
- **Quick Help**: See [QUICKSTART.md](QUICKSTART.md)

---

**Last Updated**: 2025-11-13
**Version**: 1.0.0
**Status**: Production-Ready
