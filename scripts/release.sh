#!/bin/bash

# Release script for synacklab with built-in semantic versioning
# Usage: ./scripts/release.sh [patch|minor|major]
# Example: ./scripts/release.sh patch

set -e

BUMP_TYPE=${1:-patch}

# Validate bump type
if [[ ! $BUMP_TYPE =~ ^(patch|minor|major)$ ]]; then
    echo "Usage: $0 [patch|minor|major]"
    echo "  patch: Bug fixes (1.0.0 -> 1.0.1)"
    echo "  minor: New features (1.0.0 -> 1.1.0)"
    echo "  major: Breaking changes (1.0.0 -> 2.0.0)"
    echo ""
    echo "Default: patch"
    exit 1
fi

# Function to get the latest version tag
get_latest_version() {
    # Get the latest version tag, fallback to v0.0.0 if none exists
    git tag -l "v*" | sort -V | tail -n1 || echo "v0.0.0"
}

# Function to bump version
bump_version() {
    local version=$1
    local bump_type=$2
    
    # Remove 'v' prefix if present
    version=${version#v}
    
    # Split version into parts
    IFS='.' read -ra VERSION_PARTS <<< "$version"
    local major=${VERSION_PARTS[0]:-0}
    local minor=${VERSION_PARTS[1]:-0}
    local patch=${VERSION_PARTS[2]:-0}
    
    case $bump_type in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
    esac
    
    echo "v${major}.${minor}.${patch}"
}

# Get current version and calculate next version
CURRENT_VERSION=$(get_latest_version)
NEXT_VERSION=$(bump_version "$CURRENT_VERSION" "$BUMP_TYPE")

echo "Current version: $CURRENT_VERSION"
echo "Next version: $NEXT_VERSION"
echo "Bump type: $BUMP_TYPE"

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "Warning: You're not on the main branch. Current branch: $CURRENT_BRANCH"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check if working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory is not clean. Please commit or stash your changes."
    git status --short
    exit 1
fi

# Check if tag already exists
if git tag -l | grep -q "^$NEXT_VERSION$"; then
    echo "Error: Tag $NEXT_VERSION already exists"
    exit 1
fi

# Pull latest changes
echo "Pulling latest changes..."
git pull origin main

# Run tests
echo "Running tests..."
make test

# Confirm release
echo ""
read -p "Create release $NEXT_VERSION? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    exit 0
fi

# Create and push tag
echo "Creating tag $NEXT_VERSION..."
git tag -a "$NEXT_VERSION" -m "Release $NEXT_VERSION"

echo "Pushing tag to origin..."
git push origin "$NEXT_VERSION"

echo "Release $NEXT_VERSION created successfully!"
echo "GitHub Actions will automatically create the release with binaries."
echo "Check the progress at: https://github.com/synacklab/synacklab/actions"