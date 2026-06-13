package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/cs229-cli/cs229"
)

func (a *App) lecturesCmd() *cobra.Command {
	var semester string

	cmd := &cobra.Command{
		Use:   "lectures",
		Short: "List lectures from the course schedule",
		Long: `List lectures from the CS229 Machine Learning course schedule.

By default the fall25 semester is used. Pass --semester to select
another semester (e.g. autumn2018, fall2022, spring2023).

Supported semesters include:
  win26, fall25, summer25, winter25, fall24, summer24, winter24,
  fall23, summer23, spring23, fall2022, spring2022, fall2021,
  spring2021, fall2020, autumn2018`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if semester == "" {
				semester = cs229.DefaultSemester
			}
			n := a.effectiveLimit(0)
			a.progressf("fetching schedule for %s...", semester)
			lectures, err := a.client.Lectures(cmd.Context(), semester, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(lectures, len(lectures))
		},
	}

	cmd.Flags().StringVar(&semester, "semester", "", "semester to fetch (default: "+cs229.DefaultSemester+")")
	return cmd
}
