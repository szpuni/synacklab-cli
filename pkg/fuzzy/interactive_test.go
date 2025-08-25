package fuzzy

import (
	"fmt"
	"testing"
)

func TestNewInteractive(t *testing.T) {
	prompt := "Test interactive prompt"
	finder := NewInteractive(prompt)

	if finder == nil {
		t.Fatal("NewInteractive should return a non-nil finder")
	}

	impl, ok := finder.(*InteractiveFinderImpl)
	if !ok {
		t.Fatal("NewInteractive should return an InteractiveFinderImpl")
	}

	if impl.prompt != prompt {
		t.Errorf("Expected prompt '%s', got '%s'", prompt, impl.prompt)
	}

	if impl.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex 0, got %d", impl.selectedIndex)
	}

	if impl.displayOffset != 0 {
		t.Errorf("Expected displayOffset 0, got %d", impl.displayOffset)
	}

	if impl.filterText != "" {
		t.Errorf("Expected empty filterText, got '%s'", impl.filterText)
	}
}

func TestInteractiveSetOptions(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "option1", Description: "First option"},
		{Value: "option2", Description: "Second option"},
	}

	err := finder.SetOptions(options)
	if err != nil {
		t.Errorf("SetOptions should not return error, got: %v", err)
	}

	if len(impl.options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(impl.options))
	}

	if len(impl.filteredOptions) != 2 {
		t.Errorf("Expected 2 filtered options, got %d", len(impl.filteredOptions))
	}

	if impl.options[0].Value != "option1" {
		t.Errorf("Expected first option 'option1', got '%s'", impl.options[0].Value)
	}

	if impl.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex reset to 0, got %d", impl.selectedIndex)
	}
}

func TestInteractiveSetOptionsNil(t *testing.T) {
	finder := NewInteractive("Test")

	err := finder.SetOptions(nil)
	if err == nil {
		t.Error("SetOptions should return error for nil options")
	}
}

func TestInteractiveSetPrompt(t *testing.T) {
	finder := NewInteractive("Original")
	impl := finder.(*InteractiveFinderImpl)

	newPrompt := "New prompt"
	finder.SetPrompt(newPrompt)

	if impl.prompt != newPrompt {
		t.Errorf("Expected prompt '%s', got '%s'", newPrompt, impl.prompt)
	}
}

func TestInteractiveSetKeyBindings(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	customBindings := KeyBindings{
		Up:     []string{"w"},
		Down:   []string{"s"},
		Select: []string{" "},
		Cancel: []string{"q"},
	}

	finder.SetKeyBindings(customBindings)

	if len(impl.keyBindings.Up) != 1 || impl.keyBindings.Up[0] != "w" {
		t.Errorf("Expected Up binding 'w', got %v", impl.keyBindings.Up)
	}

	if len(impl.keyBindings.Down) != 1 || impl.keyBindings.Down[0] != "s" {
		t.Errorf("Expected Down binding 's', got %v", impl.keyBindings.Down)
	}
}

func TestDefaultKeyBindings(t *testing.T) {
	bindings := DefaultKeyBindings()

	expectedUp := []string{"\x1b[A", "k"}
	expectedDown := []string{"\x1b[B", "j"}

	if len(bindings.Up) != len(expectedUp) {
		t.Errorf("Expected %d Up bindings, got %d", len(expectedUp), len(bindings.Up))
	}

	for i, expected := range expectedUp {
		if i >= len(bindings.Up) || bindings.Up[i] != expected {
			t.Errorf("Expected Up binding[%d] '%s', got '%s'", i, expected, bindings.Up[i])
		}
	}

	if len(bindings.Down) != len(expectedDown) {
		t.Errorf("Expected %d Down bindings, got %d", len(expectedDown), len(bindings.Down))
	}

	for i, expected := range expectedDown {
		if i >= len(bindings.Down) || bindings.Down[i] != expected {
			t.Errorf("Expected Down binding[%d] '%s', got '%s'", i, expected, bindings.Down[i])
		}
	}

	// Test that Select and Cancel bindings exist
	if len(bindings.Select) == 0 {
		t.Error("Expected Select bindings to be non-empty")
	}

	if len(bindings.Cancel) == 0 {
		t.Error("Expected Cancel bindings to be non-empty")
	}
}

