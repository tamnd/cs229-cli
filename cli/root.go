// Package cli builds the cs229 command tree on top of the cs229 library.
package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/tamnd/cs229-cli/cs229"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// exit codes
const (
	exitError  = 1
	exitUsage  = 2
	exitNoData = 3
)

// ExitError carries a process exit code up to main.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func (e *ExitError) Unwrap() error { return e.Err }

func codeError(code int, err error) error { return &ExitError{Code: code, Err: err} }

// App holds shared state threaded through every command.
type App struct {
	client *cs229.Client
	cfg    cs229.Config

	output   string
	noHeader bool
	template string
	limit    int
	quiet    bool
}

// Root builds the root command and its subtree.
func Root() *cobra.Command {
	app := &App{cfg: cs229.DefaultConfig()}

	root := &cobra.Command{
		Use:   "cs229",
		Short: "Browse Stanford CS229 Machine Learning course content",
		Long: `cs229 fetches lecture schedules and materials from the Stanford CS229
Machine Learning course at https://cs229.stanford.edu.

It returns records as table, JSON, JSONL, CSV, or TSV. All data is
pulled directly from the public course website; no API key is required.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return app.setup()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&app.output, "output", "o", "auto", "output format: table|json|jsonl|csv|tsv (auto=table on TTY, jsonl piped)")
	pf.BoolVar(&app.noHeader, "no-header", false, "omit the header row in table/csv/tsv")
	pf.StringVar(&app.template, "template", "", "Go text/template applied per record")
	pf.IntVarP(&app.limit, "limit", "n", 0, "maximum number of records (0 = all)")
	pf.BoolVarP(&app.quiet, "quiet", "q", false, "suppress progress messages on stderr")

	pf.StringVar(&app.cfg.BaseURL, "base-url", app.cfg.BaseURL, "override the course base URL")
	pf.DurationVar(&app.cfg.Rate, "delay", app.cfg.Rate, "minimum spacing between requests")
	pf.DurationVar(&app.cfg.Timeout, "timeout", app.cfg.Timeout, "per-request timeout")
	pf.IntVar(&app.cfg.Retries, "retries", app.cfg.Retries, "retry attempts on 429/5xx")
	pf.StringVar(&app.cfg.UserAgent, "user-agent", app.cfg.UserAgent, "User-Agent sent with each request")

	root.AddCommand(
		app.lecturesCmd(),
		app.searchCmd(),
		newVersionCmd(),
	)
	return root
}

func (a *App) setup() error {
	if a.output == "" || a.output == "auto" {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			a.output = "table"
		} else {
			a.output = "jsonl"
		}
	}
	if !validFormat(a.output) {
		return codeError(exitUsage, fmt.Errorf("unknown output format %q", a.output))
	}
	a.client = cs229.NewClient(a.cfg)
	return nil
}

func (a *App) render(records any) error {
	r := newRenderer(os.Stdout, a.output, a.noHeader, a.template)
	return r.render(records)
}

func (a *App) renderOrEmpty(records any, n int) error {
	if err := a.render(records); err != nil {
		return err
	}
	if n == 0 {
		return codeError(exitNoData, nil)
	}
	return nil
}

func (a *App) progressf(format string, args ...any) {
	if a.quiet {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (a *App) effectiveLimit(def int) int {
	if a.limit > 0 {
		return a.limit
	}
	return def
}
