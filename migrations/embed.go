package migrations

import "embed"

// Files is the authoritative, ordered database migration sequence.
//
//go:embed *.sql
var Files embed.FS