func TestInteractiveFilterOptions(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "production-cluster", Description: "Production environment"},
		{Value: "staging-cluster", Description: "Staging environment"},
		{Value: "development-cluster", Description: "Development environment"},
		{Value: "test-cluster", Description: "Test environment"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test filtering by value
	filtered := impl.filterOptions("prod")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered option for 'prod', got %d", len(filtered))
	}
	if len(filtered) > 0 && filtered[0].Value != "production-cluster" {
		t.Errorf("Expected filtered option 'production-cluster', got '%s'", filtered[0].Value)
	}

	// Test filtering by description
	filtered = impl.filterOptions("staging")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered option for 'staging', got %d", len(filtered))
	}
	if len(filtered) > 0 && filtered[0].Value != "staging-cluster" {
		t.Errorf("Expected filtered option 'staging-cluster', got '%s'", filtered[0].Value)
	}

	// Test filtering with multiple matches
	filtered = impl.filterOptions("cluster")
	if len(filtered) != 4 {
		t.Errorf("Expected 4 filtered options for 'cluster', got %d", len(filtered))
	}

	// Test filtering with no matches
	filtered = impl.filterOptions("nonexistent")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 filtered options for 'nonexistent', got %d", len(filtered))
	}

	// Test case insensitive filtering
	filtered = impl.filterOptions("PROD")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered option for 'PROD' (case insensitive), got %d", len(filtered))
	}
}

func TestUpdateFilter(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "production-cluster", Description: "Production environment"},
		{Value: "staging-cluster", Description: "Staging environment"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Set initial selection
	impl.selectedIndex = 1
	impl.displayOffset = 1

	// Update filter
	impl.filterText = "prod"
	impl.updateFilter()

	// Check that selection and offset are reset
	if impl.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex reset to 0, got %d", impl.selectedIndex)
	}

	if impl.displayOffset != 0 {
		t.Errorf("Expected displayOffset reset to 0, got %d", impl.displayOffset)
	}

	// Check filtered options
	if len(impl.filteredOptions) != 1 {
		t.Errorf("Expected 1 filtered option, got %d", len(impl.filteredOptions))
	}

	// Test empty filter
	impl.filterText = ""
	impl.updateFilter()

	if len(impl.filteredOptions) != 2 {
		t.Errorf("Expected 2 filtered options with empty filter, got %d", len(impl.filteredOptions))
	}
}

func TestMoveUp(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "option1", Description: "First"},
		{Value: "option2", Description: "Second"},
		{Value: "option3", Description: "Third"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Start at index 2
	impl.selectedIndex = 2

	// Move up
	impl.moveUp()
	if impl.selectedIndex != 1 {
		t.Errorf("Expected selectedIndex 1 after moveUp, got %d", impl.selectedIndex)
	}

	// Move up again
	impl.moveUp()
	if impl.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex 0 after second moveUp, got %d", impl.selectedIndex)
	}

	// Try to move up from 0 (should stay at 0)
	impl.moveUp()
	if impl.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to stay at 0, got %d", impl.selectedIndex)
	}
}

