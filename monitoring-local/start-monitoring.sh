#!/bin/bash
set -e

echo "🚀 Starting Local Grafana Monitoring for Remote Gethrelay Nodes"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker Desktop."
    exit 1
fi

# Check SSH connection
if ! ssh -o ConnectTimeout=5 geth-onion-dev "echo connected" > /dev/null 2>&1; then
    echo "❌ Cannot connect to geth-onion-dev via SSH."
    echo "   Please check your SSH configuration."
    exit 1
fi

# Kill existing SSH tunnels
echo "🔧 Cleaning up existing SSH tunnels..."
pkill -f 'ssh.*16060' 2>/dev/null || true
pkill -f 'ssh.*26060' 2>/dev/null || true
pkill -f 'ssh.*36060' 2>/dev/null || true
pkill -f 'ssh.*46060' 2>/dev/null || true
pkill -f 'ssh.*56060' 2>/dev/null || true
pkill -f 'ssh.*61060' 2>/dev/null || true
sleep 2

# Create SSH tunnels
echo "🔒 Creating SSH tunnels to remote nodes..."
echo "   Creating tunnels for --only-onion nodes..."
ssh -f -N -L 16060:172.20.0.11:6060 geth-onion-dev
ssh -f -N -L 26060:172.20.0.12:6060 geth-onion-dev
ssh -f -N -L 36060:172.20.0.13:6060 geth-onion-dev
echo "   Creating tunnels for --prefer-tor nodes..."
ssh -f -N -L 46060:172.20.0.14:6060 geth-onion-dev
ssh -f -N -L 56060:172.20.0.15:6060 geth-onion-dev
ssh -f -N -L 61060:172.20.0.16:6060 geth-onion-dev

echo "✅ SSH tunnels created:"
echo "   --only-onion nodes:"
echo "   - localhost:16060 → gethrelay-1"
echo "   - localhost:26060 → gethrelay-2"
echo "   - localhost:36060 → gethrelay-3"
echo "   --prefer-tor nodes:"
echo "   - localhost:46060 → gethrelay-4"
echo "   - localhost:56060 → gethrelay-5"
echo "   - localhost:61060 → gethrelay-6"

# Verify tunnels
sleep 2
echo ""
echo "🔍 Verifying metrics endpoints..."
for port in 16060 26060 36060 46060 56060 61060; do
    if curl -s --max-time 5 http://localhost:$port/debug/metrics/prometheus | head -1 > /dev/null 2>&1; then
        echo "   ✅ localhost:$port is responding"
    else
        echo "   ⚠️  localhost:$port is not responding (metrics may not be enabled on remote)"
    fi
done

# Start docker-compose
echo ""
echo "🐳 Starting Prometheus + Grafana..."
docker-compose up -d

# Wait for services
echo "⏳ Waiting for services to start..."
sleep 5

# Check service health
echo ""
echo "🏥 Checking service health..."
if curl -s http://localhost:9090/-/healthy > /dev/null 2>&1; then
    echo "   ✅ Prometheus is healthy"
else
    echo "   ❌ Prometheus is not healthy"
fi

if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
    echo "   ✅ Grafana is healthy"
else
    echo "   ❌ Grafana is not healthy"
fi

echo ""
echo "✨ Monitoring stack is running!"
echo ""
echo "📊 Access your dashboards:"
echo "   Prometheus: http://localhost:9090"
echo "   Grafana:    http://localhost:8080"
echo ""
echo "🔑 Grafana credentials:"
echo "   Username: admin"
echo "   Password: admin"
echo ""
echo "📈 Check targets: http://localhost:9090/targets"
echo ""
echo "To stop: docker-compose down && pkill -f 'ssh.*6060'"
