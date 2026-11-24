// Package doomcli provides the root embedded filesystem for curated catalog
// entities, source-port configuration templates, and the portable agent skill.
package doomcli

import "embed"

// FS holds the embedded curated catalog entities (`catalog/*.json`), default
// source-port configuration templates (`configs/dsda-doom/dsda-doom.cfg` and
// `configs/uzdoom/autoexec.cfg`), and the portable CLI agent skill
// (`skills/doom-cli/SKILL.md`).
//
//go:embed catalog/*.json configs/dsda-doom/dsda-doom.cfg configs/uzdoom/autoexec.cfg skills/doom-cli/SKILL.md
var FS embed.FS
