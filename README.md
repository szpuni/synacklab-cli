# Synacklab CLI

A powerful command-line tool for DevOps engineers to streamline AWS SSO authentication, Kubernetes management, and GitHub repository operations.

## 🚀 Features

- **🔐 AWS SSO Authentication**: Device authorization flow with automatic token refresh
- **📋 Profile Management**: Sync and manage AWS SSO profiles across multiple accounts
- **🎯 Kubernetes Integration**: EKS cluster discovery and context management
- **🐙 GitHub Automation**: Declarative repository management with YAML configuration
- **🔄 Multi-Repository Support**: Batch operations across multiple repositories
- **🛡️ Security First**: No plaintext credential storage, uses SSO tokens only
- **⚡ Performance Optimized**: Caching, parallel operations, and smart rate limiting

## 📚 Documentation

**Complete documentation is available in the [`docs/`](docs/) directory:**

### Getting Started
- **[Installation Guide](docs/installation.md)** - Install and set up Synacklab
- **[Quick Start](docs/quick-start.md)** - Get up and running in minutes
- **[Configuration](docs/configuration.md)** - Configure Synacklab for your environment

### Core Features
- **[AWS SSO Authentication](docs/aws-sso.md)** - Authenticate and manage AWS profiles
- **[Kubernetes Management](docs/kubernetes.md)** - Manage EKS clusters and contexts
- **[GitHub Repository Management](docs/github.md)** - Declarative GitHub repository configuration

### Reference
- **[Command Reference](docs/commands.md)** - Complete command documentation
- **[Configuration Reference](docs/config-reference.md)** - All configuration options
- **[Examples](docs/examples.md)** - Real-world usage examples
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

## ⚡ Quick Start

1. **Install Synacklab**:
   ```bash
   git clone <repository-url>
   cd synacklab
   make build
   ```

2. **Initialize configuration**:
   ```bash
   ./bin/synacklab init
   ```

3. **Configure AWS SSO** (edit `~/.synacklab/config.yaml`):
   ```yaml
   aws:
     sso:
       start_url: "https://your-company.awsapps.com/start"
       region: "us-east-1"
   ```

4. **Authenticate and sync profiles**:
   ```bash
   synacklab auth aws-login
   synacklab auth sync
   ```

## 🎯 Core Capabilities

### AWS SSO Authentication
- **Device Authorization Flow**: Secure browser-based authentication
- **Profile Synchronization**: Automatic sync of all AWS accounts and roles
- **Multi-Account Support**: Seamless switching between AWS accounts
- **Session Management**: Automatic token refresh and validation

### Kubernetes Management
- **EKS Cluster Discovery**: Automatic discovery across AWS accounts and regions
- **Context Switching**: Interactive context selection with fuzzy search
- **Kubeconfig Integration**: Seamless integration with `~/.kube/config`
- **Multi-Cluster Support**: Manage multiple clusters efficiently

### GitHub Repository Management
- **Declarative Configuration**: Define repositories as code using YAML
- **Multi-Repository Support**: Batch operations across multiple repositories
- **Branch Protection**: Configure branch protection rules and policies
- **Access Control**: Manage collaborators and team permissions
- **Webhook Management**: Configure webhooks for CI/CD and notifications

## 🛠️ Development

### Build Commands

```bash
# Install dependencies
make deps

# Format and lint code
make fmt
make lint

# Run tests
make test

# Build binary
make build

# Cross-compile for all platforms
make build-all
```

### Project Structure

```
synacklab/
├── cmd/synacklab/       # Application entry point
├── internal/cmd/        # CLI command implementations
├── pkg/                 # Public packages
├── examples/            # Configuration examples
├── docs/                # Complete documentation
└── test/integration/    # Integration tests
```

## 🤝 Contributing

We welcome contributions! Please see our development guidelines:

1. **Fork the repository** and create a feature branch
2. **Follow the coding standards** defined in our linting configuration
3. **Add tests** for new functionality
4. **Update documentation** as needed
5. **Submit a pull request** with a clear description

### Development Setup

```bash
# Clone and setup
git clone <repository-url>
cd synacklab
make deps

# Run tests
make test
make integration-test

# Build and test locally
make build
./bin/synacklab --help
```

## 📄 License

See [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: Browse the [`docs/`](docs/) directory
- **Examples**: Check the [`examples/`](examples/) directory
- **Issues**: Report bugs and request features on GitHub
- **Discussions**: Join community discussions for questions and ideas