#!/bin/bash
# Update only-onion StatefulSets with Tor peer discovery init container

set -e

DEPLOYMENT_FILE="deployment/k8s/deployments.yaml"
TEMP_FILE=$(mktemp)

echo "Updating only-onion StatefulSets with Tor peer discovery..."

# Use Python to do a more sophisticated YAML update
python3 << 'EOF'
import re

# Read the deployment file
with open('deployment/k8s/deployments.yaml', 'r') as f:
    content = f.read()

# Define the init container to add
init_container = '''      - name: tor-peer-discovery
        image: bitnami/kubectl:latest
        command: ['/bin/sh']
        args: ['/scripts/discover-peers.sh']
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: P2P_PORT
          value: "30303"
        volumeMounts:
        - name: tor-data
          mountPath: /var/lib/tor
          readOnly: true
        - name: geth-data
          mountPath: /data/geth
        - name: discovery-script
          mountPath: /scripts'''

# Define the volumes to add
discovery_volumes = '''      - name: discovery-script
        configMap:
          name: tor-peer-discovery-script
          defaultMode: 0755
      - name: geth-data
        emptyDir: {}'''

# Pattern to find only-onion StatefulSets
statefulset_pattern = r'(# StatefulSet \d+: Tor-Only mode.*?kind: StatefulSet.*?metadata:.*?name: gethrelay-only-onion-\d+)'

# For each only-onion StatefulSet
statefulsets = list(re.finditer(statefulset_pattern, content, re.DOTALL))
print(f"Found {len(statefulsets)} only-onion StatefulSets")

# Process in reverse to maintain positions
for match in reversed(statefulsets):
    start = match.start()

    # Find the spec: section after this StatefulSet metadata
    spec_pos = content.find('    spec:', start)
    if spec_pos == -1:
        continue

    # Add serviceAccountName after spec:
    serviceaccount_line = '\n      serviceAccountName: gethrelay-tor-discovery'
    security_context_pos = content.find('      securityContext:', spec_pos)
    if security_context_pos != -1:
        content = content[:security_context_pos] + serviceaccount_line + '\n' + content[security_context_pos:]

    # Find the initContainers section
    init_containers_pos = content.find('      initContainers:', spec_pos)
    if init_containers_pos == -1:
        continue

    # Find the end of fix-permissions init container
    fix_perm_start = content.find('      - name: fix-permissions', init_containers_pos)
    if fix_perm_start == -1:
        continue

    # Find the containers: line (end of init containers)
    containers_pos = content.find('      containers:', fix_perm_start)
    if containers_pos == -1:
        continue

    # Insert the new init container before containers:
    content = content[:containers_pos] + init_container + '\n' + content[containers_pos:]

    # Find the volumes section
    volumes_pos = content.find('      volumes:', containers_pos)
    if volumes_pos == -1:
        continue

    # Find the end of volumes (before volumeClaimTemplates)
    volume_claim_pos = content.find('  volumeClaimTemplates:', volumes_pos)
    if volume_claim_pos == -1:
        continue

    # Insert new volumes before volumeClaimTemplates
    content = content[:volume_claim_pos] + discovery_volumes + '\n' + content[volume_claim_pos:]

# Write the updated content
with open('deployment/k8s/deployments.yaml', 'w') as f:
    f.write(content)

print("Successfully updated deployments.yaml with Tor peer discovery")
EOF

echo "✓ Updated deployments.yaml"
echo ""
echo "Changes made:"
echo "  - Added serviceAccountName: gethrelay-tor-discovery"
echo "  - Added tor-peer-discovery init container"
echo "  - Added discovery-script and geth-data volumes"
echo ""
echo "Next: Run './deployment/scripts/deploy-tor-discovery.sh' to deploy"
