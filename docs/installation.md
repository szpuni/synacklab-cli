# Installation Guide

This guide covers all methods for installing Synacklab CLI on your system.

## Prerequisites

- **Go 1.21 or later** (for building from source)
- **Git** (for cloning the repository)
- **AWS CLI** (optional, but recommended for AWS integration)

## Installation Methods

### Method 1: Build from Source (Recommended)

This is currently the primary installation method:

```bash
# Clone the repository
git clone <repository-url>
cd synacklab

# Install dependencies and build
make deps
make build

# The binary will be available at ./bin/synacklab
./bin/synacklab --help
```

### Method 2: Install Globally

To install Synacklab globally on your system:

```bash
# After building from source
make install

# Or manually copy to your PATH
sudo cp ./bin/synacklab /usr/local/bin/synacklab

# Verify installation
synacklab --help
```

### Method 3: Development Build

For development with debug symbols:

```bash
make dev-build
./bin/synacklab-dev --help
```

### Method 4: Cross-Platform Builds

Build for multiple platforms:

```bash
make build-all
```

This creates binaries for:
- Linux (AMD64): `synacklab-linux-amd64`
- macOS (AMD64): `synacklab-darwin-amd64`
- macOS (ARM64): `synacklab-darwin-arm64`
- Windows (AMD64): `synacklab-windows-amd64.exe`

## Verification

After installation, verify Synacklab is working:

```bash
# Check version and help
synacklab --help

# Initialize configuration (optional)
synacklab init

# Test AWS SSO functionality (requires configuration)
synacklab auth --help
```

## Shell Completion (Optional)

Enable shell completion for better CLI experience:

### Bash

```bash
# Add to ~/.bashrc
eval "$(synacklab completion bash)"

# Or generate completion file
synacklab completion bash > /etc/bash_completion.d/synacklab
```

### Zsh

```bash
# Add to ~/.zshrc
eval "$(synacklab completion zsh)"

# Or for oh-my-zsh
synacklab completion zsh > ~/.oh-my-zsh/completions/_synacklab
```

### Fish

```bash
synacklab completion fish > ~/.config/fish/completions/synacklab.fish
```

## Dependencies

Synacklab uses these external tools that should be available in your PATH:

### Required for AWS Features
- **AWS CLI v2** (recommended): For enhanced AWS integration
  ```bash
  # macOS
  brew install awscli
  
  # Linux
  curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
  unzip awscliv2.zip
  sudo ./aws/install
  ```

### Required for Kubernetes Features
- **kubectl**: For Kubernetes cluster management
  ```bash
  # macOS
  brew install kubectl
  
  # Linux
  curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
  sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
  ```

### Optional for Enhanced Experience
- **fzf**: For fuzzy finding (auto-detected and used if available)
  ```bash
  # macOS
  brew install fzf
  
  # Linux
  git clone --depth 1 https://github.com/junegunn/fzf.git ~/.fzf
  ~/.fzf/install
  ```

## Configuration Directory

Synacklab creates a configuration directory at `~/.synacklab/`:

```
~/.synacklab/
├── config.yaml          # Main configuration file
├── cache/               # Cached data (tokens, etc.)
└── logs/                # Application logs
```

## Updating

To update Synacklab to the latest version:

```bash
# Pull latest changes
git pull origin main

# Rebuild
make clean
make build

# Or reinstall globally
make install
```

## Uninstallation

To remove Synacklab:

```bash
# Remove binary
sudo rm /usr/local/bin/synacklab

# Remove configuration (optional)
rm -rf ~/.synacklab/

# Remove shell completion (if installed)
rm /etc/bash_completion.d/synacklab  # Bash
rm ~/.oh-my-zsh/completions/_synacklab  # Zsh
rm ~/.config/fish/completions/synacklab.fish  # Fish
```

## Troubleshooting Installation

### Build Errors

**Go version too old:**
```bash
# Check Go version
go version

# Update Go if needed (macOS)
brew install go

# Update Go if needed (Linux)
# Download from https://golang.org/dl/
```

**Missing dependencies:**
```bash
# Clean and reinstall dependencies
make clean
make deps
make build
```

### Permission Errors

**Cannot write to /usr/local/bin:**
```bash
# Use sudo for global installation
sudo make install

# Or install to user directory
mkdir -p ~/bin
cp ./bin/synacklab ~/bin/
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Runtime Errors

**Command not found:**
```bash
# Check if binary is in PATH
which synacklab

# Add to PATH if needed
export PATH="/path/to/synacklab:$PATH"
```

**Permission denied:**
```bash
# Make binary executable
chmod +x ./bin/synacklab
```

## Next Steps

After successful installation:

1. [Complete the Quick Start guide](quick-start.md)
2. [Configure Synacklab](configuration.md)
3. [Set up AWS SSO authentication](aws-sso.md)