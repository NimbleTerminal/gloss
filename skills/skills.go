// Package skills carries what gloss installs for the programs around it, so
// that the binary can print and write it: the skill that teaches agents to use
// gloss (gloss skill), and the Lua that joins it to the Rex terminal
// multiplexer (gloss skill install rex).
package skills

import _ "embed"

// Gloss is skills/gloss/SKILL.md, whole, as an agent would install it.
//
//go:embed gloss/SKILL.md
var Gloss string

// RexActions is skills/rex/gloss.lua: the actions and key Rex's init.lua loads
// with require("gloss").
//
//go:embed rex/gloss.lua
var RexActions string

// RexCompanion is skills/rex/gloss-companion.lua: the script that keeps a gloss
// split in step with the block beside it, which the gloss_companion action starts.
//
//go:embed rex/gloss-companion.lua
var RexCompanion string
