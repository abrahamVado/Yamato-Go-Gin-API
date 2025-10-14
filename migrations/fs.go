package migrations

import "embed"

// 1.- Embed every SQL migration in the first core bundle so they are available at runtime.
//
//go:embed 0001_core/*.sql
var Core embed.FS
