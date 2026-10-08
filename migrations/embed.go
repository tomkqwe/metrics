// Package migrations embeds SQL files used to initialize and upgrade the database schema.
package migrations

import "embed"

// FS contains SQL migration files embedded into the application binary.
//
//go:embed *.sql
var FS embed.FS
