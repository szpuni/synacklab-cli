package fuzzy

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/term"
)

// InteractiveFinder provides an interactive fuzzy finder interface
type InteractiveFinder interface {
	// SetOptions sets the available options for selection
	SetOptions(options []Option) error

	// SetPrompt sets the display prompt
	SetPrompt(prompt string)

	// Select starts the interactive selection process
	Select() (string, error)

	// SetKeyBindings allows customization of key bindings
	SetKeyBindings(bindings KeyBindings)
}

// KeyBindings defines keyboard shortcuts for the interactive finder
type KeyBindings struct {
	Up     []string // Default: ["↑", "k"]
	Down   []string // Default: ["↓", "j"]
	Select []string // Default: ["Enter"]
	Cancel []string // Default: ["Escape", "Ctrl+C"]
}

// DefaultKeyBindings returns the default key bindings
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		Up:     []string{"\x1b[A", "k"},  // Up arrow, vim k
		Down:   []string{"\x1b[B", "j"},  // Down arrow, vim j
		Select: []string{"\r", "\n"},     // Enter
		Cancel: []string{"\x1b", "\x03"}, // Escape, Ctrl+C
	}
}

// InteractiveFinderImpl implements the InteractiveFinder interface
type InteractiveFinderImpl struct {
	options         []Option
	filteredOptions []Option
	selectedIndex   int
	filterText      string
	displayOffset   int
	prompt          string
	keyBindings     KeyBindings
	maxDisplayRows  int
	terminalWidth   int
	terminalHeight  int
}

// NewInteractive creates a new interactive fuzzy finder
func NewInteractive(prompt string) InteractiveFinder {
	finder := &InteractiveFinderImpl{
		prompt:         prompt,
		keyBindings:    DefaultKeyBindings(),
		maxDisplayRows: 10, // Default to showing 10 options at a time
		selectedIndex:  0,
		displayOffset:  0,
	}

	// Get terminal dimensions
	finder.updateTerminalSize()

	return finder
}

// NewInteractiveWithConsistentBindings creates a new interactive fuzzy finder with consistent key bindings
// This ensures all commands use the same keyboard shortcuts for a uniform user experience
func NewInteractiveWithConsistentBindings(prompt string) InteractiveFinder {
	finder := NewInteractive(prompt)

	// Set consistent key bindings across all selection scenarios
	consistentBindings := KeyBindings{
		Up:     []string{"\x1b[A", "k"},  // Up arrow, vim k
		Down:   []string{"\x1b[B", "j"},  // Down arrow, vim j
		Select: []string{"\r", "\n"},     // Enter
		Cancel: []string{"\x1b", "\x03"}, // Escape, Ctrl+C
	}

	finder.SetKeyBindings(consistentBindings)
	return finder
}

// SetOptions sets the available options for selection
func (f *InteractiveFinderImpl) SetOptions(options []Option) error {
	if options == nil {
		return fmt.Errorf("options cannot be nil")
	}

	f.options = make([]Option, len(options))
	copy(f.options, options)
	f.filteredOptions = make([]Option, len(options))
	copy(f.filteredOptions, options)
	f.selectedIndex = 0
	f.displayOffset = 0
	f.filterText = ""

	return nil
}

// SetPrompt sets the display prompt
func (f *InteractiveFinderImpl) SetPrompt(prompt string) {
	f.prompt = prompt
}

// SetKeyBindings allows customization of key bindings
func (f *InteractiveFinderImpl) SetKeyBindings(bindings KeyBindings) {
	f.keyBindings = bindings
}

