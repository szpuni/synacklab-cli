package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/config"
	"synacklab/pkg/tunnel"
)

var configuredDBs = config.TunnelConfig{Databases: []config.TunnelDatabase{
	{Name: "prod-orders", Engine: "postgres", Host: "orders.internal", Port: 6432},
	{Name: "billing", Engine: "mysql", Host: "10.20.3.15"},
}}

func noPicker(t *testing.T) func([]config.TunnelDatabase) (string, error) {
	t.Helper()
	return func([]config.TunnelDatabase) (string, error) {
		t.Fatal("picker must not be shown")
		return "", nil
	}
}

func TestResolveDBTarget_UsesTheNamedDatabase(t *testing.T) {
	got, err := resolveDBTarget(configuredDBs, []string{"prod-orders"}, dbTargetFlags{}, noPicker(t))
	require.NoError(t, err)
	assert.Equal(t, tunnel.Target{Kind: "postgres", Host: "orders.internal", Port: 6432}, got)

	got, err = resolveDBTarget(configuredDBs, []string{"billing"}, dbTargetFlags{}, noPicker(t))
	require.NoError(t, err)
	assert.Equal(t, tunnel.Target{Kind: "mysql", Host: "10.20.3.15", Port: 3306}, got, "engine default port")
}

func TestResolveDBTarget_UsesAnAdHocHost(t *testing.T) {
	got, err := resolveDBTarget(config.TunnelConfig{}, nil, dbTargetFlags{host: "db.internal", engine: "postgres"}, noPicker(t))
	require.NoError(t, err)
	assert.Equal(t, tunnel.Target{Kind: "postgres", Host: "db.internal", Port: 5432}, got)

	got, err = resolveDBTarget(config.TunnelConfig{}, nil, dbTargetFlags{host: "db.internal", engine: "mysql", port: 3307}, noPicker(t))
	require.NoError(t, err)
	assert.Equal(t, tunnel.Target{Kind: "mysql", Host: "db.internal", Port: 3307}, got)
}

func TestResolveDBTarget_OffersConfiguredDatabasesWhenNoneIsNamed(t *testing.T) {
	var offered []config.TunnelDatabase
	pick := func(dbs []config.TunnelDatabase) (string, error) {
		offered = dbs
		return "billing", nil
	}

	got, err := resolveDBTarget(configuredDBs, nil, dbTargetFlags{}, pick)
	require.NoError(t, err)
	assert.Equal(t, configuredDBs.Databases, offered)
	assert.Equal(t, tunnel.Target{Kind: "mysql", Host: "10.20.3.15", Port: 3306}, got)
}

func TestResolveDBTarget_PropagatesACancelledPick(t *testing.T) {
	pick := func([]config.TunnelDatabase) (string, error) { return "", errors.New("selection cancelled") }
	_, err := resolveDBTarget(configuredDBs, nil, dbTargetFlags{}, pick)
	assert.ErrorContains(t, err, "selection cancelled")
}

func TestResolveDBTarget_RejectsUnusableInput(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg     config.TunnelConfig
		args    []string
		flags   dbTargetFlags
		wantErr []string
	}{
		"name and --host": {
			cfg: configuredDBs, args: []string{"billing"}, flags: dbTargetFlags{host: "db", engine: "mysql"},
			wantErr: []string{"either a database name or --host"},
		},
		"--host without --engine": {
			flags:   dbTargetFlags{host: "db"},
			wantErr: []string{"--engine"},
		},
		"--engine without --host": {
			cfg: configuredDBs, args: []string{"billing"}, flags: dbTargetFlags{engine: "mysql"},
			wantErr: []string{"--host"},
		},
		"--port without --host": {
			cfg: configuredDBs, args: []string{"billing"}, flags: dbTargetFlags{port: 3307},
			wantErr: []string{"--host"},
		},
		"unsupported engine": {
			flags:   dbTargetFlags{host: "db", engine: "oracle"},
			wantErr: []string{"mysql, postgres"},
		},
		"unknown name": {
			cfg: configuredDBs, args: []string{"nope"},
			wantErr: []string{`"nope"`, "prod-orders", "billing"},
		},
		"nothing configured and no flags": {
			wantErr: []string{"tunnel.databases", "--host"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := resolveDBTarget(tc.cfg, tc.args, tc.flags, noPicker(t))
			require.Error(t, err)
			for _, want := range tc.wantErr {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}
