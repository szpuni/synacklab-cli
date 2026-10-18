package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/config"
	"synacklab/pkg/tunnel"
)

func TestTunnelServe_ResolvesTargetsExactlyLikeTunnelDB(t *testing.T) {
	srv := tunnelServerConfig(configuredDBs, nil, "")

	got, err := srv.Resolve(tunnel.Request{Name: "prod-orders"})
	require.NoError(t, err)
	assert.Equal(t, tunnel.Target{Kind: "postgres", Host: "orders.internal", Port: 6432}, got)

	got, err = srv.Resolve(tunnel.Request{Host: "db.internal", Engine: "mysql"})
	require.NoError(t, err)
	assert.Equal(t, tunnel.Target{Kind: "mysql", Host: "db.internal", Port: 3306}, got)

	for _, tc := range []struct {
		req  tunnel.Request
		args []string
		dbf  dbTargetFlags
	}{
		{tunnel.Request{Name: "nope"}, []string{"nope"}, dbTargetFlags{}},
		{tunnel.Request{Name: "billing", Host: "x"}, []string{"billing"}, dbTargetFlags{host: "x"}},
		{tunnel.Request{Host: "db.internal", Engine: "redis"}, nil, dbTargetFlags{host: "db.internal", engine: "redis"}},
		{tunnel.Request{Host: "-oops", Engine: "mysql"}, nil, dbTargetFlags{host: "-oops", engine: "mysql"}},
	} {
		_, err := srv.Resolve(tc.req)
		_, cliErr := resolveDBTarget(configuredDBs, tc.args, tc.dbf, noPicker(t))
		require.Error(t, cliErr)
		assert.EqualError(t, err, cliErr.Error())
	}
}

func TestTunnelServe_AsksForATargetInsteadOfShowingThePicker(t *testing.T) {
	_, err := tunnelServerConfig(configuredDBs, nil, "").Resolve(tunnel.Request{})
	assert.EqualError(t, err, "choose a configured database or enter a host")
}

func TestTunnelServe_ListsConfiguredDatabasesWithTheirPorts(t *testing.T) {
	srv := tunnelServerConfig(configuredDBs, []string{"dev", "prod"}, "dev")

	assert.Equal(t, []tunnel.Database{
		{Name: "prod-orders", Engine: "postgres", Host: "orders.internal", Port: 6432},
		{Name: "billing", Engine: "mysql", Host: "10.20.3.15", Port: 3306},
	}, srv.Databases)
	assert.Empty(t, srv.DatabaseErr)
	assert.Equal(t, []string{"dev", "prod"}, srv.Contexts)
}

func TestTunnelServe_StillOffersAdHocTargetsWhenConfiguredDatabasesAreInvalid(t *testing.T) {
	broken := config.TunnelConfig{Databases: []config.TunnelDatabase{{Name: "bad", Engine: "redis", Host: "x"}}}
	srv := tunnelServerConfig(broken, nil, "")

	assert.Contains(t, srv.DatabaseErr, `database "bad"`)
	assert.Empty(t, srv.Databases)
	_, err := srv.Resolve(tunnel.Request{Host: "db.internal", Engine: "postgres"})
	assert.NoError(t, err)
}

func TestTunnelServe_PreselectsContextAndNamespaceLikeTheCLI(t *testing.T) {
	oldCtx, oldNS := tunnelContext, tunnelNamespace
	t.Cleanup(func() { tunnelContext, tunnelNamespace = oldCtx, oldNS })

	tunnelContext, tunnelNamespace = "", ""
	srv := tunnelServerConfig(config.TunnelConfig{}, []string{"dev", "prod"}, "dev")
	assert.Equal(t, tunnel.Request{Context: "dev", Namespace: tunnel.DefaultNamespace}, srv.Defaults)

	srv = tunnelServerConfig(config.TunnelConfig{Namespace: "team-tunnels"}, nil, "dev")
	assert.Equal(t, "team-tunnels", srv.Defaults.Namespace)

	tunnelContext, tunnelNamespace = "prod", "mine"
	srv = tunnelServerConfig(config.TunnelConfig{Namespace: "team-tunnels"}, nil, "dev")
	assert.Equal(t, tunnel.Request{Context: "prod", Namespace: "mine"}, srv.Defaults)
}

func TestTunnelServe_NonLoopbackBindFailsBeforeListening(t *testing.T) {
	old := tunnelServeBind
	t.Cleanup(func() { tunnelServeBind = old })
	tunnelServeBind = "0.0.0.0"

	assert.ErrorContains(t, runTunnelServe(nil, nil), "local-only")
}
