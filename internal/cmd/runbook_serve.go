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

path may be a runbook file, a directory, or omitted entirely to use the
current directory. Either way, the browser shows a file-browser sidebar of
every *.md file under that directory (recursively) — pick one to open it.
If path is a file, or the directory unambiguously resolves to one (via
RUNBOOK.md/runbook.md, or being the only *.md file present), that one
opens automatically; otherwise pick from the sidebar.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runServe,
}

func init() {
	runbookServeCmd.Flags().IntVar(&servePort, "port", 4747, "port to listen on")
	runbookServeCmd.Flags().StringVar(&serveBind, "bind", "127.0.0.1", "address to bind to")
	runbookServeCmd.Flags().StringVar(&serveCwd, "cwd", "", "working directory for step execution (default: the open file's own directory)")
	runbookCmd.AddCommand(runbookServeCmd)
}

func runServe(_ *cobra.Command, args []string) error {
	pathArg := ""
	if len(args) > 0 {
		pathArg = args[0]
	}

	root, initialFile, err := resolveServeTarget(pathArg)
	if err != nil {
		return err
	}

	srv, label, err := buildRunbookServer(root, initialFile, serveCwd)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(serveBind, fmt.Sprintf("%d", servePort))
	if !isLoopbackBind(serveBind) {
		fmt.Printf("WARNING: --bind %s exposes this server beyond localhost.\n", serveBind)
		fmt.Println("Anyone who can reach this port can execute arbitrary commands as the local user — there is no authentication in v1.")
	}

	fmt.Printf("Serving %s at http://%s\n", label, addr)
	return http.ListenAndServe(addr, srv.Routes())
}

// resolveServeTarget resolves pathArg to a workspace root plus an optional
// initial file to open. Unlike resolveRunbookPath (used by run/fmt), an
// ambiguous or empty directory is not an error here — serve always enables
// the file-browser sidebar, so the user can pick a file interactively
// instead of being blocked at the command line.
func resolveServeTarget(pathArg string) (root, initialFile string, err error) {
	path := pathArg
	if path == "" {
		path = "."
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to access %s: %w", path, err)
	}
	if !info.IsDir() {
		return filepath.Dir(path), path, nil
	}

	if candidate, findErr := findRunbookInDir(path); findErr == nil {
		return path, candidate, nil
	}
	return path, "", nil
}

// buildRunbookServer wires a Server rooted at root, optionally pre-loaded
// with initialFile, always with the file-browser sidebar enabled. Split out
// from runServe so it's testable without binding a real listener.
func buildRunbookServer(root, initialFile, cwdFlag string) (*runbook.Server, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve %s: %w", root, err)
	}

	defaultCwd := cwdFlag
	if defaultCwd != "" {
		if defaultCwd, err = filepath.Abs(defaultCwd); err != nil {
			return nil, "", fmt.Errorf("failed to resolve working directory %q: %w", cwdFlag, err)
		}
	}

	var doc *runbook.Document
	var store runbook.SessionStore

	if initialFile != "" {
		source, err := os.ReadFile(initialFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read %s: %w", initialFile, err)
		}
		doc, err = (&runbook.GoldmarkParser{}).Parse(source, initialFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse %s: %w", initialFile, err)
		}

		sessCwd := defaultCwd
		if sessCwd == "" {
			sessCwd = doc.Dir
		}
		store = runbook.NewSessionStore(runbook.NewID(), initialFile, sessCwd)
	}

	engine := runbook.NewEngine(runbook.NewFileLogWriter(".synacklab"))
	srv := runbook.NewServer(doc, store, engine)
	srv.EnableWorkspace(absRoot, defaultCwd)

	label := absRoot
	if initialFile != "" {
		label = initialFile
	}
	return srv, label, nil
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