func TestMoveDown(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "option1", Description: "First"},
		{Value: "option2", Description: "Second"},
		{Value: "option3", Description: "Third"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Start at index 0
	if impl.selectedIndex != 0 {
		t.Errorf("Expected initial selectedIndex 0, got %d", impl.selectedIndex)
	}

	// Move down
	impl.moveDown()
	if impl.selectedIndex != 1 {
		t.Errorf("Expected selectedIndex 1 after moveDown, got %d", impl.selectedIndex)
	}

	// Move down again
	impl.moveDown()
	if impl.selectedIndex != 2 {
		t.Errorf("Expected selectedIndex 2 after second moveDown, got %d", impl.selectedIndex)
	}

	// Try to move down from last index (should stay at 2)
	impl.moveDown()
	if impl.selectedIndex != 2 {
		t.Errorf("Expected selectedIndex to stay at 2, got %d", impl.selectedIndex)
	}
}

func TestUpdateDisplayOffset(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	// Set small display rows for testing
	impl.maxDisplayRows = 3

	options := make([]Option, 10)
	for i := 0; i < 10; i++ {
		options[i] = Option{Value: fmt.Sprintf("option%d", i+1)}
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test scrolling down
	impl.selectedIndex = 4
	impl.updateDisplayOffset()

	expectedOffset := 4 - 3 + 1 // selectedIndex - maxDisplayRows + 1
	if impl.displayOffset != expectedOffset {
		t.Errorf("Expected displayOffset %d, got %d", expectedOffset, impl.displayOffset)
	}

	// Test scrolling up
	impl.selectedIndex = 1
	impl.updateDisplayOffset()

	if impl.displayOffset != 1 {
		t.Errorf("Expected displayOffset 1, got %d", impl.displayOffset)
	}

	// Test within visible range
	impl.displayOffset = 2
	impl.selectedIndex = 3
	impl.updateDisplayOffset()

	if impl.displayOffset != 2 {
		t.Errorf("Expected displayOffset to remain 2, got %d", impl.displayOffset)
	}
}

func TestHandleInput(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "option1", Description: "First"},
		{Value: "option2", Description: "Second"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test up key
	action := impl.handleInput("\x1b[A") // Up arrow
	if action != "update" {
		t.Errorf("Expected 'update' action for up key, got '%s'", action)
	}

	// Test vim up key
	action = impl.handleInput("k")
	if action != "update" {
		t.Errorf("Expected 'update' action for vim up key, got '%s'", action)
	}

	// Test down key
	action = impl.handleInput("\x1b[B") // Down arrow
	if action != "update" {
		t.Errorf("Expected 'update' action for down key, got '%s'", action)
	}

	// Test vim down key
	action = impl.handleInput("j")
	if action != "update" {
		t.Errorf("Expected 'update' action for vim down key, got '%s'", action)
	}

	// Test select key
	action = impl.handleInput("\r") // Enter
	if action != "select" {
		t.Errorf("Expected 'select' action for enter key, got '%s'", action)
	}

	// Test cancel key
	action = impl.handleInput("\x1b") // Escape
	if action != "cancel" {
		t.Errorf("Expected 'cancel' action for escape key, got '%s'", action)
	}

	// Test printable character
	action = impl.handleInput("a")
	if action != "update" {
		t.Errorf("Expected 'update' action for printable character, got '%s'", action)
	}

	if impl.filterText != "a" {
		t.Errorf("Expected filterText 'a', got '%s'", impl.filterText)
	}

	// Test backspace
	action = impl.handleInput("\x7f") // Backspace
	if action != "update" {
		t.Errorf("Expected 'update' action for backspace, got '%s'", action)
	}

	if impl.filterText != "" {
		t.Errorf("Expected empty filterText after backspace, got '%s'", impl.filterText)
	}

	// Test backspace with empty filter
	action = impl.handleInput("\x7f")
	if action != "" {
		t.Errorf("Expected empty action for backspace with empty filter, got '%s'", action)
	}
}

func TestInteractiveSelectWithNoOptions(t *testing.T) {
	finder := NewInteractive("Test")

	_, err := finder.Select()
	if err == nil {
		t.Error("Select should return error when no options are available")
	}

	expectedMsg := "no options available"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestIsTerminalSupported(_ *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	// This test will vary based on the environment
	// We just ensure the method doesn't panic and returns a boolean
	supported := impl.isTerminalSupported()
	_ = supported // We can't assert the value as it depends on the test environment
}

func TestGetTerminalWidth(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	width := impl.getTerminalWidth()
	if width <= 0 {
		t.Errorf("Expected positive terminal width, got %d", width)
	}

	// Test fallback
	impl.terminalWidth = 0
	width = impl.getTerminalWidth()
	if width != 80 {
		t.Errorf("Expected fallback width 80, got %d", width)
	}
}

// TestRealTimeFilteringUpdates tests the real-time filtering behavior
func TestRealTimeFilteringUpdates(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "production-web-server", Description: "Production web server"},
		{Value: "production-database", Description: "Production database"},
		{Value: "staging-web-server", Description: "Staging web server"},
		{Value: "development-api", Description: "Development API server"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Test progressive filtering
	testCases := []struct {
		filterText    string
		expectedCount int
		expectedFirst string
		description   string
	}{
		{"", 4, "production-web-server", "no filter should show all options"},
		{"p", 3, "production-web-server", "single char filter should match production items and development-api"},
		{"pr", 2, "production-web-server", "two char filter should still match production items"},
		{"prod", 2, "production-web-server", "word filter should match production items"},
		{"production", 2, "production-web-server", "full word filter should match production items"},
		{"production-w", 1, "production-web-server", "specific filter should match one item"},
		{"staging", 1, "staging-web-server", "different filter should match staging item"},
		{"server", 3, "production-web-server", "description filter should match servers"},
		{"api", 1, "development-api", "filter should match API server"},
		{"nonexistent", 0, "", "non-matching filter should return no results"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			impl.filterText = tc.filterText
			impl.updateFilter()

			if len(impl.filteredOptions) != tc.expectedCount {
				t.Errorf("Filter '%s': expected %d options, got %d", tc.filterText, tc.expectedCount, len(impl.filteredOptions))
			}

			if tc.expectedCount > 0 && impl.filteredOptions[0].Value != tc.expectedFirst {
				t.Errorf("Filter '%s': expected first option '%s', got '%s'", tc.filterText, tc.expectedFirst, impl.filteredOptions[0].Value)
			}

			// Verify selection is reset to 0
			if impl.selectedIndex != 0 {
				t.Errorf("Filter '%s': expected selectedIndex reset to 0, got %d", tc.filterText, impl.selectedIndex)
			}

			// Verify display offset is reset to 0
			if impl.displayOffset != 0 {
				t.Errorf("Filter '%s': expected displayOffset reset to 0, got %d", tc.filterText, impl.displayOffset)
			}
		})
	}
}

// TestKeyboardNavigationEdgeCases tests comprehensive keyboard navigation scenarios
func TestKeyboardNavigationEdgeCases(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	// Test with single option
	t.Run("single option navigation", func(t *testing.T) {
		singleOption := []Option{{Value: "only-option", Description: "The only option"}}
		if err := finder.SetOptions(singleOption); err != nil {
			t.Errorf("SetOptions failed: %v", err)
		}

		// Should start at index 0
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex 0 with single option, got %d", impl.selectedIndex)
		}

		// Moving up should stay at 0
		impl.moveUp()
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex to stay at 0 when moving up from single option, got %d", impl.selectedIndex)
		}

		// Moving down should stay at 0
		impl.moveDown()
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex to stay at 0 when moving down from single option, got %d", impl.selectedIndex)
		}
	})

	// Test with empty filtered results
	t.Run("empty filtered results navigation", func(t *testing.T) {
		options := []Option{
			{Value: "option1", Description: "First option"},
			{Value: "option2", Description: "Second option"},
		}
		if err := finder.SetOptions(options); err != nil {
			t.Errorf("SetOptions failed: %v", err)
		}

		// Filter to get no results
		impl.filterText = "nonexistent"
		impl.updateFilter()

		if len(impl.filteredOptions) != 0 {
			t.Errorf("Expected 0 filtered options, got %d", len(impl.filteredOptions))
		}

		// Navigation should not crash with empty results
		impl.moveUp()
		impl.moveDown()
		// If we get here without panic, the test passes
	})

	// Test navigation with large list and scrolling
	t.Run("large list scrolling navigation", func(t *testing.T) {
		// Create many options to test scrolling
		var manyOptions []Option
		for i := 0; i < 20; i++ {
			manyOptions = append(manyOptions, Option{
				Value:       fmt.Sprintf("option-%02d", i+1),
				Description: fmt.Sprintf("Option number %d", i+1),
			})
		}

		if err := finder.SetOptions(manyOptions); err != nil {
			t.Errorf("SetOptions failed: %v", err)
		}

		// Set small display rows to force scrolling
		impl.maxDisplayRows = 5

		// Navigate to middle of list
		for i := 0; i < 10; i++ {
			impl.moveDown()
		}

		if impl.selectedIndex != 10 {
			t.Errorf("Expected selectedIndex 10 after navigation, got %d", impl.selectedIndex)
		}

		// Check display offset is updated for scrolling
		expectedOffset := 10 - 5 + 1 // selectedIndex - maxDisplayRows + 1
		if impl.displayOffset != expectedOffset {
			t.Errorf("Expected displayOffset %d for scrolling, got %d", expectedOffset, impl.displayOffset)
		}

		// Navigate to end
		for i := 0; i < 20; i++ {
			impl.moveDown()
		}

		if impl.selectedIndex != 19 {
			t.Errorf("Expected selectedIndex 19 at end of list, got %d", impl.selectedIndex)
		}

		// Navigate back to beginning
		for i := 0; i < 30; i++ {
			impl.moveUp()
		}

		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex 0 at beginning of list, got %d", impl.selectedIndex)
		}

		if impl.displayOffset != 0 {
			t.Errorf("Expected displayOffset 0 at beginning of list, got %d", impl.displayOffset)
		}
	})
}