// Select starts the interactive selection process
func (f *InteractiveFinderImpl) Select() (string, error) {
	if len(f.options) == 0 {
		return "", fmt.Errorf("no options available")
	}

	// Check if terminal supports interactive mode
	if !f.isTerminalSupported() {
		return f.fallbackSelect()
	}

	// Save terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return f.fallbackSelect()
	}
	defer func() {
		if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
			// Log error but don't fail the operation
			fmt.Fprintf(os.Stderr, "Warning: failed to restore terminal state: %v\n", err)
		}
	}()

	// Hide cursor and clear screen
	fmt.Print("\x1b[?25l\x1b[2J\x1b[H")
	defer fmt.Print("\x1b[?25h") // Show cursor on exit

	// Initial render
	f.render()

	// Input loop
	buffer := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		input := string(buffer[:n])
		action := f.handleInput(input)

		switch action {
		case "select":
			if len(f.filteredOptions) > 0 && f.selectedIndex < len(f.filteredOptions) {
				return f.filteredOptions[f.selectedIndex].Value, nil
			}
		case "cancel":
			return "", fmt.Errorf("selection cancelled")
		case "update":
			f.render()
		}
	}
}

// isTerminalSupported checks if the terminal supports interactive features
func (f *InteractiveFinderImpl) isTerminalSupported() bool {
	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	// Check if stdout is a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}

	// Check terminal type
	termType := os.Getenv("TERM")
	if termType == "" || termType == "dumb" {
		return false
	}

	return true
}

// fallbackSelect provides a simple selection for unsupported terminals
func (f *InteractiveFinderImpl) fallbackSelect() (string, error) {
	// Use the existing simple fuzzy finder as fallback
	finder := New(f.prompt)
	for _, option := range f.options {
		finder.AddOption(option.Value, option.Description)
	}
	return finder.SelectWithFilter()
}

// handleInput processes keyboard input and returns the action to take
func (f *InteractiveFinderImpl) handleInput(input string) string {
	// Check for special keys first
	for _, key := range f.keyBindings.Up {
		if input == key {
			f.moveUp()
			return "update"
		}
	}

	for _, key := range f.keyBindings.Down {
		if input == key {
			f.moveDown()
			return "update"
		}
	}

	for _, key := range f.keyBindings.Select {
		if input == key {
			return "select"
		}
	}

	for _, key := range f.keyBindings.Cancel {
		if input == key {
			return "cancel"
		}
	}

	// Handle backspace
	if input == "\x7f" || input == "\b" {
		if len(f.filterText) > 0 {
			f.filterText = f.filterText[:len(f.filterText)-1]
			f.updateFilter()
			return "update"
		}
		return ""
	}

	// Handle printable characters
	if len(input) == 1 && input[0] >= 32 && input[0] <= 126 {
		f.filterText += input
		f.updateFilter()
		return "update"
	}

	return ""
}

// moveUp moves the selection up
func (f *InteractiveFinderImpl) moveUp() {
	if f.selectedIndex > 0 {
		f.selectedIndex--
		f.updateDisplayOffset()
	}
}

// moveDown moves the selection down
func (f *InteractiveFinderImpl) moveDown() {
	if f.selectedIndex < len(f.filteredOptions)-1 {
		f.selectedIndex++
		f.updateDisplayOffset()
	}
}

// updateDisplayOffset adjusts the display offset for scrolling
func (f *InteractiveFinderImpl) updateDisplayOffset() {
	if f.selectedIndex < f.displayOffset {
		f.displayOffset = f.selectedIndex
	} else if f.selectedIndex >= f.displayOffset+f.maxDisplayRows {
		f.displayOffset = f.selectedIndex - f.maxDisplayRows + 1
	}
}

// updateFilter applies the current filter text to options
func (f *InteractiveFinderImpl) updateFilter() {
	if f.filterText == "" {
		f.filteredOptions = make([]Option, len(f.options))
		copy(f.filteredOptions, f.options)
	} else {
		f.filteredOptions = f.filterOptions(f.filterText)
	}

	// Reset selection to first item
	f.selectedIndex = 0
	f.displayOffset = 0
}

// filterOptions filters options based on the filter text
func (f *InteractiveFinderImpl) filterOptions(filter string) []Option {
	filter = strings.ToLower(filter)
	var filtered []Option

	for _, option := range f.options {
		// Check if filter matches value or description
		if strings.Contains(strings.ToLower(option.Value), filter) ||
			strings.Contains(strings.ToLower(option.Description), filter) {
			filtered = append(filtered, option)
		}
	}

	return filtered
}

