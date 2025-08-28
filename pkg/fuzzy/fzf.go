package fuzzy

import (
	"fmt"
	"io"
	"os"
	"strings"

	fzf "github.com/junegunn/fzf/src"
)

// FzfRunner defines the interface for running fzf
type FzfRunner interface {
	Run(opts *fzf.Options) (int, error)
}

// DefaultFzfRunner implements the FzfRunner interface using the real fzf library
type DefaultFzfRunner struct{}

// Run executes fzf with the given options
func (r *DefaultFzfRunner) Run(opts *fzf.Options) (int, error) {
	return fzf.Run(opts)
}

// FzfFinder implements fuzzy finding using the fzf library
type FzfFinder struct {
	options []Option
	prompt  string
	runner  FzfRunner
}

// NewFzf creates a new fzf-style fuzzy finder
func NewFzf(prompt string) *FzfFinder {
	return &FzfFinder{
		prompt:  prompt,
		options: make([]Option, 0),
		runner:  &DefaultFzfRunner{},
	}
}

// NewFzfWithRunner creates a new fzf-style fuzzy finder with a custom runner (for testing)
func NewFzfWithRunner(prompt string, runner FzfRunner) *FzfFinder {
	return &FzfFinder{
		prompt:  prompt,
		options: make([]Option, 0),
		runner:  runner,
	}
}

// SetOptions sets the available options for selection
func (f *FzfFinder) SetOptions(options []Option) error {
	if options == nil {
		return fmt.Errorf("options cannot be nil")
	}

	f.options = make([]Option, len(options))
	copy(f.options, options)
	return nil
}

// SetPrompt sets the display prompt
func (f *FzfFinder) SetPrompt(prompt string) {
	f.prompt = prompt
}

// Select starts the fuzzy selection process using the fzf library
func (f *FzfFinder) Select() (string, error) {
	if len(f.options) == 0 {
		return "", fmt.Errorf("no options available")
	}

	// Create a temporary file with the options
	tmpFile, err := os.CreateTemp("", "fzf-options-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name()) // Ignore cleanup errors
	}()
	defer func() {
		_ = tmpFile.Close() // Ignore close errors
	}()

	// Write options to temporary file
	for _, option := range f.options {
		displayText := option.Value
		if option.Description != "" {
			displayText = fmt.Sprintf("%s  │  %s", option.Value, option.Description)
		}
		if _, err := fmt.Fprintln(tmpFile, displayText); err != nil {
			return "", fmt.Errorf("failed to write option to file: %w", err)
		}
	}

	// Close the file so fzf can read it
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Prepare fzf arguments
	args := []string{
		"--prompt=" + f.prompt + " ",
		"--height=10",
		"--layout=default",
		"--no-multi",
		"--cycle",
		"--hscroll",
		"--hscroll-off=10",
		"--tabstop=8",
		"--clear",
		"--extended",
		"--algo=v2",
		"--tiebreak=length",
		"--sort=1000",
		"--no-mouse",
		"--no-reverse",
		"--border=none",
	}

	// Parse options and run fzf
	opts, err := fzf.ParseOptions(true, args)
	if err != nil {
		return "", fmt.Errorf("failed to parse fzf options: %w", err)
	}

	// Redirect stdin to read from our temporary file
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	tmpFileForReading, err := os.Open(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to open temporary file for reading: %w", err)
	}
	defer func() {
		_ = tmpFileForReading.Close() // Ignore close errors
	}()

	os.Stdin = tmpFileForReading

	// Capture stdout to get the selected result
	originalStdout := os.Stdout
	defer func() { os.Stdout = originalStdout }()

	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("failed to create pipe: %w", err)
	}
	defer func() {
		_ = r.Close() // Ignore close errors
	}()
	defer func() {
		_ = w.Close() // Ignore close errors
	}()

	os.Stdout = w

	// Run fzf
	exitCode, err := f.runner.Run(opts)

	// Restore stdout before reading result
	_ = w.Close() // Ignore close errors
	os.Stdout = originalStdout

	if err != nil {
		// Fallback to simple finder if fzf fails
		return f.fallbackSelect()
	}

	if exitCode != fzf.ExitOk {
		return "", fmt.Errorf("fzf selection cancelled or failed")
	}

	// Read the result
	result, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read fzf result: %w", err)
	}

	selectedText := strings.TrimSpace(string(result))
	if selectedText == "" {
		return "", fmt.Errorf("no selection made")
	}

	// Extract the original value from the selected text
	// The format is "value  │  description" so we need to extract just the value
	parts := strings.Split(selectedText, "  │  ")
	selectedValue := strings.TrimSpace(parts[0])

	// Find the matching option to return the original value
	for _, option := range f.options {
		if option.Value == selectedValue {
			return option.Value, nil
		}
	}

	// Fallback: return the selected text as-is
	return selectedValue, nil
}

// FzfFinderInterface defines the interface for fzf-based fuzzy finding
type FzfFinderInterface interface {
	SetOptions(options []Option) error
	SetPrompt(prompt string)
	Select() (string, error)
}

// fallbackSelect provides a simple selection for when fzf fails
func (f *FzfFinder) fallbackSelect() (string, error) {
	// Use the existing simple fuzzy finder as fallback
	finder := New(f.prompt)
	for _, option := range f.options {
		finder.AddOption(option.Value, option.Description)
	}
	return finder.SelectWithFilter()
}

// Ensure FzfFinder implements the interface
var _ FzfFinderInterface = (*FzfFinder)(nil)
