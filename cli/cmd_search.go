package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/cs229-cli/cs229"
)

func (a *App) searchCmd() *cobra.Command {
	var semester string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search lectures by topic or session",
		Long: `Search the CS229 lecture schedule for lectures whose topic, session, or
details contain the given query (case-insensitive substring match).

Examples:
  cs229 search "neural networks"
  cs229 search regression --semester autumn2018
  cs229 search SVM --semester fall2022`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if semester == "" {
				semester = cs229.DefaultSemester
			}
			query := args[0]
			n := a.effectiveLimit(0)
			a.progressf("searching lectures for %q in %s...", query, semester)
			results, err := a.client.Search(cmd.Context(), query, semester, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(results, len(results))
		},
	}

	cmd.Flags().StringVar(&semester, "semester", "", "semester to search (default: "+cs229.DefaultSemester+")")
	return cmd
}
