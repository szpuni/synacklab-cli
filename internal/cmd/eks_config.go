package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	eksRegion string
	dryRun    bool
)

var eksConfigCmd = &cobra.Command{
	Use:   "eks-config",
	Short: "Configure EKS cluster authentication",
	Long: `Discover EKS clusters in your AWS account and update ~/.kube/config with authentication.
This command will:
- List all EKS clusters in the specified region (or all regions if not specified)
- Add new clusters to your kubeconfig
- Update existing cluster configurations with current AWS data
- Support multiple AWS accounts when switching profiles`,
	RunE: runEKSConfig,
}

func init() {
	eksConfigCmd.Flags().StringVarP(&eksRegion, "region", "r", "", "AWS region to search for EKS clusters (searches all regions if not specified)")
	eksConfigCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
}

// EKSCluster represents an EKS cluster configuration
type EKSCluster struct {
	Name     string
	Region   string
	Endpoint string
	ARN      string
	Status   string
}

// KubeConfig represents the structure of a kubeconfig file
type KubeConfig struct {
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	CurrentContext string         `yaml:"current-context,omitempty"`
	Clusters       []KubeCluster  `yaml:"clusters"`
	Contexts       []KubeContext  `yaml:"contexts"`
	Users          []KubeUser     `yaml:"users"`
	Preferences    map[string]any `yaml:"preferences,omitempty"`
}

type KubeCluster struct {
	Name    string            `yaml:"name"`
	Cluster KubeClusterConfig `yaml:"cluster"`
}

type KubeClusterConfig struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
}

type KubeContext struct {
	Name    string            `yaml:"name"`
	Context KubeContextConfig `yaml:"context"`
}

type KubeContextConfig struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace,omitempty"`
}

type KubeUser struct {
	Name string       `yaml:"name"`
	User KubeUserExec `yaml:"user"`
}

type KubeUserExec struct {
	Exec KubeExecConfig `yaml:"exec"`
}

type KubeExecConfig struct {
	APIVersion string           `yaml:"apiVersion"`
	Command    string           `yaml:"command"`
	Args       []string         `yaml:"args"`
	Env        []KubeExecEnvVar `yaml:"env,omitempty"`
}

type KubeExecEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func runEKSConfig(_ *cobra.Command, _ []string) error {
	fmt.Println("🔍 Discovering EKS clusters...")

	// Get current AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Get regions to search
	regions, err := getRegionsToSearch(cfg)
	if err != nil {
		return fmt.Errorf("failed to determine regions to search: %w", err)
	}

	fmt.Printf("🌍 Searching for EKS clusters in %d region(s)...\n", len(regions))

	// Discover clusters across all regions
	var allClusters []EKSCluster
	for _, region := range regions {
		clusters, err := discoverEKSClusters(cfg, region)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to discover clusters in region %s: %v\n", region, err)
			continue
		}
		allClusters = append(allClusters, clusters...)
	}

	if len(allClusters) == 0 {
		fmt.Println("❌ No EKS clusters found in the specified region(s)")
		return nil
	}

	fmt.Printf("📋 Found %d EKS cluster(s):\n", len(allClusters))
	for _, cluster := range allClusters {
		fmt.Printf("  • %s (%s) - %s\n", cluster.Name, cluster.Region, cluster.Status)
	}

	if dryRun {
		fmt.Println("\n🔍 Dry run mode - no changes will be made")
		return nil
	}

	// Update kubeconfig
	err = updateKubeConfig(allClusters)
	if err != nil {
		return fmt.Errorf("failed to update kubeconfig: %w", err)
	}

	fmt.Printf("✅ Successfully updated kubeconfig with %d EKS cluster(s)\n", len(allClusters))
	return nil
}

func getRegionsToSearch(_ aws.Config) ([]string, error) {
	if eksRegion != "" {
		return []string{eksRegion}, nil
	}

	// If no region specified, use common AWS regions where EKS is available
	// In a production tool, you might want to dynamically fetch this list
	return []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2",
		"ca-central-1", "sa-east-1",
	}, nil
}

