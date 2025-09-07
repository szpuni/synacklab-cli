# Troubleshooting Guide

Comprehensive troubleshooting guide for common issues, error messages, and debugging techniques.

## General Troubleshooting

### Enable Debug Logging

First step for any issue - enable debug logging:

```bash
export SYNACKLAB_LOG_LEVEL=debug
synacklab <command>
```

### Check Version and Help

Verify installation and get help:

```bash
# Check version
synacklab --version

# Get command help
synacklab --help
synacklab auth --help
synacklab github --help
```

### Validate Configuration

Check configuration file syntax and values:

```bash
# Test configuration loading
synacklab init --dry-run

# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('~/.synacklab/config.yaml'))"

# Check file permissions
ls -la ~/.synacklab/config.yaml
```

## Installation Issues

### Command Not Found

**Error:** `synacklab: command not found`

**Solutions:**

1. **Check if binary exists:**
   ```bash
   ls -la ./bin/synacklab
   ```

2. **Add to PATH:**
   ```bash
   export PATH="$PWD/bin:$PATH"
   # Or install globally
   make install
   ```

3. **Use full path:**
   ```bash
   ./bin/synacklab --help
   ```

### Permission Denied

**Error:** `permission denied: ./bin/synacklab`

**Solutions:**

1. **Make executable:**
   ```bash
   chmod +x ./bin/synacklab
   ```

2. **Check file ownership:**
   ```bash
   ls -la ./bin/synacklab
   chown $USER ./bin/synacklab
   ```

### Build Failures

**Error:** Build fails with Go errors

**Solutions:**

1. **Check Go version:**
   ```bash
   go version
   # Requires Go 1.21+
   ```

2. **Clean and rebuild:**
   ```bash
   make clean
   make deps
   make build
   ```

3. **Check dependencies:**
   ```bash
   go mod tidy
   go mod download
   ```

## Configuration Issues

### Configuration File Not Found

**Error:** `failed to load configuration: file not found`

**Solutions:**

1. **Create configuration:**
   ```bash
   synacklab init
   ```

2. **Check file location:**
   ```bash
   ls -la ~/.synacklab/config.yaml
   ```

3. **Use custom config:**
   ```bash
   synacklab auth aws-login --config /path/to/config.yaml
   ```

### Invalid YAML Syntax

**Error:** `yaml: line X: found character that cannot start any token`

**Solutions:**

1. **Validate YAML:**
   ```bash
   python -c "import yaml; yaml.safe_load(open('~/.synacklab/config.yaml'))"
   ```

2. **Check indentation:**
   - Use spaces, not tabs
   - Consistent indentation (2 or 4 spaces)

3. **Check special characters:**
   - Quote strings with special characters
   - Escape quotes within strings

### Missing Required Fields

**Error:** `aws.sso.start_url is required`

**Solutions:**

1. **Add required fields:**
   ```yaml
   aws:
     sso:
       start_url: "https://company.awsapps.com/start"
       region: "us-east-1"
   ```

2. **Use environment variables:**
   ```bash
   export SYNACKLAB_AWS_SSO_START_URL="https://company.awsapps.com/start"
   ```

## AWS SSO Issues

### Authentication Timeout

**Error:** `authentication timed out after 300 seconds`

**Solutions:**

1. **Increase timeout:**
   ```bash
   synacklab auth aws-login --timeout 600
   ```

2. **Check network connectivity:**
   ```bash
   curl -I https://device.sso.us-east-1.amazonaws.com/
   ```

3. **Complete authorization quickly:**
   - Keep browser ready
   - Complete device authorization promptly

### Invalid SSO Configuration

**Error:** `invalid SSO start URL` or `invalid region`

**Solutions:**

1. **Verify SSO URL format:**
   ```bash
   # Correct format
   https://company.awsapps.com/start
   https://d-1234567890.awsapps.com/start
   ```

2. **Check region:**
   ```bash
   # Valid AWS regions
   us-east-1, us-west-2, eu-west-1, etc.
   ```

