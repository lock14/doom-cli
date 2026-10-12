// Package doomcli provides the root embedded filesystem for curated catalog
// entities and source-port configuration templates.
package doomcli

import "embed"

// FS holds the embedded curated catalog entities (`catalog/*.json`) and the
// default source-port configuration templates (`dsda-doom/dsda-doom.cfg` and
// `uzdoom/autoexec.cfg`).
//
//go:embed catalog/*.json dsda-doom/dsda-doom.cfg uzdoom/autoexec.cfg
var FS embed.FS
