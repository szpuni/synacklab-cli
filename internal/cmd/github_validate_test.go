package cmd

import (
"os"
"path/filepath"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"

"synacklab/pkg/github"
)

func TestValidateCmd_FileNotFound(t *testing.T) {
err := runGitHubValidate(githubValidateCmd, []string{"nonexistent.yaml"})
assert.Error(t, err)
assert.Contains(t, err.Error(), "failed to read config file")
}

func TestValidConfiguration(t *testing.T) {
tempDir := t.TempDir()
configFile := filepath.Join(tempDir, "test.yaml")
config := `name: test-repo
description: A test repository
private: true
`
err := os.WriteFile(configFile, []byte(config), 0644)
require.NoError(t, err)

_, err = github.LoadRepositoryConfigFromFile(configFile)
assert.NoError(t, err)
}
