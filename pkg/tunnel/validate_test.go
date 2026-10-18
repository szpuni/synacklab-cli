package tunnel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"synacklab/pkg/tunnel"
)

func TestValidateHost_AcceptsHostnamesAndIPs(t *testing.T) {
	for _, host := range []string{
		"orders.internal",
		"db",
		"prod-db.cluster-abc123.eu-west-1.rds.amazonaws.com",
		"Orders.Internal",
		"10.20.3.15",
		"fd00::15",
	} {
		t.Run(host, func(t *testing.T) {
			assert.NoError(t, tunnel.ValidateHost(host))
		})
	}
}

func TestValidateHost_RejectsValuesThatAreNotAHost(t *testing.T) {
	for name, host := range map[string]string{
		"empty":             "",
		"socat option":      "-dd",
		"leading dash":      "-db.internal",
		"space":             "db internal",
		"socat address":     "db:5432",
		"shell":             "db;rm -rf /",
		"label too long":    "a234567890123456789012345678901234567890123456789012345678901234.internal",
		"trailing dash":     "db-.internal",
		"empty label":       "db..internal",
		"url":               "postgres://db.internal",
		"bracketed ipv6":    "[fd00::15]",
		"underscore":        "db_1.internal",
		"only dots":         "...",
		"newline":           "db\n.internal",
		"comma socat opts":  "db,fork",
		"ipv6 with zone id": "fe80::1%eth0",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, tunnel.ValidateHost(host))
		})
	}
}

func TestValidatePort_AcceptsOnlyTCPPortRange(t *testing.T) {
	for port, ok := range map[int]bool{0: false, -1: false, 1: true, 5432: true, 65535: true, 65536: false} {
		err := tunnel.ValidatePort(port)
		if ok {
			assert.NoError(t, err, "port %d", port)
		} else {
			assert.Error(t, err, "port %d", port)
		}
	}
}

func TestDefaultPort_KnowsSupportedDatabaseEngines(t *testing.T) {
	port, err := tunnel.DefaultPort("postgres")
	assert.NoError(t, err)
	assert.Equal(t, 5432, port)

	port, err = tunnel.DefaultPort("mysql")
	assert.NoError(t, err)
	assert.Equal(t, 3306, port)

	_, err = tunnel.DefaultPort("oracle")
	assert.ErrorContains(t, err, "mysql, postgres")
}

func TestSanitizeUser_ProducesAPodNameSafeFragment(t *testing.T) {
	for in, want := range map[string]string{
		"wojtek":                         "wojtek",
		"Wojtek.Szpunar":                 "wojtek-szpunar",
		`CORP\jdoe`:                      "corp-jdoe",
		"jdoe@example.com":               "jdoe-example-com",
		"--admin--":                      "admin",
		"":                               "user",
		"@@@":                            "user",
		"averyveryverylongusername12345": "averyveryverylonguse",
		"averyveryverylongus-rname":      "averyveryverylongus",
	} {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, tunnel.SanitizeUser(in))
		})
	}
}
