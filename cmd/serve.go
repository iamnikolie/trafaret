package cmd

import (
	"github.com/iamnikolie/trafaret/internal/site"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the local live manual (default 127.0.0.1:4477)",
	Long: `Serves a bilingual operator manual rendered from the config directory on every
refresh: live per-project sync status, the template catalog and bundle table.
Binds the --addr you give (default 127.0.0.1:4477); nothing is published.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return site.Serve(eng, addrFlag, stdout)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
