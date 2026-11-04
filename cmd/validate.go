package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check the manifest and render every template",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := eng.LoadManifest()
		if err != nil {
			return err
		}

		rep := eng.Validate(m)
		for _, p := range rep.Problems {
			fmt.Fprintf(stdout, "  %-16s %s\n", p.Where, p.Msg)
		}

		if len(rep.Problems) == 0 {
			fmt.Fprintf(stdout, "ok: %d project(s), %d home(s) valid\n", rep.Projects, rep.Homes)

			return nil
		}

		fmt.Fprintf(stdout, "%d problem(s)\n", len(rep.Problems))

		return exitError{1}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
