package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ReadsTunnelSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`tunnel:
  namespace: tunnels
  image: registry.corp/socat:1
  max_duration: 2h
  node_selector: {subnet: app}
  databases:
    - {name: prod-orders, engine: postgres, host: orders.internal, port: 6432}
    - {name: billing, engine: mysql, host: 10.20.3.15}
`), 0o600))

	cfg, err := LoadConfigFromPath(path)
	require.NoError(t, err)

	assert.Equal(t, TunnelConfig{
		Namespace:    "tunnels",
		Image:        "registry.corp/socat:1",
		MaxDuration:  "2h",
		NodeSelector: map[string]string{"subnet": "app"},
		Databases: []TunnelDatabase{
			{Name: "prod-orders", Engine: "postgres", Host: "orders.internal", Port: 6432},
			{Name: "billing", Engine: "mysql", Host: "10.20.3.15"},
		},
	}, cfg.Tunnel)
	require.NoError(t, cfg.Tunnel.ValidateDatabases())
}

func TestValidateDatabases_AcceptsEmptySection(t *testing.T) {
	assert.NoError(t, TunnelConfig{}.ValidateDatabases())
}

func TestValidateDatabases_NamesTheInvalidEntry(t *testing.T) {
	valid := TunnelDatabase{Name: "orders", Engine: "postgres", Host: "orders.internal"}
	for name, tc := range map[string]struct {
		tunnel  TunnelConfig
		wantErr string
	}{
		"bad engine": {
			tunnel:  TunnelConfig{Databases: []TunnelDatabase{{Name: "orders", Engine: "oracle", Host: "db"}}},
			wantErr: `database "orders"`,
		},
		"bad host": {
			tunnel:  TunnelConfig{Databases: []TunnelDatabase{{Name: "orders", Engine: "mysql", Host: "-dd"}}},
			wantErr: `database "orders"`,
		},
		"bad port": {
			tunnel:  TunnelConfig{Databases: []TunnelDatabase{{Name: "orders", Engine: "mysql", Host: "db", Port: 70000}}},
			wantErr: `database "orders"`,
		},
		"empty name": {
			tunnel:  TunnelConfig{Databases: []TunnelDatabase{{Engine: "mysql", Host: "db"}}},
			wantErr: "tunnel.databases[0]: name is required",
		},
		"duplicate name": {
			tunnel:  TunnelConfig{Databases: []TunnelDatabase{valid, valid}},
			wantErr: `duplicate database name "orders"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.tunnel.ValidateDatabases()
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestTunnelMaxDuration_DefaultsToEightHours(t *testing.T) {
	got, err := TunnelConfig{}.ParseMaxDuration()
	require.NoError(t, err)
	assert.Equal(t, 8*time.Hour, got)

	got, err = TunnelConfig{MaxDuration: "90m"}.ParseMaxDuration()
	require.NoError(t, err)
	assert.Equal(t, 90*time.Minute, got)
}

func TestTunnelMaxDuration_RejectsWhatThePodDeadlineCannotExpress(t *testing.T) {
	// activeDeadlineSeconds is a whole, positive number of seconds.
	for _, value := range []string{"forever", "0s", "-1h", "500ms", "90.5s"} {
		t.Run(value, func(t *testing.T) {
			_, err := TunnelConfig{MaxDuration: value}.ParseMaxDuration()
			assert.ErrorContains(t, err, "tunnel.max_duration")
		})
	}
}
