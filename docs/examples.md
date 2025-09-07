# Examples and Use Cases

Real-world examples and common use cases for Synacklab CLI across different scenarios and environments.

## Quick Reference

### Daily Workflows
- [AWS SSO Authentication](#aws-sso-authentication)
- [Kubernetes Context Switching](#kubernetes-context-switching)
- [GitHub Repository Management](#github-repository-management)

### Team Scenarios
- [Multi-Account AWS Setup](#multi-account-aws-setup)
- [Microservices Repository Management](#microservices-repository-management)
- [CI/CD Integration](#cicd-integration)

### Advanced Use Cases
- [Enterprise Configuration](#enterprise-configuration)
- [Automation Scripts](#automation-scripts)
- [Multi-Environment Management](#multi-environment-management)

## AWS SSO Authentication

### Basic Daily Authentication

```bash
#!/bin/bash
# daily-aws-setup.sh - Daily AWS authentication workflow

echo "🚀 Setting up AWS environment for the day..."

# Authenticate with AWS SSO (uses cached session if valid)
synacklab auth aws-login

# Sync any new profiles from SSO
synacklab auth sync

# Set production as default for the day
echo "Setting production as default profile..."
echo "1" | synacklab auth aws-config  # Assumes production is first option

# Verify setup
echo "✅ AWS setup complete!"
aws sts get-caller-identity
```

### Multi-Region Profile Setup

```bash
#!/bin/bash
# multi-region-setup.sh - Set up profiles for multiple regions

# Configuration for different regions
declare -A REGIONS=(
    ["production"]="us-east-1"
    ["staging"]="us-west-2"
    ["development"]="eu-west-1"
)

echo "🌍 Setting up multi-region AWS profiles..."

# Authenticate once
synacklab auth aws-login

# Sync profiles
synacklab auth sync

# Test access to each region
for env in "${!REGIONS[@]}"; do
    region="${REGIONS[$env]}"
    echo "Testing $env environment in $region..."
    
    # Set profile for environment
    export AWS_PROFILE="${env}-administratoraccess"
    
    # Test access
    if aws sts get-caller-identity --region "$region" >/dev/null 2>&1; then
        echo "✅ $env ($region) - Access confirmed"
    else
        echo "❌ $env ($region) - Access failed"
    fi
done

echo "Multi-region setup complete!"
```

### Automated Profile Rotation

```bash
#!/bin/bash
# profile-rotation.sh - Rotate between AWS profiles for different tasks

PROFILES=(
    "production-administratoraccess"
    "staging-poweruseraccess"
    "development-poweruseraccess"
)

TASKS=(
    "Check production health"
    "Deploy to staging"
    "Run development tests"
)

echo "🔄 AWS Profile Rotation Workflow"

for i in "${!PROFILES[@]}"; do
    profile="${PROFILES[$i]}"
    task="${TASKS[$i]}"
    
    echo ""
    echo "📋 Task: $task"
    echo "🔑 Using profile: $profile"
    
    export AWS_PROFILE="$profile"
    
    # Verify profile works
    if aws sts get-caller-identity >/dev/null 2>&1; then
        echo "✅ Profile active: $(aws sts get-caller-identity --query 'Account' --output text)"
        
        # Simulate task-specific work
        case "$i" in
            0) echo "   Checking production resources..." ;;
            1) echo "   Deploying to staging environment..." ;;
            2) echo "   Running development tests..." ;;
        esac
    else
        echo "❌ Profile authentication failed"
    fi
done
```

## Kubernetes Context Switching

### EKS Cluster Discovery and Setup

```bash
#!/bin/bash
# eks-setup.sh - Complete EKS cluster setup

echo "🔍 Discovering and configuring EKS clusters..."

# Authenticate with AWS first
synacklab auth aws-login

# Discover clusters in primary regions
REGIONS=("us-east-1" "us-west-2" "eu-west-1")

for region in "${REGIONS[@]}"; do
    echo "Discovering clusters in $region..."
    synacklab auth eks-config --region "$region"
done

# List all available contexts
echo ""
echo "📋 Available Kubernetes contexts:"
synacklab auth eks-ctx --list

# Interactive context selection
echo ""
echo "🎯 Select your working cluster:"
synacklab auth eks-ctx

# Verify cluster access
echo ""
echo "✅ Testing cluster access..."
kubectl cluster-info
kubectl get nodes
```

### Multi-Cluster Workflow

```bash
#!/bin/bash
# multi-cluster-workflow.sh - Work across multiple EKS clusters

CLUSTERS=(
    "production-cluster-us-east-1"
    "staging-cluster-us-west-2"
    "development-cluster-eu-west-1"
)

NAMESPACES=(
    "default"
    "staging"
    "development"
)

echo "🚀 Multi-cluster Kubernetes workflow"

for i in "${!CLUSTERS[@]}"; do
    cluster="${CLUSTERS[$i]}"
    namespace="${NAMESPACES[$i]}"
    
    echo ""
    echo "🎯 Switching to $cluster"
    
    # Switch context
    kubectl config use-context "$cluster"
    
    # Set namespace
    kubectl config set-context --current --namespace="$namespace"
    
    # Verify access and show cluster info
    echo "📊 Cluster: $(kubectl config current-context)"
    echo "📦 Namespace: $(kubectl config view --minify --output 'jsonpath={..namespace}')"
    echo "🏷️  Nodes: $(kubectl get nodes --no-headers | wc -l)"
    
    # Example operations
    kubectl get pods --no-headers 2>/dev/null | head -3
done
```

### Kubernetes Development Workflow

```bash
#!/bin/bash
# k8s-dev-workflow.sh - Development workflow with multiple clusters

echo "🛠️  Kubernetes Development Workflow"

# Function to run command on specific cluster
run_on_cluster() {
    local cluster=$1
    local command=$2
    
    echo "🎯 Running on $cluster: $command"
    kubectl config use-context "$cluster"
    eval "$command"
}

# Development workflow
echo "1. Testing on development cluster..."
run_on_cluster "development-cluster-eu-west-1" "kubectl apply -f ./k8s/dev/"

echo ""
echo "2. Promoting to staging cluster..."
run_on_cluster "staging-cluster-us-west-2" "kubectl apply -f ./k8s/staging/"

echo ""
echo "3. Checking production cluster status..."
run_on_cluster "production-cluster-us-east-1" "kubectl get deployments"

# Return to development context
kubectl config use-context "development-cluster-eu-west-1"
echo "✅ Returned to development context"
```

## GitHub Repository Management

### Single Repository Setup

```yaml
# examples/production-api.yaml
name: production-api
description: "Production API service with comprehensive protection"
private: true

topics:
  - production
  - api
  - golang
  - microservice

features:
  issues: true
  wiki: true
  projects: true
  discussions: false

# Comprehensive branch protection
branch_protection:
  - pattern: "main"
    required_status_checks:
      - "ci/build"
      - "ci/test"
      - "security/scan"
      - "quality/sonar"
    require_up_to_date: true
    required_reviews: 2
    dismiss_stale_reviews: true
    require_code_owner_review: true
    restrict_pushes:
      - "admin-team"
      - "release-team"

  - pattern: "release/*"
    required_status_checks:
      - "ci/build"
      - "ci/test"
    required_reviews: 1
    require_code_owner_review: true

# Team-based access control
teams:
  - team: "backend-team"
    permission: "write"
  - team: "devops-team"
    permission: "admin"
  - team: "security-team"
    permission: "read"

# Production webhooks
webhooks:
  - url: "https://ci.company.com/webhook/production"
    events: ["push", "pull_request", "release"]
    secret: "${WEBHOOK_SECRET_PROD}"
    active: true
    
  - url: "https://monitoring.company.com/github-webhook"
    events: ["push", "issues", "pull_request"]
    secret: "${WEBHOOK_SECRET_MONITORING}"
    active: true
```

```bash
# Apply production repository configuration
synacklab github validate examples/production-api.yaml --owner mycompany
synacklab github apply examples/production-api.yaml --owner mycompany --dry-run
synacklab github apply examples/production-api.yaml --owner mycompany
```

### Microservices Repository Management

```yaml
# examples/microservices-team.yaml
version: "1.0"

# Global defaults for all microservices
defaults:
  private: true
  
  topics:
    - microservice
    - golang
    - kubernetes
  
  features:
    issues: true
    wiki: false
    projects: true
    discussions: false
  
  # Standard branch protection for all services
  branch_protection:
    - pattern: "main"
      required_status_checks:
        - "ci/build"
        - "ci/test"
        - "security/scan"
      required_reviews: 2
      require_code_owner_review: true
      dismiss_stale_reviews: true
  
  # Standard team access
  teams:
    - team: "microservices-team"
      permission: "write"
    - team: "platform-team"
      permission: "admin"
    - team: "security-team"
      permission: "read"
  
  # Standard webhooks
  webhooks:
    - url: "https://ci.company.com/webhook/microservices"
      events: ["push", "pull_request"]
      secret: "${WEBHOOK_SECRET_CI}"
      active: true

# Individual microservices
repositories:
  - name: "user-service"
    description: "User management and authentication service"
    topics: [users, authentication, jwt]
    
  - name: "payment-service"
    description: "Payment processing and billing service"
    topics: [payments, billing, stripe]
    
    # Additional security for payment service
    branch_protection:
      - pattern: "main"
        required_status_checks:
          - "ci/build"
          - "ci/test"
          - "security/scan"
          - "compliance/pci"
        required_reviews: 3  # Override: more reviews for payments
        
  - name: "notification-service"
    description: "Email and SMS notification service"
    topics: [notifications, email, sms]
    
  - name: "analytics-service"
    description: "User analytics and reporting service"
    topics: [analytics, reporting, data]
    
  - name: "gateway-service"
    description: "API gateway and routing service"
    topics: [gateway, routing, proxy]
    
    # Additional collaborator for gateway team
    collaborators:
      - username: "gateway-lead"
        permission: "admin"
```

```bash
# Microservices management workflow
echo "🚀 Managing microservices repositories..."

# Validate all repositories
synacklab github validate examples/microservices-team.yaml --owner mycompany

# Preview changes for all repositories
synacklab github apply examples/microservices-team.yaml --owner mycompany --dry-run

# Apply to critical services first
synacklab github apply examples/microservices-team.yaml --owner mycompany \
  --repos "user-service,payment-service,gateway-service"

# Apply to remaining services
synacklab github apply examples/microservices-team.yaml --owner mycompany \
  --repos "notification-service,analytics-service"

echo "✅ Microservices repositories configured!"
```

### Repository Template Management

```yaml
# examples/repository-template.yaml
name: "service-template"
description: "Template repository for new microservices"
private: false  # Template should be accessible

topics:
  - template
  - microservice
  - golang
  - kubernetes

features:
  issues: true
  wiki: true
  projects: true
  discussions: true

# Template branch protection (will be inherited)
branch_protection:
  - pattern: "main"
    required_status_checks:
      - "ci/build"
      - "ci/test"
      - "security/scan"
    required_reviews: 2
    require_code_owner_review: true
    dismiss_stale_reviews: true

# Template team access
teams:
  - team: "platform-team"
    permission: "admin"
  - team: "developers"
    permission: "read"

# Template webhooks
webhooks:
  - url: "https://ci.company.com/webhook/template"
    events: ["push", "pull_request"]
    secret: "${WEBHOOK_SECRET_TEMPLATE}"
    active: true
```

## Multi-Account AWS Setup

### Enterprise Multi-Account Configuration

```yaml
# ~/.synacklab/enterprise-config.yaml
aws:
  sso:
    start_url: "https://enterprise.awsapps.com/start"
    region: "us-east-1"
    session_timeout: 7200
    profile_prefix: "enterprise-"

github:
  token: "${GITHUB_TOKEN}"
  organization: "enterprise-corp"
  api_url: "https://github.enterprise.com/api/v3"
  web_url: "https://github.enterprise.com"

app:
  log_level: "info"
  timeout: 600
  proxy:
    http: "http://proxy.enterprise.com:8080"
    https: "https://proxy.enterprise.com:8080"
    no_proxy: "localhost,127.0.0.1,.enterprise.com"
```

```bash
#!/bin/bash
# enterprise-setup.sh - Enterprise multi-account setup

export SYNACKLAB_CONFIG=~/.synacklab/enterprise-config.yaml

echo "🏢 Enterprise AWS Multi-Account Setup"

# Authenticate with enterprise SSO
synacklab auth aws-login

# Sync all enterprise accounts
synacklab auth sync

# Set up different profiles for different roles
PROFILES=(
    "enterprise-production-administratoraccess:Production Admin"
    "enterprise-staging-poweruseraccess:Staging Power User"
    "enterprise-development-poweruseraccess:Development Power User"
    "enterprise-security-readonlyaccess:Security Read-Only"
)

echo ""
echo "📋 Available Enterprise Profiles:"
for profile_info in "${PROFILES[@]}"; do
    IFS=':' read -r profile description <<< "$profile_info"
    echo "  • $profile - $description"
done

# Test each profile
echo ""
echo "🧪 Testing profile access..."
for profile_info in "${PROFILES[@]}"; do
    IFS=':' read -r profile description <<< "$profile_info"
    
    export AWS_PROFILE="$profile"
    if aws sts get-caller-identity >/dev/null 2>&1; then
        account=$(aws sts get-caller-identity --query 'Account' --output text)
        echo "✅ $description: Account $account"
    else
        echo "❌ $description: Access failed"
    fi
done

echo ""
echo "🎯 Select your primary working profile:"
synacklab auth aws-config
```

### Cross-Account Resource Access

```bash
#!/bin/bash
# cross-account-access.sh - Access resources across multiple AWS accounts

ACCOUNTS=(
    "production:123456789012:enterprise-production-administratoraccess"
    "staging:234567890123:enterprise-staging-poweruseraccess"
    "development:345678901234:enterprise-development-poweruseraccess"
)

echo "🔄 Cross-Account Resource Access"

for account_info in "${ACCOUNTS[@]}"; do
    IFS=':' read -r env account_id profile <<< "$account_info"
    
    echo ""
    echo "🎯 Accessing $env environment (Account: $account_id)"
    
    export AWS_PROFILE="$profile"
    
    # Verify access
    current_account=$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null)
    
    if [[ "$current_account" == "$account_id" ]]; then
        echo "✅ Successfully switched to $env"
        
        # Example operations for each environment
        case "$env" in
            "production")
                echo "   📊 Production S3 buckets: $(aws s3 ls | wc -l)"
                echo "   🖥️  Production EC2 instances: $(aws ec2 describe-instances --query 'Reservations[].Instances[?State.Name==`running`]' --output text | wc -l)"
                ;;
            "staging")
                echo "   🧪 Staging EKS clusters: $(aws eks list-clusters --query 'clusters' --output text | wc -w)"
                echo "   📦 Staging RDS instances: $(aws rds describe-db-instances --query 'DBInstances[?DBInstanceStatus==`available`]' --output text | wc -l)"
                ;;
            "development")
                echo "   🛠️  Development Lambda functions: $(aws lambda list-functions --query 'Functions' --output text | wc -l)"
                echo "   📋 Development CloudFormation stacks: $(aws cloudformation list-stacks --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE --query 'StackSummaries' --output text | wc -l)"
                ;;
        esac
    else
        echo "❌ Failed to access $env environment"
    fi
done
```

## CI/CD Integration

### GitHub Actions Integration

```yaml
# .github/workflows/synacklab-management.yml
name: Repository Management with Synacklab

on:
  push:
    branches: [main]
    paths:
      - 'repositories/**/*.yaml'
      - '.github/workflows/synacklab-management.yml'
  pull_request:
    branches: [main]
    paths:
      - 'repositories/**/*.yaml'

jobs:
  validate:
    name: Validate Repository Configurations
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
          
      - name: Install Synacklab
        run: |
          git clone https://github.com/company/synacklab.git
          cd synacklab
          make build
          sudo cp bin/synacklab /usr/local/bin/
          
      - name: Validate configurations
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          echo "🔍 Validating repository configurations..."
          
          for config in repositories/*.yaml; do
            echo "Validating $config..."
            synacklab github validate "$config" --owner ${{ github.repository_owner }}
          done
          
          echo "✅ All configurations validated successfully"

  apply:
    name: Apply Repository Configurations
    runs-on: ubuntu-latest
    needs: validate
    if: github.ref == 'refs/heads/main'
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
          
      - name: Install Synacklab
        run: |
          git clone https://github.com/company/synacklab.git
          cd synacklab
          make build
          sudo cp bin/synacklab /usr/local/bin/
          
      - name: Apply configurations
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          echo "🚀 Applying repository configurations..."
          
          # Apply configurations in order of importance
          CRITICAL_REPOS=(
            "repositories/production-api.yaml"
            "repositories/user-service.yaml"
            "repositories/payment-service.yaml"
          )
          
          # Apply critical repositories first
          for config in "${CRITICAL_REPOS[@]}"; do
            if [[ -f "$config" ]]; then
              echo "Applying critical config: $config"
              synacklab github apply "$config" --owner ${{ github.repository_owner }}
            fi
          done
          
          # Apply remaining configurations
          for config in repositories/*.yaml; do
            # Skip if already applied
            skip=false
            for critical in "${CRITICAL_REPOS[@]}"; do
              if [[ "$config" == "$critical" ]]; then
                skip=true
                break
              fi
            done
            
            if [[ "$skip" == "false" ]]; then
              echo "Applying config: $config"
              synacklab github apply "$config" --owner ${{ github.repository_owner }}
            fi
          done
          
          echo "✅ All configurations applied successfully"
```

### Jenkins Pipeline Integration

```groovy
// Jenkinsfile - Synacklab integration pipeline
pipeline {
    agent any
    
    environment {
        GITHUB_TOKEN = credentials('github-token')
        SYNACKLAB_CONFIG = '/var/jenkins_home/synacklab-config.yaml'
    }
    
    stages {
        stage('Setup') {
            steps {
                script {
                    // Install Synacklab
                    sh '''
                        if ! command -v synacklab &> /dev/null; then
                            echo "Installing Synacklab..."
                            git clone https://github.com/company/synacklab.git /tmp/synacklab
                            cd /tmp/synacklab
                            make build
                            sudo cp bin/synacklab /usr/local/bin/
                        fi
                        
                        synacklab --version
                    '''
                }
            }
        }
        
        stage('AWS Setup') {
            steps {
                script {
                    sh '''
                        echo "🔐 Setting up AWS environment..."
                        synacklab auth aws-login
                        synacklab auth sync
                        
                        echo "🔍 Discovering EKS clusters..."
                        synacklab auth eks-config --region us-east-1
                        synacklab auth eks-config --region us-west-2
                    '''
                }
            }
        }
        
        stage('Validate Repositories') {
            steps {
                script {
                    sh '''
                        echo "🔍 Validating repository configurations..."
                        
                        for config in repositories/*.yaml; do
                            echo "Validating $config..."
                            synacklab github validate "$config" --owner ${GITHUB_ORG}
                        done
                    '''
                }
            }
        }
        
        stage('Apply Repositories') {
            when {
                branch 'main'
            }
            steps {
                script {
                    sh '''
                        echo "🚀 Applying repository configurations..."
                        
                        # Apply multi-repository configurations
                        for config in repositories/multi-*.yaml; do
                            if [[ -f "$config" ]]; then
                                echo "Applying multi-repo config: $config"
                                synacklab github apply "$config" --owner ${GITHUB_ORG}
                            fi
                        done
                        
                        # Apply single repository configurations
                        for config in repositories/single-*.yaml; do
                            if [[ -f "$config" ]]; then
                                echo "Applying single-repo config: $config"
                                synacklab github apply "$config" --owner ${GITHUB_ORG}
                            fi
                        done
                    '''
                }
            }
        }
        
        stage('Deploy to Kubernetes') {
            steps {
                script {
                    sh '''
                        echo "🚀 Deploying to Kubernetes clusters..."
                        
                        # Deploy to staging
                        synacklab auth eks-ctx --filter staging
                        kubectl apply -f k8s/staging/
                        
                        # Deploy to production (if main branch)
                        if [[ "${BRANCH_NAME}" == "main" ]]; then
                            synacklab auth eks-ctx --filter production
                            kubectl apply -f k8s/production/
                        fi
                    '''
                }
            }
        }
    }
    
    post {
        always {
            script {
                sh '''
                    echo "📊 Pipeline Summary:"
                    echo "  • AWS Profiles: $(aws configure list-profiles | wc -l)"
                    echo "  • Kubernetes Contexts: $(kubectl config get-contexts --no-headers | wc -l)"
                    echo "  • Current AWS Profile: ${AWS_PROFILE:-default}"
                    echo "  • Current K8s Context: $(kubectl config current-context)"
                '''
            }
        }
        
        failure {
            script {
                sh '''
                    echo "❌ Pipeline failed. Debugging information:"
                    echo "AWS Identity: $(aws sts get-caller-identity || echo 'AWS auth failed')"
                    echo "Kubernetes access: $(kubectl get nodes || echo 'K8s access failed')"
                '''
            }
        }
    }
}
```

## Enterprise Configuration

### Complete Enterprise Setup

```yaml
# ~/.synacklab/enterprise-complete.yaml
aws:
  sso:
    start_url: "https://enterprise-corp.awsapps.com/start"
    region: "us-east-1"
    session_timeout: 14400  # 4 hours for enterprise
    default_region: "us-east-1"
    default_output: "json"
    profile_prefix: "corp-"
    profile_template: "{account_name}-{role_name}"

github:
  token: "${GITHUB_TOKEN}"
  organization: "enterprise-corp"
  api_url: "https://github.enterprise-corp.com/api/v3"
  web_url: "https://github.enterprise-corp.com"
  timeout: 120
  user_agent: "EnterpriseCorp-Synacklab/1.0"
  
  rate_limit:
    enabled: true
    max_retries: 10
    backoff_factor: 3
    max_backoff: 600
  
  defaults:
    private: true
    auto_init: false

app:
  log_level: "info"
  timeout: 900
  color: true
  progress: true
  
  cache:
    enabled: true
    directory: "/enterprise/synacklab/cache"
    ttl: 7200
    max_size: "500MB"
  
  fuzzy:
    enabled: true
    case_sensitive: false
    algorithm: "fzf"
  
  proxy:
    http: "http://proxy.enterprise-corp.com:8080"
    https: "https://proxy.enterprise-corp.com:8080"
    no_proxy: "localhost,127.0.0.1,.enterprise-corp.com,.internal"
  
  tls:
    insecure_skip_verify: false
    ca_bundle: "/etc/ssl/certs/enterprise-ca-bundle.pem"
```

### Enterprise Team Workflow

```bash
#!/bin/bash
# enterprise-team-workflow.sh - Complete enterprise team workflow

export SYNACKLAB_CONFIG=~/.synacklab/enterprise-complete.yaml

echo "🏢 Enterprise Team Workflow Setup"
echo "=================================="

# Step 1: Authentication
echo ""
echo "1️⃣  Authenticating with Enterprise SSO..."
if synacklab auth aws-login; then
    echo "✅ AWS SSO authentication successful"
else
    echo "❌ AWS SSO authentication failed"
    exit 1
fi

# Step 2: Profile synchronization
echo ""
echo "2️⃣  Synchronizing AWS profiles..."
if synacklab auth sync; then
    profile_count=$(aws configure list-profiles | wc -l)
    echo "✅ Synchronized $profile_count AWS profiles"
else
    echo "❌ Profile synchronization failed"
    exit 1
fi

# Step 3: EKS cluster discovery
echo ""
echo "3️⃣  Discovering EKS clusters across regions..."
ENTERPRISE_REGIONS=("us-east-1" "us-west-2" "eu-west-1" "ap-southeast-1")

for region in "${ENTERPRISE_REGIONS[@]}"; do
    echo "   Scanning region: $region"
    synacklab auth eks-config --region "$region" >/dev/null 2>&1
done

cluster_count=$(kubectl config get-contexts --no-headers | wc -l)
echo "✅ Discovered $cluster_count EKS clusters"

# Step 4: Repository management setup
echo ""
echo "4️⃣  Setting up repository management..."

# Validate enterprise repository configurations
REPO_CONFIGS=(
    "repositories/enterprise/platform-services.yaml"
    "repositories/enterprise/business-applications.yaml"
    "repositories/enterprise/infrastructure.yaml"
)

for config in "${REPO_CONFIGS[@]}"; do
    if [[ -f "$config" ]]; then
        echo "   Validating: $config"
        if synacklab github validate "$config" --owner enterprise-corp >/dev/null 2>&1; then
            echo "   ✅ $config - Valid"
        else
            echo "   ❌ $config - Invalid"
        fi
    fi
done

# Step 5: Environment verification
echo ""
echo "5️⃣  Verifying environment access..."

# Test production access
export AWS_PROFILE="corp-production-administratoraccess"
if aws sts get-caller-identity >/dev/null 2>&1; then
    echo "✅ Production AWS access confirmed"
else
    echo "⚠️  Production AWS access not available"
fi

# Test Kubernetes access
if kubectl config use-context "production-cluster-us-east-1" >/dev/null 2>&1; then
    if kubectl get nodes >/dev/null 2>&1; then
        echo "✅ Production Kubernetes access confirmed"
    else
        echo "⚠️  Production Kubernetes access limited"
    fi
else
    echo "⚠️  Production Kubernetes context not available"
fi

# Step 6: Daily environment setup
echo ""
echo "6️⃣  Setting up daily working environment..."

# Set default AWS profile for the day
echo "   Setting default AWS profile..."
echo "1" | synacklab auth aws-config >/dev/null 2>&1

# Set default Kubernetes context
echo "   Setting default Kubernetes context..."
kubectl config use-context "development-cluster-us-east-1" >/dev/null 2>&1

echo ""
echo "🎉 Enterprise environment setup complete!"
echo ""
echo "📊 Environment Summary:"
echo "   • AWS Profiles: $(aws configure list-profiles | wc -l)"
echo "   • K8s Contexts: $(kubectl config get-contexts --no-headers | wc -l)"
echo "   • Current AWS: $(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null || echo 'Not set')"
echo "   • Current K8s: $(kubectl config current-context 2>/dev/null || echo 'Not set')"
echo ""
echo "💡 Ready for enterprise development workflow!"
```

## Automation Scripts

### Daily Developer Setup

```bash
#!/bin/bash
# daily-dev-setup.sh - Automated daily developer environment setup

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="$HOME/.synacklab/daily-setup.log"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Error handling
error_exit() {
    log "ERROR: $1"
    exit 1
}

log "🚀 Starting daily developer environment setup..."

# Check prerequisites
command -v synacklab >/dev/null 2>&1 || error_exit "Synacklab not installed"
command -v aws >/dev/null 2>&1 || error_exit "AWS CLI not installed"
command -v kubectl >/dev/null 2>&1 || error_exit "kubectl not installed"

# Step 1: AWS Authentication
log "1️⃣  Authenticating with AWS SSO..."
if synacklab auth aws-login --timeout 120; then
    log "✅ AWS authentication successful"
else
    error_exit "AWS authentication failed"
fi

# Step 2: Sync AWS profiles
log "2️⃣  Syncing AWS profiles..."
if synacklab auth sync; then
    profile_count=$(aws configure list-profiles | wc -l)
    log "✅ Synced $profile_count AWS profiles"
else
    error_exit "Profile sync failed"
fi

# Step 3: Set development profile as default
log "3️⃣  Setting development profile as default..."
if echo "development-poweruseraccess" | synacklab auth aws-config >/dev/null 2>&1; then
    log "✅ Development profile set as default"
else
    log "⚠️  Could not set development profile, using interactive selection"
    synacklab auth aws-config
fi

# Step 4: Update EKS clusters
log "4️⃣  Updating EKS cluster configurations..."
DEV_REGIONS=("us-east-1" "us-west-2")

for region in "${DEV_REGIONS[@]}"; do
    log "   Updating clusters in $region..."
    synacklab auth eks-config --region "$region" >/dev/null 2>&1 || log "   No clusters found in $region"
done

cluster_count=$(kubectl config get-contexts --no-headers | wc -l)
log "✅ Updated $cluster_count Kubernetes contexts"

# Step 5: Set development cluster as default
log "5️⃣  Setting development cluster as default..."
if kubectl config use-context "development-cluster-us-east-1" >/dev/null 2>&1; then
    log "✅ Development cluster set as default"
else
    log "⚠️  Development cluster not available, using interactive selection"
    synacklab auth eks-ctx
fi

# Step 6: Verify environment
log "6️⃣  Verifying environment..."

# Test AWS access
current_account=$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null)
if [[ -n "$current_account" ]]; then
    log "✅ AWS access verified (Account: $current_account)"
else
    log "❌ AWS access verification failed"
fi

# Test Kubernetes access
if kubectl get nodes >/dev/null 2>&1; then
    node_count=$(kubectl get nodes --no-headers | wc -l)
    log "✅ Kubernetes access verified ($node_count nodes)"
else
    log "❌ Kubernetes access verification failed"
fi

# Step 7: Setup development tools
log "7️⃣  Setting up development tools..."

# Set kubectl namespace to development
kubectl config set-context --current --namespace=development >/dev/null 2>&1 || true

# Create useful aliases
cat > "$HOME/.synacklab/daily-aliases.sh" << 'EOF'
# Daily development aliases
alias k='kubectl'
alias kgp='kubectl get pods'
alias kgs='kubectl get services'
alias kgd='kubectl get deployments'
alias aws-dev='export AWS_PROFILE=development-poweruseraccess'
alias aws-staging='export AWS_PROFILE=staging-poweruseraccess'
alias slab='synacklab'
EOF

log "✅ Development tools configured"

# Step 8: Summary
log "8️⃣  Environment setup summary:"
log "   • AWS Profile: $(echo $AWS_PROFILE)"
log "   • AWS Account: $current_account"
log "   • K8s Context: $(kubectl config current-context 2>/dev/null || echo 'Not set')"
log "   • K8s Namespace: $(kubectl config view --minify --output 'jsonpath={..namespace}' 2>/dev/null || echo 'default')"

log ""
log "🎉 Daily developer environment setup complete!"
log "💡 Source aliases: source ~/.synacklab/daily-aliases.sh"
log "📝 Setup log: $LOG_FILE"
```

### Weekly Maintenance Script

```bash
#!/bin/bash
# weekly-maintenance.sh - Weekly Synacklab maintenance tasks

set -e

BACKUP_DIR="$HOME/.synacklab/backups/$(date +%Y%m%d)"
LOG_FILE="$HOME/.synacklab/maintenance.log"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "🔧 Starting weekly Synacklab maintenance..."

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Step 1: Backup configurations
log "1️⃣  Backing up configurations..."
cp ~/.synacklab/config.yaml "$BACKUP_DIR/config.yaml" 2>/dev/null || log "   No config file to backup"
cp ~/.aws/config "$BACKUP_DIR/aws-config" 2>/dev/null || log "   No AWS config to backup"
cp ~/.kube/config "$BACKUP_DIR/kube-config" 2>/dev/null || log "   No kubeconfig to backup"
log "✅ Configurations backed up to $BACKUP_DIR"

# Step 2: Clear old caches
log "2️⃣  Clearing old caches..."
if [[ -d ~/.synacklab/cache ]]; then
    find ~/.synacklab/cache -type f -mtime +7 -delete
    cache_size=$(du -sh ~/.synacklab/cache 2>/dev/null | cut -f1)
    log "✅ Cache cleaned (current size: $cache_size)"
else
    log "   No cache directory found"
fi

# Step 3: Update AWS profiles
log "3️⃣  Updating AWS profiles..."
if synacklab auth aws-login --timeout 60; then
    if synacklab auth sync; then
        profile_count=$(aws configure list-profiles | wc -l)
        log "✅ AWS profiles updated ($profile_count profiles)"
    else
        log "⚠️  Profile sync failed"
    fi
else
    log "⚠️  AWS authentication failed, skipping profile update"
fi

# Step 4: Update EKS clusters
log "4️⃣  Updating EKS cluster configurations..."
REGIONS=("us-east-1" "us-west-2" "eu-west-1")

for region in "${REGIONS[@]}"; do
    log "   Scanning region: $region"
    synacklab auth eks-config --region "$region" >/dev/null 2>&1 || log "   No clusters in $region"
done

cluster_count=$(kubectl config get-contexts --no-headers 2>/dev/null | wc -l)
log "✅ EKS clusters updated ($cluster_count contexts)"

# Step 5: Validate repository configurations
log "5️⃣  Validating repository configurations..."
config_count=0
valid_count=0

if [[ -d repositories ]]; then
    for config in repositories/*.yaml; do
        if [[ -f "$config" ]]; then
            ((config_count++))
            if synacklab github validate "$config" >/dev/null 2>&1; then
                ((valid_count++))
            else
                log "   ⚠️  Invalid config: $config"
            fi
        fi
    done
    log "✅ Repository validation complete ($valid_count/$config_count valid)"
else
    log "   No repository configurations found"
fi

# Step 6: Clean old backups
log "6️⃣  Cleaning old backups..."
if [[ -d ~/.synacklab/backups ]]; then
    find ~/.synacklab/backups -type d -mtime +30 -exec rm -rf {} + 2>/dev/null || true
    backup_count=$(find ~/.synacklab/backups -type d -maxdepth 1 | wc -l)
    log "✅ Old backups cleaned ($backup_count backups remaining)"
else
    log "   No backup directory found"
fi

# Step 7: Generate maintenance report
log "7️⃣  Generating maintenance report..."
cat > "$BACKUP_DIR/maintenance-report.txt" << EOF
Synacklab Weekly Maintenance Report
Generated: $(date)

System Information:
- Synacklab Version: $(synacklab --version 2>/dev/null || echo 'Unknown')
- AWS CLI Version: $(aws --version 2>/dev/null || echo 'Not installed')
- kubectl Version: $(kubectl version --client --short 2>/dev/null || echo 'Not installed')

Configuration Status:
- AWS Profiles: $(aws configure list-profiles 2>/dev/null | wc -l)
- Kubernetes Contexts: $(kubectl config get-contexts --no-headers 2>/dev/null | wc -l)
- Repository Configs: $config_count (Valid: $valid_count)

Cache Information:
- Cache Directory: ~/.synacklab/cache
- Cache Size: $(du -sh ~/.synacklab/cache 2>/dev/null | cut -f1 || echo 'N/A')

Backup Information:
- Backup Location: $BACKUP_DIR
- Total Backups: $backup_count

Recommendations:
$(if [[ $valid_count -lt $config_count ]]; then echo "- Review invalid repository configurations"; fi)
$(if [[ $(kubectl config get-contexts --no-headers 2>/dev/null | wc -l) -eq 0 ]]; then echo "- No Kubernetes contexts found, run EKS discovery"; fi)
$(if [[ $(aws configure list-profiles 2>/dev/null | wc -l) -eq 0 ]]; then echo "- No AWS profiles found, run profile sync"; fi)
EOF

log "✅ Maintenance report generated: $BACKUP_DIR/maintenance-report.txt"

log ""
log "🎉 Weekly maintenance complete!"
log "📊 Summary:"
log "   • Configurations backed up: $BACKUP_DIR"
log "   • AWS profiles: $(aws configure list-profiles 2>/dev/null | wc -l)"
log "   • K8s contexts: $(kubectl config get-contexts --no-headers 2>/dev/null | wc -l)"
log "   • Repository configs: $valid_count/$config_count valid"
log "📝 Full log: $LOG_FILE"
```

## Multi-Environment Management

### Environment-Specific Configurations

```bash
#!/bin/bash
# multi-env-setup.sh - Manage multiple environment configurations

ENVIRONMENTS=("development" "staging" "production")
BASE_CONFIG_DIR="$HOME/.synacklab/environments"

# Create environment-specific configurations
setup_environment_configs() {
    mkdir -p "$BASE_CONFIG_DIR"
    
    for env in "${ENVIRONMENTS[@]}"; do
        cat > "$BASE_CONFIG_DIR/$env-config.yaml" << EOF
aws:
  sso:
    start_url: "https://$env-company.awsapps.com/start"
    region: "us-east-1"
    profile_prefix: "$env-"
    session_timeout: $([[ "$env" == "production" ]] && echo "7200" || echo "3600")

github:
  token: "\${GITHUB_TOKEN}"
  organization: "$env-company"
  $([[ "$env" == "production" ]] && echo 'api_url: "https://github.company.com/api/v3"' || echo '')

app:
  log_level: "$([[ "$env" == "development" ]] && echo "debug" || echo "info")"
  timeout: $([[ "$env" == "production" ]] && echo "600" || echo "300")
  cache:
    enabled: $([[ "$env" == "development" ]] && echo "false" || echo "true")
EOF
        echo "✅ Created $env environment configuration"
    done
}

# Switch between environments
switch_environment() {
    local env=$1
    local config_file="$BASE_CONFIG_DIR/$env-config.yaml"
    
    if [[ ! -f "$config_file" ]]; then
        echo "❌ Configuration for $env environment not found"
        return 1
    fi
    
    export SYNACKLAB_CONFIG="$config_file"
    echo "🔄 Switched to $env environment"
    
    # Authenticate and setup for the environment
    echo "🔐 Authenticating with $env AWS SSO..."
    synacklab auth aws-login
    
    echo "📋 Syncing $env AWS profiles..."
    synacklab auth sync
    
    echo "🔍 Discovering $env EKS clusters..."
    synacklab auth eks-config --region us-east-1
    
    echo "✅ $env environment ready!"
}

# Main menu
main() {
    echo "🌍 Multi-Environment Management"
    echo "==============================="
    
    if [[ "$1" == "setup" ]]; then
        setup_environment_configs
        return
    fi
    
    if [[ -n "$1" ]] && [[ " ${ENVIRONMENTS[*]} " =~ " $1 " ]]; then
        switch_environment "$1"
        return
    fi
    
    echo "Available environments:"
    for i in "${!ENVIRONMENTS[@]}"; do
        echo "  $((i+1)). ${ENVIRONMENTS[$i]}"
    done
    
    echo ""
    read -p "Select environment (1-${#ENVIRONMENTS[@]}): " choice
    
    if [[ "$choice" =~ ^[1-${#ENVIRONMENTS[@]}]$ ]]; then
        env="${ENVIRONMENTS[$((choice-1))]}"
        switch_environment "$env"
    else
        echo "❌ Invalid selection"
        exit 1
    fi
}

# Usage: ./multi-env-setup.sh [setup|development|staging|production]
main "$@"
```

This comprehensive documentation provides real-world examples and use cases for Synacklab CLI across different scenarios. The examples cover everything from basic daily workflows to complex enterprise setups and automation scripts.

## Next Steps

- [Review command reference](commands.md)
- [Check configuration reference](config-reference.md)
- [Browse troubleshooting guide](troubleshooting.md)
- [Read development guide](development.md)