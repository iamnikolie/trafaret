// Command trafaret renders a library of agent skills, subagents and rules —
// written once — into every harness (Claude Code, Codex, Cursor), for per-repo
// projects and user-level homes alike.
//
// It reads manifest.yaml from the config directory, merges each project's and
// home's vars, renders the Go templates (embedded examples, optionally overlaid
// by <config>/templates) and writes the concrete artifacts where each harness
// reads them. Run `trafaret skill` for the full reference.
package main

import "github.com/iamnikolie/trafaret/cmd"

func main() {
	cmd.Execute()
}