func discoverEKSClusters(cfg aws.Config, region string) ([]EKSCluster, error) {
	// Create EKS client for the specific region
	eksClient := eks.NewFromConfig(cfg, func(o *eks.Options) {
		o.Region = region
	})

	// List clusters
	listResp, err := eksClient.ListClusters(context.TODO(), &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters in region %s: %w", region, err)
	}

	var clusters []EKSCluster
	for _, clusterName := range listResp.Clusters {
		// Get cluster details
		describeResp, err := eksClient.DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to describe cluster %s: %v\n", clusterName, err)
			continue
		}

		cluster := EKSCluster{
			Name:     clusterName,
			Region:   region,
			Endpoint: *describeResp.Cluster.Endpoint,
			ARN:      *describeResp.Cluster.Arn,
			Status:   string(describeResp.Cluster.Status),
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func updateKubeConfig(clusters []EKSCluster) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	kubeDir := filepath.Join(homeDir, ".kube")
	configPath := filepath.Join(kubeDir, "config")

	// Create .kube directory if it doesn't exist
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	// Load existing kubeconfig or create new one
	var kubeConfig *KubeConfig
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		kubeConfig = &KubeConfig{
			APIVersion: "v1",
			Kind:       "Config",
			Clusters:   []KubeCluster{},
			Contexts:   []KubeContext{},
			Users:      []KubeUser{},
		}
	} else {
		kubeConfig, err = loadKubeConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load existing kubeconfig: %w", err)
		}
	}

	addedCount := 0
	updatedCount := 0

	for _, cluster := range clusters {
		contextName := fmt.Sprintf("%s-%s", cluster.Name, cluster.Region)
		clusterName := contextName
		userName := contextName

		// Check if cluster already exists
		clusterExists := false
		for i, existingCluster := range kubeConfig.Clusters {
			if existingCluster.Name == clusterName {
				// Update existing cluster
				kubeConfig.Clusters[i].Cluster.Server = cluster.Endpoint
				clusterExists = true
				updatedCount++
				break
			}
		}

		if !clusterExists {
			// Add new cluster
			kubeConfig.Clusters = append(kubeConfig.Clusters, KubeCluster{
				Name: clusterName,
				Cluster: KubeClusterConfig{
					Server: cluster.Endpoint,
				},
			})
			addedCount++
		}

		// Check if user already exists
		userExists := false
		for i, existingUser := range kubeConfig.Users {
			if existingUser.Name == userName {
				// Update existing user
				kubeConfig.Users[i].User.Exec = KubeExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    "aws",
					Args: []string{
						"eks", "get-token",
						"--cluster-name", cluster.Name,
						"--region", cluster.Region,
					},
				}
				userExists = true
				break
			}
		}

		if !userExists {
			// Add new user
			kubeConfig.Users = append(kubeConfig.Users, KubeUser{
				Name: userName,
				User: KubeUserExec{
					Exec: KubeExecConfig{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    "aws",
						Args: []string{
							"eks", "get-token",
							"--cluster-name", cluster.Name,
							"--region", cluster.Region,
						},
					},
				},
			})
		}

		// Check if context already exists
		contextExists := false
		for i, existingContext := range kubeConfig.Contexts {
			if existingContext.Name == contextName {
				// Update existing context
				kubeConfig.Contexts[i].Context.Cluster = clusterName
				kubeConfig.Contexts[i].Context.User = userName
				contextExists = true
				break
			}
		}

		if !contextExists {
			// Add new context
			kubeConfig.Contexts = append(kubeConfig.Contexts, KubeContext{
				Name: contextName,
				Context: KubeContextConfig{
					Cluster: clusterName,
					User:    userName,
				},
			})
		}
	}

	// Sort entries for consistent output
	sort.Slice(kubeConfig.Clusters, func(i, j int) bool {
		return kubeConfig.Clusters[i].Name < kubeConfig.Clusters[j].Name
	})
	sort.Slice(kubeConfig.Contexts, func(i, j int) bool {
		return kubeConfig.Contexts[i].Name < kubeConfig.Contexts[j].Name
	})
	sort.Slice(kubeConfig.Users, func(i, j int) bool {
		return kubeConfig.Users[i].Name < kubeConfig.Users[j].Name
	})

	// Save kubeconfig
	err = saveKubeConfig(kubeConfig, configPath)
	if err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	fmt.Printf("📊 Added %d new clusters, updated %d existing clusters\n", addedCount, updatedCount)
	return nil
}

func loadKubeConfig(configPath string) (*KubeConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig file: %w", err)
	}

	var kubeConfig KubeConfig
	err = yaml.Unmarshal(data, &kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig YAML: %w", err)
	}

	return &kubeConfig, nil
}

func saveKubeConfig(kubeConfig *KubeConfig, configPath string) error {
	data, err := yaml.Marshal(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig to YAML: %w", err)
	}

	err = os.WriteFile(configPath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %w", err)
	}

	return nil
}
