# Kubernetes Management

Synacklab provides powerful Kubernetes cluster discovery and context management, with special focus on Amazon EKS integration. This guide covers all Kubernetes-related features.

## Overview

Kubernetes management in Synacklab includes:

- **EKS Cluster Discovery**: Automatic discovery across AWS accounts and regions
- **Kubeconfig Management**: Seamless integration with `~/.kube/config`
- **Context Switching**: Interactive context selection with fuzzy search
- **Multi-Account Support**: Discover clusters across multiple AWS accounts
- **Authentication Integration**: Automatic AWS authentication configuration

## Prerequisites

- **kubectl**: Kubernetes command-line tool
- **AWS CLI v2**: For EKS authentication (recommended)
- **AWS SSO Authentication**: Configured via Synacklab
- **EKS Cluster Access**: Appropriate IAM permissions

### Installation Requirements

```bash
# Install kubectl (macOS)
brew install kubectl

# Install kubectl (Linux)
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Verify installation
kubectl version --client
```

## Commands

### EKS Cluster Discovery

#### `synacklab auth eks-config`

Discover EKS clusters and update kubeconfig automatically.

```bash
# Discover clusters in all regions
synacklab auth eks-config

# Discover clusters in specific region
synacklab auth eks-config --region us-west-2

# Preview changes without applying
synacklab auth eks-config --dry-run

# Discover in specific region with dry-run
synacklab auth eks-config --region eu-west-1 --dry-run
```

**Features:**
- Scans all AWS regions (or specified region)
- Discovers EKS clusters across all accessible accounts
- Adds clusters to `~/.kube/config`
- Configures AWS authentication automatically
- Preserves existing kubeconfig entries
- Sorts entries for consistent output

**Example Output:**
```
🔍 Discovering EKS clusters...
🌍 Searching for EKS clusters in 14 region(s)...
📋 Found 5 EKS cluster(s):
  • production-cluster (us-east-1) - ACTIVE
  • staging-cluster (us-east-1) - ACTIVE
  • development-cluster (us-west-2) - ACTIVE
  • testing-cluster (eu-west-1) - ACTIVE
  • demo-cluster (ap-southeast-1) - ACTIVE

📊 Added 3 new clusters, updated 2 existing clusters
✅ Successfully updated kubeconfig with 5 EKS cluster(s)
```

### Context Management

#### `synacklab auth eks-ctx`

Switch between Kubernetes contexts interactively.

```bash
# Interactive context selection
synacklab auth eks-ctx

# List all contexts without switching
synacklab auth eks-ctx --list

# Use filtering mode for better search
synacklab auth eks-ctx --filter
```

**Features:**
- Interactive fuzzy search through contexts
- Shows cluster, user, and namespace information
- Highlights current context
- Updates `current-context` in kubeconfig
- Validates context configuration

**Example Output:**
```
🔍 Loading Kubernetes contexts...
📋 Found 8 Kubernetes context(s)

🔍 Select Kubernetes context:
> production-cluster-us-east-1    Cluster: production-cluster, Namespace: default
  staging-cluster-us-east-1       Cluster: staging-cluster, Namespace: default (current)
  development-cluster-us-west-2   Cluster: development-cluster, Namespace: kube-system
  testing-cluster-eu-west-1       Cluster: testing-cluster, Namespace: default

✅ Successfully switched to context: production-cluster-us-east-1
```

#### `synacklab auth eks-ctx --list`

Display all available contexts in a formatted table.

```bash
synacklab auth eks-ctx --list
```

**Example Output:**
```
📋 Available Kubernetes contexts:
------------------------------------------------------------
* production-cluster-us-east-1  | Cluster: production-cluster    | Namespace: default
  staging-cluster-us-east-1     | Cluster: staging-cluster       | Namespace: default
  development-cluster-us-west-2 | Cluster: development-cluster   | Namespace: kube-system
  testing-cluster-eu-west-1     | Cluster: testing-cluster       | Namespace: default

Current context: production-cluster-us-east-1
```

## Kubeconfig Integration

### Generated Configuration

Synacklab creates standard kubeconfig entries:

