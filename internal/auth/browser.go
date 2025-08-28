package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// BrowserOpener defines the interface for opening URLs in the default browser
type BrowserOpener interface {
	Open(url string) error
}

// DefaultBrowserOpener implements cross-platform browser opening
type DefaultBrowserOpener struct{}

// NewBrowserOpener creates a new browser opener instance
func NewBrowserOpener() *DefaultBrowserOpener {
	return &DefaultBrowserOpener{}
}

// Open opens the specified URL in the default browser
func (b *DefaultBrowserOpener) Open(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
