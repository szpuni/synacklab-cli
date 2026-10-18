package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"synacklab/pkg/config"
	"synacklab/pkg/fuzzy"
	"synacklab/pkg/tunnel"
)

// dbTargetFlags are the ad hoc target flags of `tunnel db`.
type dbTargetFlags struct {
	host   string
	engine string
	port   int
}

var (
	dbFlags     dbTargetFlags
	dbLocalPort int
)

var tunnelDBCmd = &cobra.Command{
	Use:   "db [name]",
	Short: "Tunnel to a MySQL or PostgreSQL database",
	Long: `db forwards a local port to a database through a short-lived socat pod.

Choose the database by:
  - name:        synacklab tunnel db prod-orders   (from tunnel.databases in config)
  - flags:       synacklab tunnel db --engine postgres --host orders.internal
  - interactive: synacklab tunnel db                (pick from tunnel.databases)

The local port defaults to the database port (postgres 5432, mysql 3306).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTunnelDB,
	// Errors here are runtime failures (cluster, network), not flag misuse.
	SilenceUsage: true,
}

func init() {
	tunnelDBCmd.Flags().StringVar(&dbFlags.host, "host", "", "database hostname or IP (ad hoc target, requires --engine)")
	tunnelDBCmd.Flags().StringVar(&dbFlags.engine, "engine", "", "database engine for --host: mysql or postgres")
	tunnelDBCmd.Flags().IntVar(&dbFlags.port, "port", 0, "database port for --host (default: engine port)")
	tunnelDBCmd.Flags().IntVar(&dbLocalPort, "local-port", 0, "local port to listen on (default: database port)")
	tunnelCmd.AddCommand(tunnelDBCmd)
}

func runTunnelDB(_ *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if err := cfg.ValidateTunnel(); err != nil {
		return err
	}

	target, err := resolveDBTarget(cfg.Tunnel, args, dbFlags, pickDatabase)
	if err != nil {
		return err
	}
	localPort := dbLocalPort
	if localPort == 0 {
		localPort = target.Port
	}
	opts, err := tunnelOptions(cfg.Tunnel, localPort)
	if err != nil {
		return err
	}

	client, restConfig, err := tunnel.NewClient(tunnelContext)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = tunnel.Open(ctx, client, tunnel.PortForwarder(restConfig, client, os.Stderr), target, opts)
	if apierrors.IsForbidden(err) {
		return fmt.Errorf("%w\nyour Kubernetes user is not allowed to run tunnels in namespace %q; "+
			"ask a cluster admin to apply the RBAC setup in docs/tunnel.md", err, opts.Namespace)
	}
	return err
}

// resolveDBTarget picks the database from a name argument, the ad hoc
// --host/--engine/--port flags, or, with neither, the interactive pick.
func resolveDBTarget(cfg config.TunnelConfig, args []string, flags dbTargetFlags,
	pick func([]config.TunnelDatabase) (string, error)) (tunnel.Target, error) {
	if flags.host != "" {
		if len(args) > 0 {
			return tunnel.Target{}, errors.New("give either a database name or --host, not both")
		}
		if flags.engine == "" {
			return tunnel.Target{}, errors.New("--host requires --engine (mysql or postgres)")
		}
		return dbTarget(config.TunnelDatabase{Engine: flags.engine, Host: flags.host, Port: flags.port})
	}
	if flags.engine != "" || flags.port != 0 {
		return tunnel.Target{}, errors.New("--engine and --port describe an ad hoc target and require --host")
	}

	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		if len(cfg.Databases) == 0 {
			return tunnel.Target{}, errors.New("no databases configured: add them under tunnel.databases in " +
				"~/.synacklab/config.yaml, or pass --host and --engine")
		}
		var err error
		if name, err = pick(cfg.Databases); err != nil {
			return tunnel.Target{}, err
		}
	}

	var names []string
	for _, db := range cfg.Databases {
		if db.Name == name {
			return dbTarget(db)
		}
		names = append(names, db.Name)
	}
	return tunnel.Target{}, fmt.Errorf("unknown database %q; configured databases: %s", name, strings.Join(names, ", "))
}

// dbTarget converts a database entry to a tunnel target, applying the engine's default port.
func dbTarget(db config.TunnelDatabase) (tunnel.Target, error) {
	defaultPort, err := tunnel.DefaultPort(db.Engine)
	if err != nil {
		return tunnel.Target{}, err
	}
	port := db.Port
	if port == 0 {
		port = defaultPort
	}
	return tunnel.Target{Kind: db.Engine, Host: db.Host, Port: port}, nil
}

func pickDatabase(dbs []config.TunnelDatabase) (string, error) {
	finder := fuzzy.New("Select database")
	for _, db := range dbs {
		target, _ := dbTarget(db) // entries passed ValidateTunnel
		finder.AddOption(db.Name, db.Engine+" "+net.JoinHostPort(target.Host, strconv.Itoa(target.Port)))
	}
	return finder.Select()
}