```yaml
apiVersion: v1
kind: Config
current-context: production-cluster-us-east-1

clusters:
- name: production-cluster-us-east-1
  cluster:
    server: https://A1B2C3D4E5F6G7H8.gr7.us-east-1.eks.amazonaws.com

contexts:
- name: production-cluster-us-east-1
  context:
    cluster: production-cluster-us-east-1
    user: production-cluster-us-east-1

users:
- name: production-cluster-us-east-1
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - production-cluster
        - --region
        - us-east-1
```

### Context Naming Convention

- **Format**: `{cluster-name}-{region}`
- **Examples**:
  - `production-cluster-us-east-1`
  - `staging-cluster-us-west-2`
  - `development-cluster-eu-west-1`

### Authentication Configuration

Each cluster uses AWS CLI for authentication:
- **Command**: `aws eks get-token`
- **Parameters**: `--cluster-name` and `--region`
- **Profile**: Uses current AWS profile/credentials
- **Token**: Automatically refreshed by AWS CLI

## Workflows

### Initial Setup Workflow

```bash
# 1. Authenticate with AWS SSO
synacklab auth aws-login

# 2. Sync AWS profiles
synacklab auth sync

# 3. Discover EKS clusters
synacklab auth eks-config

# 4. Switch to desired cluster
synacklab auth eks-ctx

# 5. Verify access
kubectl get nodes
```

### Daily Workflow

```bash
# Quick cluster switching
synacklab auth eks-ctx

# Verify current context
kubectl config current-context

# Check cluster access
kubectl get namespaces
```

### Multi-Account Workflow

```bash
# Switch to production AWS account
export AWS_PROFILE=production-administratoraccess

# Discover production clusters
synacklab auth eks-config --region us-east-1

# Switch to development AWS account
export AWS_PROFILE=development-poweruseraccess

# Discover development clusters
synacklab auth eks-config --region us-west-2

# Switch between clusters
synacklab auth eks-ctx
```

### Automated Workflow

```bash
#!/bin/bash
# Automated EKS setup script

echo "Setting up EKS environment..."

# Ensure AWS authentication
synacklab auth aws-login

# Update kubeconfig with latest clusters
synacklab auth eks-config

# List available contexts
echo "Available clusters:"
synacklab auth eks-ctx --list

echo "EKS environment ready!"
```

## Multi-Region Discovery

### Automatic Region Detection

By default, Synacklab searches common AWS regions:

- **US Regions**: us-east-1, us-east-2, us-west-1, us-west-2
- **EU Regions**: eu-west-1, eu-west-2, eu-west-3, eu-central-1
- **Asia Pacific**: ap-southeast-1, ap-southeast-2, ap-northeast-1, ap-northeast-2
- **Other**: ca-central-1, sa-east-1

### Specific Region Discovery

```bash
# Single region
synacklab auth eks-config --region us-east-1

# Multiple regions (run multiple times)
synacklab auth eks-config --region us-east-1
synacklab auth eks-config --region eu-west-1
synacklab auth eks-config --region ap-southeast-1
```

### Region-Specific Workflows

```bash
# Production in us-east-1
export AWS_PROFILE=production-administratoraccess
synacklab auth eks-config --region us-east-1

# Staging in us-west-2
export AWS_PROFILE=staging-poweruseraccess
synacklab auth eks-config --region us-west-2

# Development in eu-west-1
export AWS_PROFILE=development-poweruseraccess
synacklab auth eks-config --region eu-west-1
```

## Advanced Usage

### Custom Namespace Configuration

Set default namespaces for contexts:

```bash
# Switch context
synacklab auth eks-ctx

# Set namespace for current context
kubectl config set-context --current --namespace=my-namespace

# Verify configuration
kubectl config view --minify
```

### Multiple Kubeconfig Files

Manage separate kubeconfig files:

```bash
# Use custom kubeconfig location
export KUBECONFIG=~/.kube/production-config
synacklab auth eks-config

# Merge multiple kubeconfig files
export KUBECONFIG=~/.kube/config:~/.kube/production-config:~/.kube/staging-config
kubectl config view --flatten > ~/.kube/merged-config
```

### Cluster Access Validation

