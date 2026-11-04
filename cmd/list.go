package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the projects (and homes) in the manifest",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := eng.LoadManifest()
		if err != nil {
			return err
		}

		if jsonFlag {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")

			_ = enc.Encode(m.Projects)

			return nil
		}

		var names []string
		for n := range m.Projects {
			names = append(names, n)
		}

		sort.Strings(names)
		fmt.Fprintf(stdout, "%-16s %-10s %-8s %s\n", "PROJECT", "TARGETS", "TICKETS", "BUNDLES")

		for _, n := range names {
			p := m.Projects[n]
			fmt.Fprintf(stdout, "%-16s %-10s %-8s %s\n", n, strings.Join(p.Targets, "+"), p.Tickets.Config, strings.ReplaceAll(p.BundlesDisplay(), ", ", ","))
		}

		if len(m.Homes) == 0 {
			return nil
		}

		var homes []string
		for n := range m.Homes {
			homes = append(homes, n)
		}

		sort.Strings(homes)
		fmt.Fprintf(stdout, "\n%-16s %-30s %s\n", "HOME", "SKILLS DIR", "SKILLS")

		for _, n := range homes {
			h := m.Homes[n]

			skills := "all"
			if len(h.Skills) > 0 {
				skills = strings.Join(h.Skills, ",")
			}

			if len(h.Skip) > 0 {
				skills += " (skip " + strings.Join(h.Skip, ",") + ")"
			}

			fmt.Fprintf(stdout, "%-16s %-38s %s\n", n, h.SkillsDir, skills)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