// TestInputHandlingEdgeCases tests comprehensive input handling scenarios
func TestInputHandlingEdgeCases(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "test-option", Description: "Test option"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	testCases := []struct {
		input          string
		expectedAction string
		description    string
	}{
		// Navigation keys
		{"\x1b[A", "update", "up arrow key"},
		{"\x1b[B", "update", "down arrow key"},
		{"k", "update", "vim up key"},
		{"j", "update", "vim down key"},

		// Selection and cancellation
		{"\r", "select", "carriage return"},
		{"\n", "select", "newline"},
		{"\x1b", "cancel", "escape key"},
		{"\x03", "cancel", "ctrl+c"},

		// Backspace variations (with empty filter, should return empty action)
		{"\x7f", "", "delete key (backspace)"},
		{"\b", "", "backspace key"},

		// Printable characters
		{"a", "update", "lowercase letter"},
		{"A", "update", "uppercase letter"},
		{"1", "update", "digit"},
		{"-", "update", "hyphen"},
		{"_", "update", "underscore"},
		{" ", "update", "space"},

		// Non-printable characters (should be ignored)
		{"\x01", "", "control character"},
		{"\x1f", "", "unit separator"},
		{"\x7f\x7f", "", "multiple delete keys when filter is empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Reset state for each test
			impl.filterText = ""
			impl.selectedIndex = 0
			impl.updateFilter()

			// For backspace tests with empty filter
			if tc.input == "\x7f\x7f" {
				// First backspace should do nothing, second should also do nothing
				action1 := impl.handleInput("\x7f")
				action2 := impl.handleInput("\x7f")
				if action1 != "" || action2 != "" {
					t.Errorf("Expected empty actions for backspace on empty filter, got '%s' and '%s'", action1, action2)
				}
				return
			}

			action := impl.handleInput(tc.input)
			if action != tc.expectedAction {
				t.Errorf("Input '%s' (%v): expected action '%s', got '%s'", tc.description, []byte(tc.input), tc.expectedAction, action)
			}

			// For printable characters (excluding vim navigation keys), verify filter text is updated
			if tc.expectedAction == "update" && len(tc.input) == 1 && tc.input[0] >= 32 && tc.input[0] <= 126 && tc.input != "k" && tc.input != "j" {
				if impl.filterText != tc.input {
					t.Errorf("Input '%s': expected filterText '%s', got '%s'", tc.description, tc.input, impl.filterText)
				}
			}
		})
	}
}

