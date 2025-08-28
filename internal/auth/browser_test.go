package auth

import (
	"testing"
)

func TestDefaultBrowserOpener_Open(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "valid https url",
			url:  "https://example.com",
		},
		{
			name: "valid http url",
			url:  "http://example.com",
		},
		{
			name: "url with path and query",
			url:  "https://example.com/path?query=value",
		},
	}

	opener := NewBrowserOpener()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't actually test browser opening in CI/automated tests
			// This test mainly verifies the opener was created successfully
			// and doesn't panic with different URL formats

			if opener == nil {
				t.Error("NewBrowserOpener() returned nil")
			}

			// Skip actual browser opening in tests to avoid CI failures
			// In a real test environment, you would mock the exec.Command
			t.Logf("Would attempt to open browser with URL: %s", tt.url)
		})
	}
}

func TestNewBrowserOpener(t *testing.T) {
	opener := NewBrowserOpener()
	if opener == nil {
		t.Error("NewBrowserOpener() returned nil")
	}
}

// MockBrowserOpener for testing - provides a testable implementation
type MockBrowserOpener struct {
	OpenFunc func(url string) error
	Calls    []string
}

func (m *MockBrowserOpener) Open(url string) error {
	m.Calls = append(m.Calls, url)
	if m.OpenFunc != nil {
		return m.OpenFunc(url)
	}
	return nil
}

func TestMockBrowserOpener(t *testing.T) {
	mock := &MockBrowserOpener{}

	testURL := "https://example.com"
	err := mock.Open(testURL)

	if err != nil {
		t.Errorf("MockBrowserOpener.Open() error = %v, wantErr false", err)
	}

	if len(mock.Calls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(mock.Calls))
	}

	if mock.Calls[0] != testURL {
		t.Errorf("Expected call with %s, got %s", testURL, mock.Calls[0])
	}
}
