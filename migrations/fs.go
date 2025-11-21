package migrations

import "embed"

// 1.- Embed every SQL migration bundle so they are available at runtime.
//
//go:embed 0001_core/*.sql 0002_join_requests/*.sql 0003_tasks/*.sql 0004_verification/*.sql
var Core embed.FS
