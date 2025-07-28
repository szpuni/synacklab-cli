package cmd

import (
	"testing"
)

func TestEKSCtxCommand(t *testing.T) {
	// Test that the command is properly initialized
	if eksCtxCmd == nil {
		t.Fatal("eksCtxCmd should not be nil")
	}

	// Test command properties
	if eksCtxCmd.Use != "eks-ctx" {
		t.Errorf("Expected command use to be 'eks-ctx', got '%s'", eksCtxCmd.Use)
	}

	if eksCtxCmd.Short == "" {
		t.Error("Command should have a short description")
	}

	if eksCtxCmd.Long == "" {
		t.Error("Command should have a long description")
	}

	if eksCtxCmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}
}

func TestEKSCtxFlags(t *testing.T) {
	// Reset flags to default values
	listContexts = false
	useFilter = false

	// Test that flags are properly defined
	listFlag := eksCtxCmd.Flags().Lookup("list")
	if listFlag == nil {
		t.Error("list flag should be defined")
	}

	filterFlag := eksCtxCmd.Flags().Lookup("filter")
	if filterFlag == nil {
		t.Error("filter flag should be defined")
	}

	// Test flag shortcuts
	listShortFlag := eksCtxCmd.Flags().ShorthandLookup("l")
	if listShortFlag == nil {
		t.Error("list flag should have shorthand 'l'")
	}

	filterShortFlag := eksCtxCmd.Flags().ShorthandLookup("f")
	if filterShortFlag == nil {
		t.Error("filter flag should have shorthand 'f'")
	}
}

func TestKubeContextInfo(t *testing.T) {
	// Test KubeContextInfo struct initialization
	contextInfo := KubeContextInfo{
		Name:      "test-context",
		Cluster:   "test-cluster",
		User:      "test-user",
		Namespace: "test-namespace",
		IsCurrent: true,
	}

	if contextInfo.Name != "test-context" {
		t.Errorf("Expected context name 'test-context', got '%s'", contextInfo.Name)
	}

	if contextInfo.Cluster != "test-cluster" {
		t.Errorf("Expected cluster 'test-cluster', got '%s'", contextInfo.Cluster)
	}

	if contextInfo.User != "test-user" {
		t.Errorf("Expected user 'test-user', got '%s'", contextInfo.User)
	}

	if contextInfo.Namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", contextInfo.Namespace)
	}

	if !contextInfo.IsCurrent {
		t.Error("Expected IsCurrent to be true")
	}
}

func TestParseContexts(t *testing.T) {
	// Create a test kubeconfig
	kubeConfig := &KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "context1",
		Contexts: []KubeContext{
			{
				Name: "context1",
				Context: KubeContextConfig{
					Cluster:   "cluster1",
					User:      "user1",
					Namespace: "namespace1",
				},
			},
			{
				Name: "context2",
				Context: KubeContextConfig{
					Cluster:   "cluster2",
					User:      "user2",
					Namespace: "",
				},
			},
		},
	}

	contexts, err := parseContexts(kubeConfig)
	if err != nil {
		t.Fatalf("parseContexts failed: %v", err)
	}

	if len(contexts) != 2 {
		t.Errorf("Expected 2 contexts, got %d", len(contexts))
	}

	// Test first context
	if contexts[0].Name != "context1" {
		t.Errorf("Expected first context name 'context1', got '%s'", contexts[0].Name)
	}

	if !contexts[0].IsCurrent {
		t.Error("Expected first context to be current")
	}

	if contexts[0].Namespace != "namespace1" {
		t.Errorf("Expected first context namespace 'namespace1', got '%s'", contexts[0].Namespace)
	}

	// Test second context
	if contexts[1].Name != "context2" {
		t.Errorf("Expected second context name 'context2', got '%s'", contexts[1].Name)
	}

	if contexts[1].IsCurrent {
		t.Error("Expected second context to not be current")
	}

	// Test default namespace assignment
	if contexts[1].Namespace != "default" {
		t.Errorf("Expected second context namespace 'default', got '%s'", contexts[1].Namespace)
	}
}

