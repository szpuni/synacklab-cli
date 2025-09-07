# Synacklab CLI Documentation

Welcome to the comprehensive documentation for Synacklab CLI, a powerful DevOps tool for managing AWS SSO authentication, Kubernetes contexts, and GitHub repositories.

## Quick Navigation

### Getting Started
- [Installation Guide](installation.md) - Install and set up Synacklab
- [Quick Start](quick-start.md) - Get up and running in minutes
- [Configuration](configuration.md) - Configure Synacklab for your environment

### Core Features
- [AWS SSO Authentication](aws-sso.md) - Authenticate and manage AWS profiles
- [Kubernetes Management](kubernetes.md) - Manage EKS clusters and contexts
- [GitHub Repository Management](github.md) - Declarative GitHub repository configuration

### Advanced Usage
- [Command Reference](commands.md) - Complete command documentation
- [Configuration Reference](config-reference.md) - All configuration options
- [Examples](examples.md) - Real-world usage examples
- [Troubleshooting](troubleshooting.md) - Common issues and solutions

### Development
- [Development Guide](development.md) - Contributing to Synacklab
- [API Reference](api-reference.md) - Internal API documentation

## What is Synacklab?

Synacklab is a CLI tool designed for DevOps engineers to streamline common workflows:

- **AWS SSO Authentication**: Simplify AWS SSO device authorization and profile management
- **Profile Synchronization**: Automatically sync all AWS SSO profiles to local configuration
- **Kubernetes Integration**: Discover EKS clusters and manage kubectl contexts
- **GitHub Automation**: Declaratively manage GitHub repositories, teams, and permissions
- **Multi-Repository Support**: Batch operations across multiple repositories

## Key Benefits

- **Zero-friction workflow**: Minimize steps between authentication and productive work
- **Fail-fast validation**: Clear error messages with actionable guidance
- **Consistent interface**: Predictable command patterns across all features
- **Scriptable by default**: All interactive features have non-interactive equivalents
- **Security first**: Never stores credentials in plaintext; uses AWS SSO tokens only

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   AWS SSO       │    │   Kubernetes    │    │     GitHub      │
│ Authentication  │    │   Management    │    │   Management    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Synacklab CLI  │
                    │                 │
                    │ • Configuration │
                    │ • Validation    │
                    │ • Fuzzy Search  │
                    │ • Batch Ops     │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Local Files     │
                    │                 │
                    │ • ~/.aws/config │
                    │ • ~/.kube/config│
                    │ • ~/.synacklab/ │
                    └─────────────────┘
```

## Getting Help

- **Command Help**: Use `synacklab --help` or `synacklab <command> --help`
- **Documentation**: Browse this documentation for detailed guides
- **Examples**: Check the `examples/` directory for configuration samples
- **Issues**: Report bugs and request features on GitHub

## Next Steps

1. [Install Synacklab](installation.md)
2. [Complete the Quick Start guide](quick-start.md)
3. [Configure your environment](configuration.md)
4. Explore specific features:
   - [AWS SSO Authentication](aws-sso.md)
   - [Kubernetes Management](kubernetes.md)
   - [GitHub Repository Management](github.md)