// TestCustomKeyBindings tests custom key binding functionality
func TestCustomKeyBindings(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	options := []Option{
		{Value: "option1", Description: "First"},
		{Value: "option2", Description: "Second"},
	}

	if err := finder.SetOptions(options); err != nil {
		t.Errorf("SetOptions failed: %v", err)
	}

	// Set custom key bindings
	customBindings := KeyBindings{
		Up:     []string{"w", "W"},
		Down:   []string{"s", "S"},
		Select: []string{" ", "x"},
		Cancel: []string{"q", "Q"},
	}

	finder.SetKeyBindings(customBindings)

	testCases := []struct {
		input          string
		expectedAction string
		description    string
	}{
		{"w", "update", "custom up key (w)"},
		{"W", "update", "custom up key (W)"},
		{"s", "update", "custom down key (s)"},
		{"S", "update", "custom down key (S)"},
		{" ", "select", "custom select key (space)"},
		{"x", "select", "custom select key (x)"},
		{"q", "cancel", "custom cancel key (q)"},
		{"Q", "cancel", "custom cancel key (Q)"},

		// Default keys should no longer work
		{"\x1b[A", "", "default up arrow should not work"},
		{"\x1b[B", "", "default down arrow should not work"},
		{"k", "update", "default vim up should now be treated as printable character"},
		{"j", "update", "default vim down should now be treated as printable character"},
		{"\r", "", "default enter should not work"},
		{"\x1b", "", "default escape should not work"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			action := impl.handleInput(tc.input)
			if action != tc.expectedAction {
				t.Errorf("Input '%s': expected action '%s', got '%s'", tc.description, tc.expectedAction, action)
			}
		})
	}
}

