// Package shitmail exports the embedded frontend filesystem so cmd/shitmail
// can pass it to the HTTP router.  The //go:embed directive bundles the
// entire web/dist directory into the binary at compile time.
package shitmail

import "embed"

// WebFS is the embedded React build produced by `make frontend`.
// The Go compiler embeds web/dist at build time — no runtime file access needed.
//
//go:embed web/dist
var WebFS embed.FS
