// Package templates embeds the built-in template catalog into the binary.
package templates

import "embed"

// FS holds every built-in template. Files end in .tmpl so that Go sources
// and go.mod files inside templates are not treated as part of this module.
//
//go:embed all:http-api all:queue-worker all:object-store
var FS embed.FS
