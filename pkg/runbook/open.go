package runbook

import (
	"fmt"
	"os"
	"path/filepath"
)

// OpenRunbook reads and parses the runbook at path and starts a fresh
// Session for it. The session's default cwd — where it starts, and where
// Reset puts it back — is cwdOverride if set, else the document's own
// directory. Both paths are made absolute so the document stays comparable
// to an absolute workspace root.
func OpenRunbook(path, cwdOverride string) (*Document, SessionStore, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("failed to resolve %s: %v", path, err), Cause: err}
	}
	source, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("failed to read %s: %v", path, err), Cause: err}
	}
	doc, err := (&GoldmarkParser{}).Parse(source, absPath)
	if err != nil {
		return nil, nil, err
	}

	cwd := doc.Dir
	if cwdOverride != "" {
		if cwd, err = filepath.Abs(cwdOverride); err != nil {
			return nil, nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("failed to resolve working directory %s: %v", cwdOverride, err), Cause: err}
		}
	}
	return doc, NewSessionStore(NewID(), absPath, cwd), nil
}
