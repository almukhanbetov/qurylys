// Package migrations embeds the Goose SQL migrations so the migrate binary
// works inside the container without the source tree.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
