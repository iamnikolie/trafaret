// Command trafaret renders a shared agent/skill/rule library into projects.
//
// It reads manifest.yaml from the config directory, resolves one Project per
// entry, renders the Go templates (embedded examples, optionally overlaid by
// <config>/templates) with that project's tokens, and writes the concrete
// artifacts into <repoRoot>/.claude/, .cursor/, and .agents/.codex when targeted.
// Run `trafaret skill` for the full reference.
package main

import "github.com/iamnikolie/trafaret/cmd"

func main() {
	cmd.Execute()
}
