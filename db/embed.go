// Package db embeds the goose SQL migrations so cmd/migrate ships as a single binary.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
