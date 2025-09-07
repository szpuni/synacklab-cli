# Quick Start Guide

Get up and running with Synacklab in just a few minutes. This guide covers the essential setup and basic usage.

## Step 1: Installation

First, install Synacklab following the [Installation Guide](installation.md):

```bash
git clone <repository-url>
cd synacklab
make deps
make build
```

## Step 2: Initialize Configuration

Create your configuration file:

```bash
./bin/synacklab init
```

This creates `~/.synacklab/config.yaml` with default values. Edit it with your AWS SSO details:

```yaml
aws:
  sso:
    start_url: "https://your-company.awsapps.com/start"
    region: "us-east-1"
```

## Step 3: AWS SSO Authentication

### Authenticate with AWS SSO

```bash
synacklab auth aws-login
```

This will:
1. Open your browser for AWS SSO authentication
2. Store your session credentials securely
3. Enable access to AWS profiles

### Sync AWS Profiles

```bash
synacklab auth sync
```

This command:
1. Fetches all available AWS accounts and roles from SSO
2. Creates profiles in `~/.aws/config`
3. Preserves existing non-SSO profiles

### Set Default Profile (Optional)

```bash
synacklab auth aws-config
```

Use the interactive selector to choose your default AWS profile.

## Step 4: Kubernetes Management (Optional)

If you use EKS clusters:

### Discover EKS Clusters

```bash
synacklab auth eks-config
```

This automatically:
1. Discovers EKS clusters in your AWS accounts
2. Adds them to `~/.kube/config`
3. Configures AWS authentication

### Switch Kubernetes Contexts

```bash
synacklab auth eks-ctx
```

Use the interactive selector to switch between Kubernetes contexts.

## Step 5: GitHub Repository Management (Optional)

### Set Up GitHub Authentication

```bash
export GITHUB_TOKEN="your_github_token"
```

Or add to your configuration:

```yaml
github:
  token: "your_github_token"
  organization: "your-org"  # optional
```

### Create a Repository Configuration

Create `my-repo.yaml`:

```yaml
name: my-awesome-repo
description: "An example repository managed by synacklab"
private: false

features:
  issues: true
  wiki: true
  projects: false

branch_protection:
  - pattern: "main"
    required_status_checks:
      - "ci/build"
      - "ci/test"
    required_reviews: 2
    require_code_owner_review: true
```

### Validate and Apply

```bash
# Validate configuration
synacklab github validate my-repo.yaml

# Preview changes
synacklab github apply my-repo.yaml --dry-run

# Apply configuration
synacklab github apply my-repo.yaml
```

## Common Workflows

### Daily AWS Workflow

```bash
# Check authentication status
synacklab auth aws-login

# Switch to production profile
synacklab auth aws-config

# Update kubeconfig with latest EKS clusters
synacklab auth eks-config

# Switch to production cluster
synacklab auth eks-ctx
```

### Repository Management Workflow

```bash
# Validate all repository configurations
synacklab github validate *.yaml

# Preview changes across multiple repos
synacklab github apply multi-repos.yaml --dry-run

# Apply changes to specific repositories
synacklab github apply multi-repos.yaml --repos repo1,repo2
```

## Configuration Examples

### Basic AWS SSO Configuration

```yaml
# ~/.synacklab/config.yaml
aws:
  sso:
    start_url: "https://mycompany.awsapps.com/start"
    region: "us-east-1"
```

### Complete Configuration

```yaml
# ~/.synacklab/config.yaml
aws:
  sso:
    start_url: "https://mycompany.awsapps.com/start"
    region: "us-east-1"

github:
  token: "ghp_your_token_here"
  organization: "mycompany"
```

## Verification

Test that everything is working:

```bash
# Check AWS profiles
aws configure list-profiles

# Check Kubernetes contexts
kubectl config get-contexts

# Test GitHub authentication
synacklab github validate examples/github-simple-repo.yaml
```

## Next Steps

Now that you have Synacklab set up:

1. **Explore AWS Features**: [AWS SSO Documentation](aws-sso.md)
2. **Learn Kubernetes Management**: [Kubernetes Documentation](kubernetes.md)
3. **Master GitHub Automation**: [GitHub Documentation](github.md)
4. **Review All Commands**: [Command Reference](commands.md)
5. **Check Examples**: Browse the `examples/` directory

## Troubleshooting Quick Issues

### AWS SSO Authentication Fails

```bash
# Check your SSO URL and region
cat ~/.synacklab/config.yaml

# Clear cached credentials and retry
rm -rf ~/.synacklab/cache/
synacklab auth aws-login
```

### No EKS Clusters Found

```bash
# Check specific region
synacklab auth eks-config --region us-west-2

# Verify AWS profile has EKS permissions
aws eks list-clusters
```

### GitHub Authentication Issues

```bash
# Verify token has correct permissions
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user

# Check token scopes
curl -H "Authorization: token $GITHUB_TOKEN" -I https://api.github.com/user
```

### Command Not Found

```bash
# Add to PATH or use full path
export PATH="$PWD/bin:$PATH"

# Or install globally
make install
```

## Getting Help

- Use `--help` with any command: `synacklab auth --help`
- Check the [Troubleshooting Guide](troubleshooting.md)
- Review [Configuration Reference](config-reference.md)
- Browse [Examples](examples.md)