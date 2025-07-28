package fuzzy

import (
	"testing"
)

func TestNew(t *testing.T) {
	prompt := "Test prompt"
	finder := New(prompt)

	if finder == nil {
		t.Fatal("New should return a non-nil finder")
	}

	if finder.prompt != prompt {
		t.Errorf("Expected prompt '%s', got '%s'", prompt, finder.prompt)
	}

	if len(finder.options) != 0 {
		t.Errorf("Expected 0 options, got %d", len(finder.options))
	}
}

func TestAddOption(t *testing.T) {
	finder := New("Test")

	finder.AddOption("value1", "description1")
	finder.AddOption("value2", "description2")

	if len(finder.options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(finder.options))
	}

	if finder.options[0].Value != "value1" {
		t.Errorf("Expected first option value 'value1', got '%s'", finder.options[0].Value)
	}

	if finder.options[0].Description != "description1" {
		t.Errorf("Expected first option description 'description1', got '%s'", finder.options[0].Description)
	}

	if finder.options[1].Value != "value2" {
		t.Errorf("Expected second option value 'value2', got '%s'", finder.options[1].Value)
	}

	if finder.options[1].Description != "description2" {
		t.Errorf("Expected second option description 'description2', got '%s'", finder.options[1].Description)
	}
}

func TestFilterOptions(t *testing.T) {
	finder := New("Test")

	finder.AddOption("production-cluster", "Production environment")
	finder.AddOption("staging-cluster", "Staging environment")
	finder.AddOption("development-cluster", "Development environment")
	finder.AddOption("test-cluster", "Test environment")

	// Test filtering by value
	filtered := finder.filterOptions("prod")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered option for 'prod', got %d", len(filtered))
	}
	if len(filtered) > 0 && filtered[0].Value != "production-cluster" {
		t.Errorf("Expected filtered option 'production-cluster', got '%s'", filtered[0].Value)
	}

	// Test filtering by description
	filtered = finder.filterOptions("staging")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered option for 'staging', got %d", len(filtered))
	}
	if len(filtered) > 0 && filtered[0].Value != "staging-cluster" {
		t.Errorf("Expected filtered option 'staging-cluster', got '%s'", filtered[0].Value)
	}

	// Test filtering with multiple matches
	filtered = finder.filterOptions("cluster")
	if len(filtered) != 4 {
		t.Errorf("Expected 4 filtered options for 'cluster', got %d", len(filtered))
	}

	// Test filtering with no matches
	filtered = finder.filterOptions("nonexistent")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 filtered options for 'nonexistent', got %d", len(filtered))
	}

	// Test case insensitive filtering
	filtered = finder.filterOptions("PROD")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered option for 'PROD' (case insensitive), got %d", len(filtered))
	}
}

func TestGetOptions(t *testing.T) {
	finder := New("Test")

	finder.AddOption("option1", "desc1")
	finder.AddOption("option2", "desc2")

	options := finder.GetOptions()
	if len(options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(options))
	}

	if options[0].Value != "option1" {
		t.Errorf("Expected first option 'option1', got '%s'", options[0].Value)
	}

	if options[1].Value != "option2" {
		t.Errorf("Expected second option 'option2', got '%s'", options[1].Value)
	}
}

func TestClear(t *testing.T) {
	finder := New("Test")

	finder.AddOption("option1", "desc1")
	finder.AddOption("option2", "desc2")

	if len(finder.options) != 2 {
		t.Errorf("Expected 2 options before clear, got %d", len(finder.options))
	}

	finder.Clear()

	if len(finder.options) != 0 {
		t.Errorf("Expected 0 options after clear, got %d", len(finder.options))
	}
}

func TestSetPrompt(t *testing.T) {
	finder := New("Original prompt")

	if finder.prompt != "Original prompt" {
		t.Errorf("Expected original prompt 'Original prompt', got '%s'", finder.prompt)
	}

	finder.SetPrompt("New prompt")

	if finder.prompt != "New prompt" {
		t.Errorf("Expected new prompt 'New prompt', got '%s'", finder.prompt)
	}
}

func TestOption(t *testing.T) {
	option := Option{
		Value:       "test-value",
		Description: "test-description",
	}

	if option.Value != "test-value" {
		t.Errorf("Expected option value 'test-value', got '%s'", option.Value)
	}

	if option.Description != "test-description" {
		t.Errorf("Expected option description 'test-description', got '%s'", option.Description)
	}
}

// Test error cases
func TestSelectWithNoOptions(t *testing.T) {
	finder := New("Test")

	// Test Select with no options
	_, err := finder.Select()
	if err == nil {
		t.Error("Select should return error when no options are available")
	}

	// Test SelectWithFilter with no options
	_, err = finder.SelectWithFilter()
	if err == nil {
		t.Error("SelectWithFilter should return error when no options are available")
	}
}
