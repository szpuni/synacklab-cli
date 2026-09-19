package runbook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileNode is one entry in the workspace file tree served to the frontend's
// file browser. Only directories that (transitively) contain at least one
// *.md file are included, and only *.md files appear as leaves.
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"` // relative to root, "/"-separated; "" for the root itself
	Dir      bool        `json:"dir"`
	Children []*FileNode `json:"children,omitempty"`
}

// ListMarkdownFiles walks root and returns its FileNode tree.
func ListMarkdownFiles(root string) (*FileNode, error) {
	return walkMarkdownDir(root, "")
}

func walkMarkdownDir(absDir, relDir string) (*FileNode, error) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, &Error{Type: ErrorTypeExecution, Message: "failed to read directory " + absDir, Cause: err}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	node := &FileNode{Name: filepath.Base(absDir), Path: relDir, Dir: true}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		childRel := e.Name()
		if relDir != "" {
			childRel = relDir + "/" + e.Name()
		}

		if e.IsDir() {
			child, err := walkMarkdownDir(filepath.Join(absDir, e.Name()), childRel)
			if err != nil {
				return nil, err
			}
			if len(child.Children) > 0 {
				node.Children = append(node.Children, child)
			}
			continue
		}

		if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			node.Children = append(node.Children, &FileNode{Name: e.Name(), Path: childRel})
		}
	}

	return node, nil
}

// resolveWorkspaceFile joins root with a client-supplied relative path,
// neutralizing any ".." traversal by cleaning it as if rooted at "/" first
// (a leading ".." can't escape "/", so it can't escape root either).
func resolveWorkspaceFile(root, rel string) string {
	safeRel := filepath.Clean(string(filepath.Separator) + filepath.FromSlash(rel))
	return filepath.Join(root, safeRel)
}

func isWithinRoot(root, path string) bool {
	rootClean := filepath.Clean(root) + string(filepath.Separator)
	return strings.HasPrefix(filepath.Clean(path)+string(filepath.Separator), rootClean)
}

// relativeToRoot returns path relative to root using "/" separators, or ""
// if root is unset (single-file/non-workspace mode) or path falls outside it.
func relativeToRoot(root, path string) string {
	if root == "" {
		return ""
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

func (s *Server) handleGetFiles(w http.ResponseWriter, _ *http.Request) {
	if s.root == "" {
		s.writeError(w, http.StatusNotFound, "workspace mode not enabled")
		return
	}
	tree, err := ListMarkdownFiles(s.root)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, tree)
}

type openRequest struct {
	File  string `json:"file"`
	Force bool   `json:"force"`
}

func (s *Server) handlePostOpen(w http.ResponseWriter, r *http.Request) {
	if s.root == "" {
		s.writeError(w, http.StatusNotFound, "workspace mode not enabled")
		return
	}

	var req openRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
		return
	}
	if !strings.EqualFold(filepath.Ext(req.File), ".md") {
		s.writeError(w, http.StatusBadRequest, "only .md files can be opened")
		return
	}

	absPath := resolveWorkspaceFile(s.root, req.File)
	if !isWithinRoot(s.root, absPath) {
		s.writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	// Refusing a switch while a step is running takes precedence over the
	// vars/history confirmation check below: canceling or waiting out that
	// execution is a precondition, not something force should paper over
	// (Requirement 6.4).
	if running, ok := s.registry.runningInfo(); ok {
		s.writeError(w, http.StatusConflict, fmt.Sprintf(
			"step %q is still running (execution %s); cancel or wait for it before switching runbooks", running.stepName, running.id))
		return
	}

	activeDocument, activeStore := s.active.get()
	if activeDocument != nil && activeDocument.Path == absPath {
		relPath := relativeToRoot(s.root, absPath)
		s.writeJSON(w, http.StatusOK, map[string]any{"opened": relPath, "unchanged": true})
		return
	}

	if activeStore != nil {
		sess := activeStore.Get()
		if !req.Force && (len(sess.Vars) > 0 || len(sess.History) > 0) {
			msg := "switching runbooks will discard the current session's captured variables and history"
			s.logger.Warn("%s", msg)
			s.writeJSON(w, http.StatusConflict, map[string]any{"error": msg, "requires_confirmation": true})
			return
		}
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		s.writeError(w, http.StatusNotFound, "file not found: "+req.File)
		return
	}

	source, err := os.ReadFile(absPath)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "failed to read "+req.File+": "+err.Error())
		return
	}
	doc, err := (&GoldmarkParser{}).Parse(source, absPath)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to parse "+req.File+": "+err.Error())
		return
	}

	cwd := s.defaultCwd
	if cwd == "" {
		cwd = filepath.Dir(absPath)
	}
	store := NewSessionStore(NewID(), absPath, cwd)
	s.active.set(doc, store)

	relPath := relativeToRoot(s.root, absPath)
	s.logger.Info("opened %s", relPath)
	s.writeJSON(w, http.StatusOK, map[string]string{"opened": relPath})
}
