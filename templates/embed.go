// Package templates embeds the built-in template catalog into the binary.
package templates

import "embed"

// FS holds every built-in template plus the _shared layers they reuse.
// Files end in .tmpl so that Go sources and go.mod files inside templates
// are not treated as part of this module. A new template directory is picked
// up automatically; there is nothing to register.
//
//go:embed all:*
var FS embed.FS