3. **Test SSO portal:**
   ```bash
   curl -I "https://your-company.awsapps.com/start"
   ```

### No Profiles Found

**Error:** `No profiles found in AWS SSO`

**Solutions:**

1. **Check SSO permissions:**
   - Contact AWS administrator
   - Verify account assignments in AWS SSO

2. **Test AWS CLI:**
   ```bash
   aws sso login --sso-start-url https://company.awsapps.com/start
   aws sts get-caller-identity
   ```

3. **Check specific region:**
   ```bash
   aws sso list-accounts --access-token <token>
   ```

### Profile Sync Issues

**Error:** `failed to update AWS config`

**Solutions:**

1. **Check file permissions:**
   ```bash
   ls -la ~/.aws/config
   chmod 644 ~/.aws/config
   ```

2. **Check directory permissions:**
   ```bash
   ls -la ~/.aws/
   chmod 755 ~/.aws/
   ```

3. **Backup and recreate:**
   ```bash
   cp ~/.aws/config ~/.aws/config.backup
   synacklab auth sync --reset
   ```

### Session Expired

**Error:** `session expired` or `invalid credentials`

**Solutions:**

1. **Re-authenticate:**
   ```bash
   synacklab auth aws-login
   ```

2. **Clear cached credentials:**
   ```bash
   rm -rf ~/.synacklab/cache/
   synacklab auth aws-login
   ```

3. **Check session status:**
   ```bash
   aws sts get-caller-identity
   ```

## Kubernetes Issues

### No EKS Clusters Found

**Error:** `No EKS clusters found in the specified region(s)`

**Solutions:**

1. **Check specific region:**
   ```bash
   synacklab auth eks-config --region us-east-1
   aws eks list-clusters --region us-east-1
   ```

2. **Verify AWS permissions:**
   ```bash
   aws iam get-user
   aws sts get-caller-identity
   ```

3. **Check EKS permissions:**
   ```bash
   aws eks describe-cluster --name cluster-name --region us-east-1
   ```

### Kubeconfig Permission Errors

**Error:** `failed to create .kube directory` or `permission denied`

**Solutions:**

1. **Check directory permissions:**
   ```bash
   ls -la ~/.kube/
   chmod 755 ~/.kube/
   ```

2. **Check file permissions:**
   ```bash
   ls -la ~/.kube/config
   chmod 644 ~/.kube/config
   ```

3. **Create directory:**
   ```bash
   mkdir -p ~/.kube
   ```

### Context Switch Failures

**Error:** `context not found` or `failed to switch context`

**Solutions:**

1. **List available contexts:**
   ```bash
   kubectl config get-contexts
   synacklab auth eks-ctx --list
   ```

2. **Validate kubeconfig:**
   ```bash
   kubectl config view
   ```

3. **Test context manually:**
   ```bash
   kubectl config use-context context-name
   kubectl get nodes
   ```

### EKS Authentication Errors

**Error:** `error: You must be logged in to the server (Unauthorized)`

**Solutions:**

1. **Update AWS credentials:**
   ```bash
   synacklab auth aws-login
   ```

2. **Test EKS token:**
   ```bash
   aws eks get-token --cluster-name cluster-name --region us-east-1
   ```

3. **Check cluster access:**
   ```bash
   aws eks describe-cluster --name cluster-name --region us-east-1
   ```

4. **Verify RBAC permissions:**
   ```bash
   kubectl auth whoami
   kubectl auth can-i get pods
   ```

## GitHub Issues

### Authentication Errors

**Error:** `GitHub authentication failed` or `401 Unauthorized`

**Solutions:**

1. **Check token:**
   ```bash
   echo $GITHUB_TOKEN
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user
   ```

