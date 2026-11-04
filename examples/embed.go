// Package examples carries the example template set that is compiled into the
// trafaret binary: the bundle registry (bundles.yaml) and the skills, agents,
// rules and partials it names. They are one small showcase of the template
// format; a real library lives in the user's config directory, in a templates/
// overlay that shadows or extends these files by path. examples/demo is the
// runnable config that renders them (with its committed output); it is not
// embedded.
//
// Paths inside FS are "bundles.yaml", "skills/<name>/...", "agents/<name>.md[.tmpl]",
// "rules/<name>.md[.tmpl]" and "partials/<name>.tmpl".
package examples

import "embed"

// FS is the embedded example template set.
//
//go:embed bundles.yaml skills agents rules partials
var FS embed.FS