// TestTerminalInteractionMocking tests terminal interaction edge cases
func TestTerminalInteractionMocking(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	// Test terminal size handling
	t.Run("terminal size updates", func(t *testing.T) {
		// Test updateTerminalSize doesn't panic
		impl.updateTerminalSize()

		// Verify reasonable defaults are set
		if impl.terminalWidth <= 0 {
			t.Errorf("Expected positive terminal width, got %d", impl.terminalWidth)
		}

		if impl.terminalHeight <= 0 {
			t.Errorf("Expected positive terminal height, got %d", impl.terminalHeight)
		}

		if impl.maxDisplayRows <= 0 {
			t.Errorf("Expected positive maxDisplayRows, got %d", impl.maxDisplayRows)
		}
	})

	// Test getTerminalSize error handling
	t.Run("terminal size error handling", func(t *testing.T) {
		width, height, err := impl.getTerminalSize()

		// The function should either succeed or fail gracefully
		if err != nil {
			// If it fails, width and height should be 0
			if width != 0 || height != 0 {
				t.Errorf("Expected zero dimensions on error, got width=%d, height=%d", width, height)
			}
		} else {
			// If it succeeds, dimensions should be positive
			if width <= 0 || height <= 0 {
				t.Errorf("Expected positive dimensions on success, got width=%d, height=%d", width, height)
			}
		}
	})

	// Test fallback behavior
	t.Run("fallback select behavior", func(t *testing.T) {
		options := []Option{
			{Value: "option1", Description: "First option"},
			{Value: "option2", Description: "Second option"},
		}

		if err := finder.SetOptions(options); err != nil {
			t.Errorf("SetOptions failed: %v", err)
		}

		// Test that fallbackSelect doesn't panic
		// Note: We can't easily test the actual selection without user input
		// but we can verify the method exists and handles basic setup
		result, err := impl.fallbackSelect()

		// The fallback should either succeed with a selection or fail gracefully
		if err != nil {
			// Error is expected in test environment without user input
			if result != "" {
				t.Errorf("Expected empty result on error, got '%s'", result)
			}
		}
	})
}