2. **Verify token scopes:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" -I https://api.github.com/user
   # Check X-OAuth-Scopes header
   ```

3. **Required scopes:**
   - `repo` - Full control of private repositories
   - `admin:org` - Full control of orgs and teams
   - `admin:repo_hook` - Full control of repository hooks

### Repository Not Found

**Error:** `repository not found` or `404 Not Found`

**Solutions:**

1. **Check repository name:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/repos/owner/repo
   ```

2. **Verify owner:**
   ```bash
   # Check organization membership
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user/orgs
   ```

3. **Check permissions:**
   ```bash
   # List accessible repositories
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user/repos
   ```

### Validation Failures

**Error:** `user not found` or `team not found`

**Solutions:**

1. **Check user existence:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/users/username
   ```

2. **Check team existence:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/orgs/org/teams
   ```

3. **Verify organization context:**
   ```bash
   synacklab github validate repo.yaml --owner myorg
   ```

### Rate Limiting

**Error:** `rate limit exceeded` or `403 Forbidden`

**Solutions:**

1. **Check rate limit status:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/rate_limit
   ```

2. **Wait for reset:**
   - Check `X-RateLimit-Reset` header
   - Wait until reset time

3. **Use smaller batches:**
   ```bash
   synacklab github apply multi-repos.yaml --repos repo1,repo2
   ```

### Permission Errors

**Error:** `insufficient permissions` or `403 Forbidden`

**Solutions:**

1. **Check repository permissions:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/repos/owner/repo
   ```

2. **Verify admin access:**
   - Repository admin required for branch protection
   - Organization admin required for team management

3. **Check token scopes:**
   ```bash
   curl -H "Authorization: token $GITHUB_TOKEN" -I https://api.github.com/user
   ```

## Network Issues

### Connection Timeouts

**Error:** `connection timeout` or `network unreachable`

**Solutions:**

1. **Check internet connectivity:**
   ```bash
   ping google.com
   curl -I https://api.github.com
   ```

2. **Increase timeout:**
   ```bash
   export SYNACKLAB_TIMEOUT=600
   synacklab auth aws-login --timeout 600
   ```

3. **Check proxy settings:**
   ```bash
   echo $HTTP_PROXY
   echo $HTTPS_PROXY
   ```

### Proxy Issues

**Error:** `proxy connection failed`

**Solutions:**

1. **Configure proxy:**
   ```yaml
   app:
     proxy:
       http: "http://proxy.company.com:8080"
       https: "https://proxy.company.com:8080"
       no_proxy: "localhost,127.0.0.1"
   ```

2. **Test proxy:**
   ```bash
   curl --proxy http://proxy.company.com:8080 https://api.github.com
   ```

3. **Bypass proxy for specific hosts:**
   ```bash
   export NO_PROXY="localhost,127.0.0.1,.company.com"
   ```

### TLS/SSL Issues

**Error:** `certificate verification failed` or `TLS handshake failed`

**Solutions:**

1. **Check certificate:**
   ```bash
   openssl s_client -connect api.github.com:443
   ```

2. **Update CA certificates:**
   ```bash
   # macOS
   brew install ca-certificates
   
   # Linux
   sudo apt-get update && sudo apt-get install ca-certificates
   ```

3. **Configure custom CA bundle:**
   ```yaml
   app:
     tls:
       ca_bundle: "/path/to/ca-bundle.pem"
   ```

## Performance Issues

### Slow Operations

**Issue:** Commands take too long to complete

**Solutions:**

1. **Use specific regions:**
   ```bash
   synacklab auth eks-config --region us-east-1
   ```

2. **Enable caching:**
   ```yaml
   app:
     cache:
       enabled: true
       ttl: 3600
   ```

3. **Use selective operations:**
   ```bash
   synacklab github apply multi-repos.yaml --repos critical-repo1,critical-repo2
   ```

### Memory Issues

**Issue:** High memory usage or out of memory errors

**Solutions:**

1. **Process repositories in batches:**
   ```bash
   synacklab github apply multi-repos.yaml --repos batch1
   synacklab github apply multi-repos.yaml --repos batch2
   ```

