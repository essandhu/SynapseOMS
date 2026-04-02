// Package migrations embeds the SQL migration files so they can be
// referenced from any package without path-relative embed constraints.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
