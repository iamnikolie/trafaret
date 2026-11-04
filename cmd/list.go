package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/iamnikolie/trafaret/internal/engine"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the projects and homes in the manifest",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := eng.LoadManifest()
		if err != nil {
			return err
		}

		if jsonFlag {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")

			_ = enc.Encode(struct {
				Projects map[string]engine.Project `json:"projects"`
				Homes    map[string]engine.Home    `json:"homes"`
			}{m.Projects, m.Homes})

			return nil
		}

		dash := func(s string) string {
			if s == "" {
				return "-"
			}

			return s
		}

		fmt.Fprintf(stdout, "%-16s %-20s %-16s %s\n", "PROJECT", "HARNESSES", "BUNDLES", "SKILLS")

		for _, n := range sortedNames(m.Projects) {
			p := m.Projects[n]
			fmt.Fprintf(stdout, "%-16s %-20s %-16s %s\n", n, strings.Join(p.Harnesses, "+"),
				dash(strings.ReplaceAll(p.BundlesDisplay(), ", ", ",")), dash(strings.Join(eng.ProjectSkills(p), ",")))
		}

		if len(m.Homes) == 0 {
			return nil
		}

		fmt.Fprintf(stdout, "\n%-16s %-10s %-30s %s\n", "HOME", "HARNESS", "DIR", "SKILLS")

		for _, n := range sortedNames(m.Homes) {
			h := m.Homes[n]
			fmt.Fprintf(stdout, "%-16s %-10s %-30s %s\n", n, h.Harness, h.Dir, eng.HomeSkills(h))
		}

		return nil
	},
}

func sortedNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}

	sort.Strings(out)

	return out
}

func init() {
	rootCmd.AddCommand(listCmd)
}
