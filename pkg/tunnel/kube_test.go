package tunnel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/tunnel"
)

func TestContexts_ListsKubeconfigContextsAndTheCurrentOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(`apiVersion: v1
kind: Config
current-context: staging
clusters: [{name: c, cluster: {server: "https://127.0.0.1:6443"}}]
users: [{name: u, user: {token: secret}}]
contexts:
- {name: prod, context: {cluster: c, user: u}}
- {name: staging, context: {cluster: c, user: u}}
- {name: dev, context: {cluster: c, user: u}}
`), 0o600))
	t.Setenv("KUBECONFIG", path)

	names, current, err := tunnel.Contexts()
	require.NoError(t, err)
	assert.Equal(t, []string{"dev", "prod", "staging"}, names)
	assert.Equal(t, "staging", current)
}