// TestGracefulEmptyAndSingleItemHandling tests edge cases with empty lists and single items
func TestGracefulEmptyAndSingleItemHandling(t *testing.T) {
	finder := NewInteractive("Test")
	impl := finder.(*InteractiveFinderImpl)

	t.Run("empty options list", func(t *testing.T) {
		emptyOptions := []Option{}
		if err := finder.SetOptions(emptyOptions); err != nil {
			t.Errorf("SetOptions with empty list failed: %v", err)
		}

		// Verify state is properly initialized
		if len(impl.options) != 0 {
			t.Errorf("Expected 0 options, got %d", len(impl.options))
		}

		if len(impl.filteredOptions) != 0 {
			t.Errorf("Expected 0 filtered options, got %d", len(impl.filteredOptions))
		}

		// Navigation should not crash
		impl.moveUp()
		impl.moveDown()

		// Input handling should not crash
		action := impl.handleInput("a")
		if action != "update" {
			t.Errorf("Expected 'update' action for character input with empty list, got '%s'", action)
		}

		// Filter update should not crash
		impl.updateFilter()

		// Select should return appropriate error
		_, err := finder.Select()
		if err == nil {
			t.Error("Expected error when selecting from empty list")
		}

		expectedMsg := "no options available"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})

	t.Run("single item list", func(t *testing.T) {
		singleOption := []Option{
			{Value: "single-option", Description: "The only option available"},
		}

		if err := finder.SetOptions(singleOption); err != nil {
			t.Errorf("SetOptions with single item failed: %v", err)
		}

		// Verify proper initialization
		if len(impl.options) != 1 {
			t.Errorf("Expected 1 option, got %d", len(impl.options))
		}

		if len(impl.filteredOptions) != 1 {
			t.Errorf("Expected 1 filtered option, got %d", len(impl.filteredOptions))
		}

		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex 0, got %d", impl.selectedIndex)
		}

		// Navigation should be bounded
		originalIndex := impl.selectedIndex
		impl.moveUp()
		if impl.selectedIndex != originalIndex {
			t.Errorf("Expected selectedIndex to remain %d after moveUp, got %d", originalIndex, impl.selectedIndex)
		}

		impl.moveDown()
		if impl.selectedIndex != originalIndex {
			t.Errorf("Expected selectedIndex to remain %d after moveDown, got %d", originalIndex, impl.selectedIndex)
		}

		// Filtering should work correctly
		impl.filterText = "single"
		impl.updateFilter()

		if len(impl.filteredOptions) != 1 {
			t.Errorf("Expected 1 filtered option after matching filter, got %d", len(impl.filteredOptions))
		}

		// Non-matching filter should result in empty list
		impl.filterText = "nonexistent"
		impl.updateFilter()

		if len(impl.filteredOptions) != 0 {
			t.Errorf("Expected 0 filtered options after non-matching filter, got %d", len(impl.filteredOptions))
		}
	})

	t.Run("filtered to empty results", func(t *testing.T) {
		options := []Option{
			{Value: "production-server", Description: "Production server"},
			{Value: "staging-server", Description: "Staging server"},
		}

		if err := finder.SetOptions(options); err != nil {
			t.Errorf("SetOptions failed: %v", err)
		}

		// Filter to get no results
		impl.filterText = "nonexistent-filter"
		impl.updateFilter()

		if len(impl.filteredOptions) != 0 {
			t.Errorf("Expected 0 filtered options, got %d", len(impl.filteredOptions))
		}

		// Verify selection is reset
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex reset to 0, got %d", impl.selectedIndex)
		}

		// Navigation should not crash or cause issues
		impl.moveUp()
		impl.moveDown()

		// Input handling should still work
		action := impl.handleInput("a")
		if action != "update" {
			t.Errorf("Expected 'update' action for character input with empty filtered list, got '%s'", action)
		}

		// Clearing filter should restore options
		impl.filterText = ""
		impl.updateFilter()

		if len(impl.filteredOptions) != 2 {
			t.Errorf("Expected 2 filtered options after clearing filter, got %d", len(impl.filteredOptions))
		}
	})

	t.Run("filtered to single result", func(t *testing.T) {
		options := []Option{
			{Value: "production-web", Description: "Production web server"},
			{Value: "production-db", Description: "Production database"},
			{Value: "staging-web", Description: "Staging web server"},
		}

		if err := finder.SetOptions(options); err != nil {
			t.Errorf("SetOptions failed: %v", err)
		}

		// Filter to get single result
		impl.filterText = "production-web"
		impl.updateFilter()

		if len(impl.filteredOptions) != 1 {
			t.Errorf("Expected 1 filtered option, got %d", len(impl.filteredOptions))
		}

		if impl.filteredOptions[0].Value != "production-web" {
			t.Errorf("Expected filtered option 'production-web', got '%s'", impl.filteredOptions[0].Value)
		}

		// Navigation should be bounded to single item
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex 0, got %d", impl.selectedIndex)
		}

		impl.moveUp()
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex to remain 0 after moveUp, got %d", impl.selectedIndex)
		}

		impl.moveDown()
		if impl.selectedIndex != 0 {
			t.Errorf("Expected selectedIndex to remain 0 after moveDown, got %d", impl.selectedIndex)
		}
	})
}

