#!/bin/bash
# prepare-release.sh - Prepare repository for initial release

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     Preparing gethrelay Repository for Release             ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not a git repository"
    exit 1
fi

# Show current status
echo "📊 Current Status:"
echo "   Branch: $(git branch --show-current)"
echo "   Remote: $(git remote get-url origin 2>/dev/null || echo 'none')"
echo "   Changes: $(git status --short | wc -l | tr -d ' ') files"
echo ""

# Ask for confirmation
read -p "Continue with preparing release? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 1
fi

# Stage all changes
echo ""
echo "📦 Staging all changes..."
git add -A

# Show what will be committed
echo ""
echo "📋 Files to be committed:"
git status --short | head -20
if [ $(git status --short | wc -l) -gt 20 ]; then
    echo "... and $(($(git status --short | wc -l) - 20)) more files"
fi

echo ""
read -p "Create initial commit? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Files staged but not committed. Run:"
    echo "  git commit -F .gitmessage"
    exit 0
fi

# Create commit
echo ""
echo "💾 Creating initial commit..."
git commit -F .gitmessage || git commit -m "Initial commit: gethrelay - Ethereum P2P Relay Node with JSON-RPC Proxy"

echo ""
echo "✅ Commit created!"
echo ""
echo "Commit hash: $(git rev-parse HEAD)"
echo ""

# Ask about remote
echo "🌐 Repository Remote Setup:"
echo ""
echo "Current remote: $(git remote get-url origin 2>/dev/null || echo 'none')"
echo ""
read -p "Do you want to update the remote URL? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    read -p "Enter new remote URL (e.g., https://github.com/username/gethrelay.git): " REMOTE_URL
    if [ -n "$REMOTE_URL" ]; then
        if git remote | grep -q "^origin$"; then
            echo "Updating origin remote..."
            git remote set-url origin "$REMOTE_URL"
        else
            echo "Adding origin remote..."
            git remote add origin "$REMOTE_URL"
        fi
        echo "✅ Remote updated to: $REMOTE_URL"
    fi
fi

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    Next Steps                                 ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "1. Review the commit:"
echo "   git show HEAD"
echo ""
echo "2. If remote is set, push to your repository:"
echo "   git push -u origin $(git branch --show-current)"
echo ""
echo "3. Or create a new repository on GitHub/GitLab and push:"
echo "   git remote add origin <your-repo-url>"
echo "   git push -u origin $(git branch --show-current)"
echo ""
echo "4. Consider creating a main/master branch:"
echo "   git branch -M main"
echo "   git push -u origin main"
echo ""


