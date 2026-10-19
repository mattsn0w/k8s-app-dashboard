// Package deploydashboard embeds the frontend so the single binary is fully
// self-contained. The import path is "deploy-dashboard"; the package
// identifier drops the dash because Go identifiers cannot contain it.
package deploydashboard

import "embed"

// WebFS holds the frontend assets (web/index.html) compiled into the binary.
// This is the single source of truth for the UI -- edit web/index.html and
// rebuild; no codegen or env var is required (see APP-01).
//
//go:embed all:web
var WebFS embed.FS