// TestNewInteractiveWithConsistentBindings tests the consistent bindings constructor
func TestNewInteractiveWithConsistentBindings(t *testing.T) {
	prompt := "Test consistent bindings"
	finder := NewInteractiveWithConsistentBindings(prompt)

	if finder == nil {
		t.Fatal("NewInteractiveWithConsistentBindings should return a non-nil finder")
	}

	impl, ok := finder.(*InteractiveFinderImpl)
	if !ok {
		t.Fatal("NewInteractiveWithConsistentBindings should return an InteractiveFinderImpl")
	}

	if impl.prompt != prompt {
		t.Errorf("Expected prompt '%s', got '%s'", prompt, impl.prompt)
	}

	// Verify consistent key bindings are set
	expectedBindings := KeyBindings{
		Up:     []string{"\x1b[A", "k"},
		Down:   []string{"\x1b[B", "j"},
		Select: []string{"\r", "\n"},
		Cancel: []string{"\x1b", "\x03"},
	}

	if len(impl.keyBindings.Up) != len(expectedBindings.Up) {
		t.Errorf("Expected %d Up bindings, got %d", len(expectedBindings.Up), len(impl.keyBindings.Up))
	}

	for i, expected := range expectedBindings.Up {
		if i >= len(impl.keyBindings.Up) || impl.keyBindings.Up[i] != expected {
			t.Errorf("Expected Up binding[%d] '%s', got '%s'", i, expected, impl.keyBindings.Up[i])
		}
	}

	if len(impl.keyBindings.Down) != len(expectedBindings.Down) {
		t.Errorf("Expected %d Down bindings, got %d", len(expectedBindings.Down), len(impl.keyBindings.Down))
	}

	for i, expected := range expectedBindings.Down {
		if i >= len(impl.keyBindings.Down) || impl.keyBindings.Down[i] != expected {
			t.Errorf("Expected Down binding[%d] '%s', got '%s'", i, expected, impl.keyBindings.Down[i])
		}
	}
}
