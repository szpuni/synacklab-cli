package runbook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkfile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
}

func TestListMarkdownFiles_FlatDirectoryListsOnlyMarkdown(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "a.md"))
	mkfile(t, filepath.Join(root, "b.md"))
	mkfile(t, filepath.Join(root, "notes.txt"))

	tree, err := ListMarkdownFiles(root)
	require.NoError(t, err)

	names := childNames(tree)
	assert.ElementsMatch(t, []string{"a.md", "b.md"}, names)
}

func TestListMarkdownFiles_NestedDirectoriesArePruned(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "top.md"))
	mkfile(t, filepath.Join(root, "sub", "nested.md"))
	mkfile(t, filepath.Join(root, "empty-of-md", "notes.txt"))

	tree, err := ListMarkdownFiles(root)
	require.NoError(t, err)

	names := childNames(tree)
	assert.ElementsMatch(t, []string{"top.md", "sub"}, names, "a directory with no markdown descendants must be pruned")

	var subDir *FileNode
	for _, c := range tree.Children {
		if c.Name == "sub" {
			subDir = c
		}
	}
	require.NotNil(t, subDir)
	assert.True(t, subDir.Dir)
	assert.Equal(t, []string{"nested.md"}, childNames(subDir))
	assert.Equal(t, "sub/nested.md", subDir.Children[0].Path)
}

func TestListMarkdownFiles_SkipsHiddenEntries(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "visible.md"))
	mkfile(t, filepath.Join(root, ".hidden.md"))
	mkfile(t, filepath.Join(root, ".git", "config.md"))

	tree, err := ListMarkdownFiles(root)
	require.NoError(t, err)

	assert.Equal(t, []string{"visible.md"}, childNames(tree))
}

func TestListMarkdownFiles_EmptyDirectoryHasNoChildren(t *testing.T) {
	root := t.TempDir()

	tree, err := ListMarkdownFiles(root)
	require.NoError(t, err)
	assert.Empty(t, tree.Children)
}

func childNames(n *FileNode) []string {
	names := make([]string, len(n.Children))
	for i, c := range n.Children {
		names[i] = c.Name
	}
	return names
}

func TestResolveWorkspaceFile_JoinsRelativePath(t *testing.T) {
	got := resolveWorkspaceFile("/root", "sub/deploy.md")
	assert.Equal(t, filepath.Join("/root", "sub", "deploy.md"), got)
}

func TestResolveWorkspaceFile_PreventsPathTraversal(t *testing.T) {
	got := resolveWorkspaceFile("/root", "../../etc/passwd")
	assert.Equal(t, filepath.Join("/root", "etc", "passwd"), got)
	assert.True(t, isWithinRoot("/root", got))
}

func TestResolveWorkspaceFile_AbsolutePathIsTreatedAsRelative(t *testing.T) {
	got := resolveWorkspaceFile("/root", "/etc/passwd")
	assert.Equal(t, filepath.Join("/root", "etc", "passwd"), got)
}

func TestRelativeToRoot(t *testing.T) {
	assert.Equal(t, "sub/deploy.md", relativeToRoot("/root", filepath.Join("/root", "sub", "deploy.md")))
	assert.Equal(t, "", relativeToRoot("", "/anything"))
}
