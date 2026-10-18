package cmd

import (
	"cmp"
	"os"
	"os/user"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	"synacklab/pkg/tunnel"
)

var (
	tunnelContext   string
	tunnelNamespace string
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Open a local tunnel to a private service through Kubernetes",
	Long: `tunnel reaches services that only accept traffic from inside your cluster's
network (for example a database whose firewall only allows the app subnet).
It starts a short-lived socat pod in the cluster, forwards a local port to it
through the Kubernetes API, and deletes the pod when you press Ctrl-C.

It works with any Kubernetes cluster your kubeconfig can reach. Cluster
setup (namespace, RBAC, networking) is described in docs/tunnel.md.

Available commands:
  db - Tunnel to a MySQL or PostgreSQL database`,
}

func init() {
	tunnelCmd.PersistentFlags().StringVar(&tunnelContext, "context", "", "kube context to use (default: current context)")
	tunnelCmd.PersistentFlags().StringVarP(&tunnelNamespace, "namespace", "n", "",
		"namespace for the tunnel pod (default: tunnel.namespace, else "+tunnel.DefaultNamespace+")")
}

// tunnelOptions builds pod options shared by every tunnel subcommand.
// Namespace precedence: --namespace, then tunnel.namespace, then the default.
func tunnelOptions(cfg config.TunnelConfig, localPort int) (tunnel.Options, error) {
	maxDuration, err := cfg.ParseMaxDuration()
	if err != nil {
		return tunnel.Options{}, err
	}
	opts := tunnel.Options{
		Namespace:    cmp.Or(tunnelNamespace, cfg.Namespace, tunnel.DefaultNamespace),
		Image:        cmp.Or(cfg.Image, tunnel.DefaultImage),
		MaxDuration:  maxDuration,
		NodeSelector: cfg.NodeSelector,
		LocalPort:    localPort,
		Out:          os.Stdout,
		Err:          os.Stderr,
	}
	if u, err := user.Current(); err == nil {
		opts.User = u.Username
	}
	return opts, nil
}
