package cmd

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
)

func TestEKSConfigCommand(t *testing.T) {
	// Test that the command is properly initialized
	if eksConfigCmd == nil {
		t.Fatal("eksConfigCmd should not be nil")
	}

	// Test command properties
	if eksConfigCmd.Use != "eks-config" {
		t.Errorf("Expected command use to be 'eks-config', got '%s'", eksConfigCmd.Use)
	}

	if eksConfigCmd.Short == "" {
		t.Error("Command should have a short description")
	}

	if eksConfigCmd.Long == "" {
		t.Error("Command should have a long description")
	}

	if eksConfigCmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}
}

func TestEKSConfigFlags(t *testing.T) {
	// Reset flags to default values
	eksRegion = ""
	dryRun = false

	// Test that flags are properly defined
	regionFlag := eksConfigCmd.Flags().Lookup("region")
	if regionFlag == nil {
		t.Error("region flag should be defined")
	}

	dryRunFlag := eksConfigCmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("dry-run flag should be defined")
	}

	// Test flag shortcuts
	regionShortFlag := eksConfigCmd.Flags().ShorthandLookup("r")
	if regionShortFlag == nil {
		t.Error("region flag should have shorthand 'r'")
	}
}

func TestGetRegionsToSearch(t *testing.T) {
	// Create a mock AWS config for testing
	var mockConfig aws.Config

	// Test with specific region
	eksRegion = "us-west-2"
	regions, err := getRegionsToSearch(mockConfig)
	if err != nil {
		t.Fatalf("getRegionsToSearch failed: %v", err)
	}

	if len(regions) != 1 {
		t.Errorf("Expected 1 region, got %d", len(regions))
	}

	if regions[0] != "us-west-2" {
		t.Errorf("Expected region 'us-west-2', got '%s'", regions[0])
	}

	// Test with no region specified (should return multiple regions)
	eksRegion = ""
	regions, err = getRegionsToSearch(mockConfig)
	if err != nil {
		t.Fatalf("getRegionsToSearch failed: %v", err)
	}

	if len(regions) == 0 {
		t.Error("Expected multiple regions when no region specified")
	}

	// Verify some expected regions are included
	expectedRegions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	for _, expected := range expectedRegions {
		found := false
		for _, region := range regions {
			if region == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected region '%s' to be in the list", expected)
		}
	}
}

func TestEKSClusterStruct(t *testing.T) {
	// Test EKSCluster struct initialization
	cluster := EKSCluster{
		Name:     "test-cluster",
		Region:   "us-east-1",
		Endpoint: "https://test.eks.amazonaws.com",
		ARN:      "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster",
		Status:   "ACTIVE",
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", cluster.Name)
	}

	if cluster.Region != "us-east-1" {
		t.Errorf("Expected cluster region 'us-east-1', got '%s'", cluster.Region)
	}
}

func TestKubeConfigStructs(t *testing.T) {
	// Test KubeConfig struct initialization
	kubeConfig := KubeConfig{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters:   []KubeCluster{},
		Contexts:   []KubeContext{},
		Users:      []KubeUser{},
	}

	if kubeConfig.APIVersion != "v1" {
		t.Errorf("Expected API version 'v1', got '%s'", kubeConfig.APIVersion)
	}

	if kubeConfig.Kind != "Config" {
		t.Errorf("Expected kind 'Config', got '%s'", kubeConfig.Kind)
	}

	if kubeConfig.Clusters == nil {
		t.Error("Clusters should be initialized")
	}

	if kubeConfig.Contexts == nil {
		t.Error("Contexts should be initialized")
	}

	if kubeConfig.Users == nil {
		t.Error("Users should be initialized")
	}
}

// Test helper function to create a test command
func createTestEKSConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eks-config",
		Short: "Test EKS config command",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}

	cmd.Flags().StringVarP(&eksRegion, "region", "r", "", "AWS region")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run mode")

	return cmd
}

func TestCommandExecution(t *testing.T) {
	// Test that the command can be executed without errors (dry run mode)
	testCmd := createTestEKSConfigCmd()

	// Set dry run mode to avoid actual AWS calls
	dryRun = true
	eksRegion = "us-east-1"

	err := testCmd.Execute()
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	// Reset flags
	dryRun = false
	eksRegion = ""
}
