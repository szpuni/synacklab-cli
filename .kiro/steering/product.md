---
inclusion: always
---

# Product Overview

Synacklab is a CLI tool for DevOps engineers to streamline AWS SSO authentication and profile management.

## Core Features
- **AWS SSO Authentication**: Device authorization flow with automatic token refresh
- **Profile Management**: List, select, and switch between AWS accounts and roles
- **Configuration Management**: Update `~/.aws/config` with selected profiles
- **Multi-Mode Operation**: Interactive fuzzy selection and non-interactive scripting support
- **GitHub Integration**: Repository management and configuration (planned/in development)

## User Experience Principles
- **Zero-friction workflow**: Minimize steps between authentication and productive work
- **Fail-fast validation**: Clear error messages with actionable guidance
- **Consistent interface**: Predictable command patterns across all features
- **Scriptable by default**: All interactive features have non-interactive equivalents

## Development Guidelines
- **User-centric design**: Every feature should solve a real DevOps workflow pain point
- **Backward compatibility**: Configuration and CLI changes must be non-breaking
- **Security first**: Never store credentials in plaintext; use AWS SSO tokens only
- **Performance matters**: Commands should complete in <2 seconds for typical operations

## Configuration Conventions
- User config: `~/.synacklab/config.yaml`
- AWS config updates: `~/.aws/config` (standard AWS CLI location)
- Example configs: Always provide working examples in `config.example.yaml`
- Validation: All config should be validated on load with helpful error messages