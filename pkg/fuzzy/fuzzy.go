package fuzzy

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Option represents a selectable option in the fuzzy finder
type Option struct {
	Value       string
	Description string
}

// Finder represents a fuzzy finder instance
type Finder struct {
	prompt  string
	options []Option
}

// New creates a new fuzzy finder with the given prompt
func New(prompt string) *Finder {
	return &Finder{
		prompt:  prompt,
		options: make([]Option, 0),
	}
}

// AddOption adds an option to the fuzzy finder
func (f *Finder) AddOption(value, description string) {
	f.options = append(f.options, Option{
		Value:       value,
		Description: description,
	})
}

// Select displays options and allows user to select one by number
func (f *Finder) Select() (string, error) {
	if len(f.options) == 0 {
		return "", fmt.Errorf("no options available")
	}

	// Display prompt
	fmt.Println(f.prompt)
	fmt.Println(strings.Repeat("-", len(f.prompt)))

	// Display options
	for i, option := range f.options {
		fmt.Printf("%d. %s", i+1, option.Value)
		if option.Description != "" {
			fmt.Printf(" - %s", option.Description)
		}
		fmt.Println()
	}

	// Get user selection
	fmt.Print("\nSelect option (1-" + strconv.Itoa(len(f.options)) + "): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	// Parse selection
	input = strings.TrimSpace(input)
	selection, err := strconv.Atoi(input)
	if err != nil {
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	if selection < 1 || selection > len(f.options) {
		return "", fmt.Errorf("selection out of range: %d", selection)
	}

	return f.options[selection-1].Value, nil
}

// SelectWithFilter provides a more advanced selection with filtering capability
func (f *Finder) SelectWithFilter() (string, error) {
	if len(f.options) == 0 {
		return "", fmt.Errorf("no options available")
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		// Display prompt
		fmt.Println(f.prompt)
		fmt.Println("Type to filter options, or enter a number to select:")
		fmt.Println(strings.Repeat("-", 50))

		// Get filter input
		fmt.Print("Filter/Select: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Try to parse as number first
		if selection, err := strconv.Atoi(input); err == nil {
			if selection >= 1 && selection <= len(f.options) {
				return f.options[selection-1].Value, nil
			}
			fmt.Printf("Selection %d is out of range (1-%d)\n\n", selection, len(f.options))
			continue
		}

		// Filter options
		filtered := f.filterOptions(input)
		if len(filtered) == 0 {
			fmt.Printf("No options match filter: %s\n\n", input)
			continue
		}

		// Display filtered options
		fmt.Printf("\nFiltered options (matching '%s'):\n", input)
		for i, option := range filtered {
			fmt.Printf("%d. %s", i+1, option.Value)
			if option.Description != "" {
				fmt.Printf(" - %s", option.Description)
			}
			fmt.Println()
		}

		// If only one match, auto-select it
		if len(filtered) == 1 {
			fmt.Printf("\nAuto-selecting: %s\n", filtered[0].Value)
			return filtered[0].Value, nil
		}

		// Get selection from filtered list
		fmt.Print("\nSelect from filtered options (1-" + strconv.Itoa(len(filtered)) + "), or press Enter to filter again: ")
		selectionInput, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read selection: %w", err)
		}

		selectionInput = strings.TrimSpace(selectionInput)
		if selectionInput == "" {
			fmt.Println() // Add blank line for readability
			continue
		}

		selection, err := strconv.Atoi(selectionInput)
		if err != nil {
			fmt.Printf("Invalid selection: %s\n\n", selectionInput)
			continue
		}

		if selection < 1 || selection > len(filtered) {
			fmt.Printf("Selection %d is out of range (1-%d)\n\n", selection, len(filtered))
			continue
		}

		return filtered[selection-1].Value, nil
	}
}

// filterOptions filters options based on the input string
func (f *Finder) filterOptions(filter string) []Option {
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

// GetOptions returns all available options
func (f *Finder) GetOptions() []Option {
	return f.options
}

// Clear removes all options from the finder
func (f *Finder) Clear() {
	f.options = make([]Option, 0)
}

// SetPrompt updates the prompt message
func (f *Finder) SetPrompt(prompt string) {
	f.prompt = prompt
}
