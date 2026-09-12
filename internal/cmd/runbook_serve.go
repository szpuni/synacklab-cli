package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"synacklab/pkg/runbook"
)

var (
	servePort int
	serveBind string
	serveCwd  string
)

var runbookServeCmd = &cobra.Command{
	Use:   "serve [path]",
	Short: "Start an interactive web server for a runbook",
	Long: `serve parses a Markdown file's fenced bash/python code blocks into an
interactive runbook and starts a local web server. Open it in a browser to
run each step one at a time, in order, carrying output from one step into
a later one via input=/capture= or opt-in {{...}} templating.

path may be a runbook file, a directory (RUNBOOK.md/runbook.md, or its
only *.md file, is used), or omitted entirely to use the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runServe,
}

func init() {
	runbookServeCmd.Flags().IntVar(&servePort, "port", 4747, "port to listen on")
	runbookServeCmd.Flags().StringVar(&serveBind, "bind", "127.0.0.1", "address to bind to")
	runbookServeCmd.Flags().StringVar(&serveCwd, "cwd", "", "working directory for step execution (default: the document's directory)")
	runbookCmd.AddCommand(runbookServeCmd)
}

func runServe(_ *cobra.Command, args []string) error {
	pathArg := ""
	if len(args) > 0 {
		pathArg = args[0]
	}
	docPath, err := resolveRunbookPath(pathArg)
	if err != nil {
		return err
	}

	srv, _, err := buildRunbookServer(docPath, serveCwd)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(serveBind, fmt.Sprintf("%d", servePort))
	if !isLoopbackBind(serveBind) {
		fmt.Printf("WARNING: --bind %s exposes this server beyond localhost.\n", serveBind)
		fmt.Println("Anyone who can reach this port can execute arbitrary commands as the local user — there is no authentication in v1.")
	}

	fmt.Printf("Serving %s at http://%s\n", docPath, addr)
	return http.ListenAndServe(addr, srv.Routes())
}

// buildRunbookServer reads and parses docPath and wires a Server around it.
// Split out from runServe so it's testable without binding a real listener.
func buildRunbookServer(docPath, cwdFlag string) (*runbook.Server, string, error) {
	source, err := os.ReadFile(docPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read %s: %w", docPath, err)
	}

	doc, err := (&runbook.GoldmarkParser{}).Parse(source, docPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse %s: %w", docPath, err)
	}

	defaultCwd := cwdFlag
	if defaultCwd == "" {
		defaultCwd = doc.Dir
	}
	absCwd, err := filepath.Abs(defaultCwd)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve working directory %q: %w", defaultCwd, err)
	}

	store := runbook.NewSessionStore(runbook.NewID(), docPath, absCwd)
	engine := runbook.NewEngine(runbook.NewFileLogWriter(".synacklab"))
	return runbook.NewServer(doc, store, engine), docPath, nil
}

// isLoopbackBind reports whether bind resolves to localhost only
// (Requirement 13.1/13.2).
func isLoopbackBind(bind string) bool {
	if bind == "localhost" {
		return true
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}
