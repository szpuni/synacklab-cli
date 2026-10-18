package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	"synacklab/pkg/tunnel"
	"synacklab/pkg/webui"
)

var (
	tunnelServePort int
	tunnelServeBind string
)

var tunnelServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Open tunnels from a local web page",
	Long: `serve starts a local web page that opens the same tunnels as "tunnel db":
pick a configured database or enter a host, choose the local port, kube
context and namespace, and see the tunnel's status and logs. Closing the
tunnel on the page, or stopping serve with Ctrl-C, deletes its pod.

The page is reachable only from this machine. --context and --namespace
preselect the form.`,
	Args: cobra.NoArgs,
	RunE: runTunnelServe,
	// Errors here are runtime failures (cluster, network), not flag misuse.
	SilenceUsage: true,
}

func init() {
	tunnelServeCmd.Flags().IntVar(&tunnelServePort, "port", 4748, "port to listen on")
	tunnelServeCmd.Flags().StringVar(&tunnelServeBind, "bind", "127.0.0.1", "loopback address to bind to (127.0.0.1, ::1 or localhost)")
	tunnelCmd.AddCommand(tunnelServeCmd)
}

func runTunnelServe(_ *cobra.Command, _ []string) error {
	if err := webui.ValidateBind(tunnelServeBind); err != nil {
		return err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if _, err := cfg.Tunnel.ParseMaxDuration(); err != nil {
		return err
	}
	logger, err := buildLogger(cfg)
	if err != nil {
		return err
	}
	contexts, current, err := tunnel.Contexts()
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(tunnelServeBind, strconv.Itoa(tunnelServePort)))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// SIGINT/SIGTERM stops the page and closes the open tunnel; Wait holds
	// the exit until its pod is deleted.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	srv := tunnel.NewServer(ctx, tunnelServerConfig(cfg.Tunnel, contexts, current))
	defer srv.Wait()
	defer stop()

	fmt.Printf("Serving tunnels at http://%s\n", ln.Addr())
	return webui.Serve(ctx, ln, srv.Routes(), logger)
}

// tunnelServerConfig gives the page the same choices and rules as `tunnel db`.
func tunnelServerConfig(cfg config.TunnelConfig, contexts []string, currentContext string) tunnel.ServerConfig {
	srv := tunnel.ServerConfig{
		Contexts: contexts,
		Defaults: tunnel.Request{
			Context:   cmp.Or(tunnelContext, currentContext),
			Namespace: cmp.Or(tunnelNamespace, cfg.Namespace, tunnel.DefaultNamespace),
		},
	}
	if err := cfg.ValidateDatabases(); err != nil {
		srv.DatabaseErr = err.Error()
	} else {
		for _, db := range cfg.Databases {
			target, _ := db.Target() // entries passed ValidateDatabases
			srv.Databases = append(srv.Databases, tunnel.Database{Name: db.Name, Engine: db.Engine, Host: target.Host, Port: target.Port})
		}
	}

	srv.Resolve = func(req tunnel.Request) (tunnel.Target, error) {
		var args []string
		switch {
		case req.Name != "":
			args = []string{req.Name}
		case req.Host == "":
			return tunnel.Target{}, errors.New("choose a configured database or enter a host")
		}
		return resolveDBTarget(cfg, args, dbTargetFlags{host: req.Host, engine: req.Engine, port: req.Port},
			func([]config.TunnelDatabase) (string, error) { return "", errors.New("no database chosen") })
	}

	srv.Start = func(ctx context.Context, req tunnel.Request, target tunnel.Target, out io.Writer, ready func(string)) error {
		opts, err := tunnelOptions(cfg, req.Namespace, req.LocalPort)
		if err != nil {
			return err
		}
		opts.Out, opts.Err, opts.Ready = out, out, ready
		client, restConfig, err := tunnel.NewClient(req.Context)
		if err != nil {
			return err
		}
		return tunnel.Open(ctx, client, tunnel.PortForwarder(restConfig, client, out), target, opts)
	}
	return srv
}