2. **Clear cache:**
   ```bash
   rm -rf ~/.synacklab/cache/
   ```

3. **Reduce concurrent operations:**
   - Process one region at a time
   - Use smaller repository batches

## Debug Techniques

### Verbose Logging

Enable maximum verbosity:

```bash
export SYNACKLAB_LOG_LEVEL=debug
synacklab auth aws-login 2>&1 | tee debug.log
```

### Network Debugging

Debug network requests:

```bash
# Enable HTTP debugging
export SYNACKLAB_HTTP_DEBUG=true

# Use curl for manual testing
curl -v -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user
```

### Configuration Debugging

Debug configuration loading:

```bash
# Test configuration parsing
synacklab init --dry-run

# Check environment variables
env | grep SYNACKLAB | sort
```

### File System Debugging

Check file permissions and paths:

```bash
# Check all relevant files
ls -la ~/.synacklab/
ls -la ~/.aws/
ls -la ~/.kube/

# Check disk space
df -h ~/.synacklab/
```

## Getting Help

### Command Help

Use built-in help:

```bash
synacklab --help
synacklab auth --help
synacklab github apply --help
```

### Log Analysis

Analyze logs for patterns:

```bash
# Save debug output
synacklab auth aws-login 2>&1 | tee synacklab-debug.log

# Search for errors
grep -i error synacklab-debug.log
grep -i "failed" synacklab-debug.log
```

### System Information

Gather system information for support:

```bash
# System info
uname -a
go version
synacklab --version

# Configuration info
cat ~/.synacklab/config.yaml
ls -la ~/.synacklab/
```

### Common Error Patterns

**Pattern:** `failed to load configuration`
- Check file existence and permissions
- Validate YAML syntax
- Check environment variables

**Pattern:** `authentication failed`
- Verify credentials and tokens
- Check network connectivity
- Test with curl/AWS CLI

**Pattern:** `permission denied`
- Check file/directory permissions
- Verify API token scopes
- Test with minimal operations

**Pattern:** `timeout`
- Increase timeout values
- Check network connectivity
- Use smaller batch sizes

## Recovery Procedures

### Reset Configuration

Start fresh with configuration:

```bash
# Backup existing config
cp ~/.synacklab/config.yaml ~/.synacklab/config.yaml.backup

# Create new config
synacklab init

# Restore custom settings manually
```

### Clear All Caches

Remove all cached data:

```bash
# Clear Synacklab cache
rm -rf ~/.synacklab/cache/

# Clear AWS CLI cache
rm -rf ~/.aws/cli/cache/
rm -rf ~/.aws/sso/cache/

# Re-authenticate
synacklab auth aws-login
```

### Reset AWS Configuration

Start fresh with AWS profiles:

```bash
# Backup AWS config
cp ~/.aws/config ~/.aws/config.backup

# Reset profiles
synacklab auth sync --reset
```

### Reset Kubernetes Configuration

Start fresh with kubeconfig:

```bash
# Backup kubeconfig
cp ~/.kube/config ~/.kube/config.backup

# Rediscover clusters
synacklab auth eks-config
```

## Prevention Tips

### Regular Maintenance

1. **Update regularly:**
   ```bash
   git pull origin main
   make clean && make build
   ```

2. **Rotate tokens:**
   - GitHub tokens: Every 90 days
   - AWS SSO sessions: As per policy

3. **Clean caches:**
   ```bash
   rm -rf ~/.synacklab/cache/
   ```

### Best Practices

1. **Use version control for configurations**
2. **Test changes with dry-run mode**
3. **Monitor rate limits and quotas**
4. **Keep backups of working configurations**
5. **Use environment variables for secrets**

### Monitoring

1. **Check authentication status regularly**
2. **Monitor AWS SSO session expiration**
3. **Validate configurations before applying**
4. **Test critical workflows periodically**

## Next Steps

- [Review configuration reference](config-reference.md)
- [Check command reference](commands.md)
- [Browse examples](examples.md)
- [Read development guide](development.md)