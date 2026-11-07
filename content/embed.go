// Package content holds everything players read or see: words, drinks, pixel art,
// lighting and weather. It's all plain files in this folder, built into the
// game when it's compiled. Edit the files, rebuild, and the game changes.
package content

import "embed"

// FS is every content file, built into the game.
//
//go:embed *.json interiors sprites text dialogue
var FS embed.FS
