// Package skills embeds skill markdown documents into the binary via go:embed.
package skills

import "embed"

//go:embed *.md */*.md
var FS embed.FS