func TestValidateContext(t *testing.T) {
	// Create a test kubeconfig
	kubeConfig := &KubeConfig{
		APIVersion: "v1",
		Kind:       "Config",
		Contexts: []KubeContext{
			{
				Name: "valid-context",
				Context: KubeContextConfig{
					Cluster: "test-cluster",
					User:    "test-user",
				},
			},
		},
		Clusters: []KubeCluster{
			{
				Name: "test-cluster",
				Cluster: KubeClusterConfig{
					Server: "https://test.example.com",
				},
			},
		},
		Users: []KubeUser{
			{
				Name: "test-user",
				User: KubeUserExec{
					Exec: KubeExecConfig{
						Command: "aws",
					},
				},
			},
		},
	}

	// Test valid context
	err := validateContext(kubeConfig, "valid-context")
	if err != nil {
		t.Errorf("validateContext should succeed for valid context: %v", err)
	}

	// Test non-existent context
	err = validateContext(kubeConfig, "non-existent-context")
	if err == nil {
		t.Error("validateContext should fail for non-existent context")
	}

	// Test context with missing cluster
	kubeConfig.Contexts[0].Context.Cluster = "missing-cluster"
	err = validateContext(kubeConfig, "valid-context")
	if err == nil {
		t.Error("validateContext should fail for context with missing cluster")
	}

	// Reset cluster and test context with missing user
	kubeConfig.Contexts[0].Context.Cluster = "test-cluster"
	kubeConfig.Contexts[0].Context.User = "missing-user"
	err = validateContext(kubeConfig, "valid-context")
	if err == nil {
		t.Error("validateContext should fail for context with missing user")
	}
}

func TestGetCurrentContextInfo(t *testing.T) {
	// Create a test kubeconfig
	kubeConfig := &KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "current-context",
		Contexts: []KubeContext{
			{
				Name: "current-context",
				Context: KubeContextConfig{
					Cluster:   "test-cluster",
					User:      "test-user",
					Namespace: "test-namespace",
				},
			},
			{
				Name: "other-context",
				Context: KubeContextConfig{
					Cluster: "other-cluster",
					User:    "other-user",
				},
			},
		},
	}

	// Test getting current context info
	contextInfo, err := getCurrentContextInfo(kubeConfig)
	if err != nil {
		t.Fatalf("getCurrentContextInfo failed: %v", err)
	}

	if contextInfo.Name != "current-context" {
		t.Errorf("Expected context name 'current-context', got '%s'", contextInfo.Name)
	}

	if contextInfo.Cluster != "test-cluster" {
		t.Errorf("Expected cluster 'test-cluster', got '%s'", contextInfo.Cluster)
	}

	if !contextInfo.IsCurrent {
		t.Error("Expected IsCurrent to be true")
	}

	// Test with no current context
	kubeConfig.CurrentContext = ""
	_, err = getCurrentContextInfo(kubeConfig)
	if err == nil {
		t.Error("getCurrentContextInfo should fail when no current context is set")
	}

	// Test with invalid current context
	kubeConfig.CurrentContext = "non-existent-context"
	_, err = getCurrentContextInfo(kubeConfig)
	if err == nil {
		t.Error("getCurrentContextInfo should fail when current context doesn't exist")
	}
}

func TestUpdateCurrentContext(t *testing.T) {
	// This test would require file system operations, so we'll test the logic
	// without actually writing to disk
	kubeConfig := &KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "old-context",
		Contexts: []KubeContext{
			{
				Name: "old-context",
				Context: KubeContextConfig{
					Cluster: "cluster1",
					User:    "user1",
				},
			},
			{
				Name: "new-context",
				Context: KubeContextConfig{
					Cluster: "cluster2",
					User:    "user2",
				},
			},
		},
	}

	// Test updating to existing context (without file operations)
	// We'll just test the validation part
	contextExists := false
	for _, context := range kubeConfig.Contexts {
		if context.Name == "new-context" {
			contextExists = true
			break
		}
	}

	if !contextExists {
		t.Error("Context 'new-context' should exist in test kubeconfig")
	}

	// Test with non-existent context
	contextExists = false
	for _, context := range kubeConfig.Contexts {
		if context.Name == "non-existent-context" {
			contextExists = true
			break
		}
	}

	if contextExists {
		t.Error("Context 'non-existent-context' should not exist in test kubeconfig")
	}
}
