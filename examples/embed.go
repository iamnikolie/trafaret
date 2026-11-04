// Package examples carries the example template set that is compiled into the
// trafaret binary: the bundle registry (bundles.yaml) and the skill, agent and
// Cursor-rule templates it names. They show the template format and every token
// family; a real library lives in the user's config directory, in a templates/
// overlay that shadows or extends these files by path.
//
// Paths inside FS are "bundles.yaml", "skills/<name>/...", "agents/<name>.md.tmpl"
// and "cursor/rules/<name>.mdc.tmpl".
package examples

import "embed"

// FS is the embedded example template set.
//
//go:embed bundles.yaml skills agents cursor
var FS embed.FS