```bash
# Test cluster connectivity
kubectl cluster-info

# Check authentication
kubectl auth whoami

# Verify permissions
kubectl auth can-i get pods
kubectl auth can-i create deployments
```

## Integration with Other Tools

### Helm Integration

```bash
# Switch to cluster
synacklab auth eks-ctx

# Use Helm with current context
helm list
helm install my-app ./my-chart
```

### Terraform Integration

```bash
# Configure Terraform for EKS
export AWS_PROFILE=production-administratoraccess
synacklab auth eks-ctx  # Select EKS cluster

# Terraform will use current kubectl context
terraform plan
terraform apply
```

### CI/CD Integration

```bash
#!/bin/bash
# CI/CD script for EKS deployment

# Authenticate
synacklab auth aws-login

# Update kubeconfig
synacklab auth eks-config --region $AWS_REGION

# Set specific context
kubectl config use-context $CLUSTER_NAME-$AWS_REGION

# Deploy application
kubectl apply -f deployment.yaml
```

## Troubleshooting

### Common Issues

#### No Clusters Found

```bash
# Check AWS authentication
aws sts get-caller-identity

# Verify EKS permissions
aws eks list-clusters --region us-east-1

# Check specific region
synacklab auth eks-config --region us-east-1 --dry-run
```

#### Authentication Errors

```bash
# Update AWS credentials
synacklab auth aws-login

# Test EKS access
aws eks describe-cluster --name my-cluster --region us-east-1

# Verify kubectl authentication
kubectl auth whoami
```

#### Context Switch Failures

```bash
# List all contexts
kubectl config get-contexts

# Validate specific context
kubectl config use-context production-cluster-us-east-1

# Check kubeconfig syntax
kubectl config view
```

#### Permission Denied

```bash
# Check IAM permissions for EKS
aws iam get-user
aws sts get-caller-identity

# Verify cluster access
aws eks describe-cluster --name my-cluster --region us-east-1

# Check RBAC permissions
kubectl auth can-i get pods --as=system:serviceaccount:default:default
```

### Debug Mode

Enable debug logging:

```bash
export SYNACKLAB_LOG_LEVEL="debug"
synacklab auth eks-config

# Enable kubectl debug
kubectl config view --minify
kubectl cluster-info dump
```

### Validation Commands

```bash
# Test AWS CLI EKS integration
aws eks get-token --cluster-name my-cluster --region us-east-1

# Validate kubeconfig
kubectl config validate ~/.kube/config

# Test cluster connectivity
kubectl get --raw /healthz
```

## Security Considerations

### Authentication Flow

1. **AWS SSO**: Authenticate via Synacklab
2. **AWS Profile**: Use appropriate AWS profile
3. **EKS Token**: AWS CLI generates temporary token
4. **Kubernetes API**: Token used for API authentication

### Best Practices

1. **Least Privilege**: Use appropriate AWS profiles for each cluster
2. **Token Rotation**: Tokens automatically expire and refresh
3. **Context Isolation**: Use separate contexts for different environments
4. **Audit Logging**: Enable EKS audit logging for compliance
5. **RBAC**: Implement proper Kubernetes RBAC policies

### Compliance

- Supports AWS CloudTrail logging
- Compatible with EKS audit logging
- Respects IAM policies and permissions
- No long-term credential storage

## Performance Optimization

### Caching

```bash
# Cache cluster information
export SYNACKLAB_CACHE_ENABLED=true

# Set cache TTL (seconds)
export SYNACKLAB_CACHE_TTL=3600
```

### Parallel Discovery

```bash
# Discover multiple regions in parallel
synacklab auth eks-config --region us-east-1 &
synacklab auth eks-config --region us-west-2 &
synacklab auth eks-config --region eu-west-1 &
wait
```

### Selective Updates

```bash
# Update only specific clusters
kubectl config delete-context old-cluster-context
synacklab auth eks-config --region us-east-1
```

## Next Steps

- [Set up GitHub repository management](github.md)
- [Review AWS SSO integration](aws-sso.md)
- [Check command reference](commands.md)
- [Browse troubleshooting guide](troubleshooting.md)