// render displays the current state of the finder
func (f *InteractiveFinderImpl) render() {
	// Clear screen and move to top
	fmt.Print("\x1b[2J\x1b[H")

	// Display prompt
	fmt.Printf("%s\n", f.prompt)

	// Display filter input
	fmt.Printf("Filter: %s\n", f.filterText)
	fmt.Println(strings.Repeat("-", f.getTerminalWidth()))

	// Display header if we have AWS profile-like options (check for account_id metadata)
	if len(f.filteredOptions) > 0 && f.filteredOptions[0].Metadata != nil {
		if _, hasAccountID := f.filteredOptions[0].Metadata["account_id"]; hasAccountID {
			// Calculate max profile name length for header alignment
			maxValueLen := 0
			for i := 0; i < len(f.filteredOptions) && i < f.maxDisplayRows; i++ {
				if len(f.filteredOptions[i].Value) > maxValueLen {
					maxValueLen = len(f.filteredOptions[i].Value)
				}
			}
			fmt.Printf("  %-*s  │  %s\n", maxValueLen, "Profile", "Details")
			fmt.Printf("  %s  │  %s\n", strings.Repeat("─", maxValueLen), strings.Repeat("─", 50))
		}
	}

	// Display options
	if len(f.filteredOptions) == 0 {
		if f.filterText == "" {
			fmt.Println("No options available")
		} else {
			fmt.Println("No matches found")
		}
		return
	}

	// Calculate display range
	start := f.displayOffset
	end := start + f.maxDisplayRows
	if end > len(f.filteredOptions) {
		end = len(f.filteredOptions)
	}

	// Calculate max profile name length for alignment
	maxValueLen := 0
	for i := start; i < end; i++ {
		if len(f.filteredOptions[i].Value) > maxValueLen {
			maxValueLen = len(f.filteredOptions[i].Value)
		}
	}

	// Display visible options with consistent metadata formatting
	for i := start; i < end; i++ {
		option := f.filteredOptions[i]
		prefix := "  "
		if i == f.selectedIndex {
			prefix = "> "
		}

		// Display option value with consistent formatting and alignment
		fmt.Printf("%s%-*s", prefix, maxValueLen, option.Value)

		// Display description if available with proper spacing
		if option.Description != "" {
			fmt.Printf("  │  %s", option.Description)
		}

		// Display additional metadata if available
		if len(option.Metadata) > 0 {
			// Show key metadata in a consistent format
			if current, exists := option.Metadata["current"]; exists && current == "true" {
				fmt.Print(" (current)")
			}
		}

		fmt.Println()
	}

	// Display scroll indicator if needed
	if len(f.filteredOptions) > f.maxDisplayRows {
		fmt.Printf("\n[%d/%d] Use ↑↓ or j/k to navigate", f.selectedIndex+1, len(f.filteredOptions))
	}

	// Display consistent help text with keyboard shortcuts
	fmt.Println("\nPress Enter to select, Escape to cancel, ↑↓ or j/k to navigate")
}

// updateTerminalSize gets the current terminal dimensions
func (f *InteractiveFinderImpl) updateTerminalSize() {
	width, height, err := f.getTerminalSize()
	if err != nil {
		// Default fallback values
		f.terminalWidth = 80
		f.terminalHeight = 24
		f.maxDisplayRows = 10
	} else {
		f.terminalWidth = width
		f.terminalHeight = height
		// Reserve space for prompt, filter, separator, and help text
		f.maxDisplayRows = height - 6
		if f.maxDisplayRows < 3 {
			f.maxDisplayRows = 3
		}
	}
}

// getTerminalSize returns the terminal width and height
func (f *InteractiveFinderImpl) getTerminalSize() (int, int, error) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}

	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(os.Stdin.Fd()),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		return 0, 0, errno
	}

	return int(ws.Col), int(ws.Row), nil
}

// getTerminalWidth returns the terminal width with a fallback
func (f *InteractiveFinderImpl) getTerminalWidth() int {
	if f.terminalWidth > 0 {
		return f.terminalWidth
	}
	return 80 // fallback
}
