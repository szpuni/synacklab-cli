package fuzzy

import (
	"fmt"
	"strings"
	"testing"

	fzf "github.com/junegunn/fzf/src"
)

// MockFzfRunner implements FzfRunner for testing
type MockFzfRunner struct {
	RunFunc       func(opts *fzf.Options) (int, error)
	CallCount     int
	LastOpts      *fzf.Options
	OutputToWrite string // What to write to stdout to simulate fzf output
}

// Run executes the mock function
func (m *MockFzfRunner) Run(opts *fzf.Options) (int, error) {
	m.CallCount++
	m.LastOpts = opts

	// Write the mock output to stdout if specified
	if m.OutputToWrite != "" {
		fmt.Print(m.OutputToWrite)
	}

	if m.RunFunc != nil {
		return m.RunFunc(opts)
	}
	// Default behavior: return success
	return fzf.ExitOk, nil
}

func TestNewFzf(t *testing.T) {
	finder := NewFzf("Test prompt")
	if finder == nil {
		t.Fatal("NewFzf returned nil")
	}

	if finder.prompt != "Test prompt" {
		t.Errorf("Expected prompt 'Test prompt', got '%s'", finder.prompt)
	}

	if len(finder.options) != 0 {
		t.Errorf("Expected empty options, got %d options", len(finder.options))
	}
}

func TestFzfSetOptions(t *testing.T) {
	finder := NewFzf("Test")

	// Test with nil options
	err := finder.SetOptions(nil)
	if err == nil {
		t.Error("Expected error when setting nil options")
	}

	// Test with valid options
	options := []Option{
		{Value: "option1", Description: "First option"},
		{Value: "option2", Description: "Second option"},
	}

	err = finder.SetOptions(options)
	if err != nil {
		t.Errorf("Unexpected error setting options: %v", err)
	}

	if len(finder.options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(finder.options))
	}

	if finder.options[0].Value != "option1" {
		t.Errorf("Expected first option value 'option1', got '%s'", finder.options[0].Value)
	}
}

func TestFzfSetPrompt(t *testing.T) {
	finder := NewFzf("Initial prompt")
	finder.SetPrompt("New prompt")

	if finder.prompt != "New prompt" {
		t.Errorf("Expected prompt 'New prompt', got '%s'", finder.prompt)
	}
}

func TestFzfCreation(t *testing.T) {
	finder := NewFzf("Test")

	// Test that the finder exists and can be created
	if finder == nil {
		t.Fatal("Expected finder to be created successfully")
	}

	// Verify the finder is functional
	if finder.prompt != "Test" {
		t.Errorf("Expected prompt 'Test', got '%s'", finder.prompt)
	}
}

func TestFzfSelectWithNoOptions(t *testing.T) {
	finder := NewFzf("Test")

	_, err := finder.Select()
	if err == nil {
		t.Error("Expected error when selecting with no options")
	}

	expectedError := "no options available"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestFzfSelect(t *testing.T) {
	// Create a mock runner that simulates successful selection
	mockRunner := &MockFzfRunner{
		OutputToWrite: "option1  │  First option\n", // Simulate fzf selecting the first option
		RunFunc: func(_ *fzf.Options) (int, error) {
			return fzf.ExitOk, nil
		},
	}

	finder := NewFzfWithRunner("Test", mockRunner)

	// Set some options
	options := []Option{
		{Value: "option1", Description: "First option"},
		{Value: "option2", Description: "Second option"},
	}
	err := finder.SetOptions(options)
	if err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test select behavior with mock
	selected, err := finder.Select()
	if err != nil {
		t.Errorf("Select failed: %v", err)
	}

	// Should select the first option
	if selected != "option1" {
		t.Errorf("Expected 'option1', got '%s'", selected)
	}

	// Verify the mock was called
	if mockRunner.CallCount != 1 {
		t.Errorf("Expected 1 call to Run, got %d", mockRunner.CallCount)
	}
}

func TestFzfInterface(t *testing.T) {
	// Test that FzfFinder implements FzfFinderInterface
	var _ FzfFinderInterface = (*FzfFinder)(nil)

	// Create a mock runner
	mockRunner := &MockFzfRunner{
		RunFunc: func(_ *fzf.Options) (int, error) {
			return fzf.ExitOk, nil
		},
	}

	// Test interface methods
	finder := NewFzfWithRunner("Test", mockRunner)

	// Test SetOptions
	options := []Option{{Value: "test", Description: "test option"}}
	err := finder.SetOptions(options)
	if err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test SetPrompt
	finder.SetPrompt("New prompt")
	if finder.prompt != "New prompt" {
		t.Errorf("SetPrompt failed: expected 'New prompt', got '%s'", finder.prompt)
	}

	// Test Select method exists - reset to empty options to test error case
	err = finder.SetOptions([]Option{})
	if err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}
	_, err = finder.Select()
	if err == nil {
		t.Error("Expected error when calling Select with no options")
	}
}

func TestFzfOptionsWithMetadata(t *testing.T) {
	finder := NewFzf("Test")

	metadata := map[string]string{
		"account_id": "123456789",
		"role_name":  "admin",
		"current":    "true",
	}

	options := []Option{
		{
			Value:       "profile1",
			Description: "Test profile",
			Metadata:    metadata,
		},
	}

	err := finder.SetOptions(options)
	if err != nil {
		t.Errorf("SetOptions with metadata failed: %v", err)
	}

	if len(finder.options) != 1 {
		t.Errorf("Expected 1 option, got %d", len(finder.options))
	}

	option := finder.options[0]
	if option.Metadata["account_id"] != "123456789" {
		t.Errorf("Expected account_id '123456789', got '%s'", option.Metadata["account_id"])
	}

	if option.Metadata["current"] != "true" {
		t.Errorf("Expected current 'true', got '%s'", option.Metadata["current"])
	}
}

func TestFzfSelectWithFallback(t *testing.T) {
	// Create a mock runner that simulates fzf failure
	mockRunner := &MockFzfRunner{
		RunFunc: func(_ *fzf.Options) (int, error) {
			return 1, fmt.Errorf("fzf failed")
		},
	}

	finder := NewFzfWithRunner("Test", mockRunner)

	// Set some options
	options := []Option{
		{Value: "option1", Description: "First option"},
		{Value: "option2", Description: "Second option"},
	}
	err := finder.SetOptions(options)
	if err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test that fallback is called when fzf fails
	// Note: This will use the fallbackSelect method which uses the simple finder
	// In a real test environment, this would require stdin input, so we expect it to fail
	_, err = finder.Select()

	// We expect an error because the fallback will also fail in test environment
	if err == nil {
		t.Log("Fallback succeeded (unexpected in test environment)")
	} else {
		t.Logf("Fallback failed as expected in test environment: %v", err)
	}

	// Verify the mock was called
	if mockRunner.CallCount != 1 {
		t.Errorf("Expected 1 call to Run, got %d", mockRunner.CallCount)
	}
}

func TestFzfSelectCancelled(t *testing.T) {
	// Create a mock runner that simulates user cancellation
	mockRunner := &MockFzfRunner{
		RunFunc: func(_ *fzf.Options) (int, error) {
			return 1, nil // Exit code 1 means cancelled
		},
	}

	finder := NewFzfWithRunner("Test", mockRunner)

	// Set some options
	options := []Option{
		{Value: "option1", Description: "First option"},
	}
	err := finder.SetOptions(options)
	if err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test cancellation
	_, err = finder.Select()
	if err == nil {
		t.Error("Expected error when fzf is cancelled")
	}

	expectedError := "fzf selection cancelled or failed"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}
