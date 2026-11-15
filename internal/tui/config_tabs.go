package tui

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (m model) renderConfigTabCLI() []string {
	s := m.cfgMgr
	nerdText := "[ ] Disabled"
	if m.nerdFonts {
		nerdText = "[X] Enabled"
	}

	type cliRow struct {
		label  string
		value  string
		source string
		isText bool
	}
	rows := []cliRow{
		{
			label:  "Theme:",
			value:  fmt.Sprintf("< %s > (%s)", m.theme.Name, m.theme.Description),
			source: s.themeSource,
		},
		{
			label:  "Nerd Fonts:",
			value:  nerdText,
			source: s.nerdFontsSource,
		},
		{
			label:  "WADs Dir:",
			value:  s.wadsDirInput.View(),
			source: s.wadsDirSource,
			isText: true,
		},
		{
			label:  "Extra WADs Dirs:",
			value:  s.extraDirsInput.View(),
			source: s.extraDirsSource,
			isText: true,
		},
		{
			label:  "Bin Dir:",
			value:  s.binDirInput.View(),
			source: s.binDirSource,
			isText: true,
		},
		{
			label:  "SoundFonts Dir:",
			value:  s.sfDirInput.View(),
			source: s.sfDirSource,
			isText: true,
		},
	}

	var out []string
	for idx, r := range rows {
		paddedLabel := padOrTruncateName(r.label, 18)
		prefix := "  "
		labelStr := m.styles.Label.Render(paddedLabel)
		valStr := r.value
		if idx == s.focusIdx {
			prefix = m.styles.CursorBar.Render("▎ ")
			labelStr = m.styles.CursorText.Render(paddedLabel)
			if !r.isText {
				valStr = m.styles.ValueBold.Render(r.value)
			}
		}
		srcBadge := m.styles.Help.Render(fmt.Sprintf("[%s]", r.source))
		out = append(out, fmt.Sprintf("%s%s %s %s", prefix, labelStr, valStr, srcBadge))
	}
	if m.paths != nil && m.paths.ConfigFile != "" {
		out = append(out, "", m.styles.Help.Render("  Config File: "+m.paths.ConfigFile))
	}
	return out
}

func (m model) renderConfigTabEngines() []string {
	s := m.cfgMgr
	engID := "dsda-doom"
	engDisp := "DSDA-Doom"
	if len(s.engines) > 0 && s.engineIdx >= 0 && s.engineIdx < len(s.engines) {
		engID = s.engines[s.engineIdx].ID
		engDisp = s.engines[s.engineIdx].EffectiveDisplayName()
	}

	type engRow struct {
		label  string
		value  string
		isText bool
	}
	rows := []engRow{
		{
			label: "Engine:",
			value: fmt.Sprintf("< %s (%s) >", engDisp, engID),
		},
		{
			label:  "Binary:",
			value:  s.engBinaryInput.View(),
			isText: true,
		},
		{
			label: "Args Style:",
			value: fmt.Sprintf("< %s >", s.engArgsStyle),
		},
		{
			label:  "Default Args:",
			value:  s.engDefaultInput.View(),
			isText: true,
		},
		{
			label:  "Prefix Args:",
			value:  s.engPrefixInput.View(),
			isText: true,
		},
		{
			label:  "Badge:",
			value:  s.engBadgeInput.View(),
			isText: true,
		},
		{
			label:  "Display Name:",
			value:  s.engDisplayInput.View(),
			isText: true,
		},
	}

	var out []string
	for idx, r := range rows {
		paddedLabel := padOrTruncateName(r.label, 18)
		prefix := "  "
		labelStr := m.styles.Label.Render(paddedLabel)
		valStr := r.value
		if idx == s.focusIdx {
			prefix = m.styles.CursorBar.Render("▎ ")
			labelStr = m.styles.CursorText.Render(paddedLabel)
			if !r.isText {
				valStr = m.styles.ValueBold.Render(r.value)
			}
		}
		out = append(out, fmt.Sprintf("%s%s %s", prefix, labelStr, valStr))
	}
	return out
}

func (m model) stylePortStatus(status string) string {
	switch status {
	case "[✓ In Sync]":
		return m.styles.StatusReady.Render(status)
	case "[● Modified]":
		return m.styles.TagDSDA.Render(status)
	default:
		return m.styles.StatusMissing.Render(status)
	}
}

func (m model) renderConfigTabPorts() []string {
	s := m.cfgMgr
	autoPath := "autoexec.cfg"
	dsdaPath := "dsda-doom.cfg"
	if m.paths != nil {
		autoPath = filepath.Join(m.paths.UZDoomDir, "autoexec.cfg")
		dsdaPath = filepath.Join(m.paths.DSDADir, "dsda-doom.cfg")
	}

	out := []string{
		fmt.Sprintf(
			"  %s %s @ %dHz",
			m.styles.Label.Render(padOrTruncateName("Detected Display:", 18)),
			s.detectedRes,
			s.detectedRate,
		),
		fmt.Sprintf(
			"  %s %s %s",
			m.styles.Label.Render(padOrTruncateName("UZDoom Config:", 18)),
			m.stylePortStatus(s.uzdoomStatus),
			m.styles.Help.Render(autoPath),
		),
		fmt.Sprintf(
			"  %s %s %s",
			m.styles.Label.Render(padOrTruncateName("DSDA-Doom Config:", 18)),
			m.stylePortStatus(s.dsdaStatus),
			m.styles.Help.Render(dsdaPath),
		),
		"",
	}

	actions := []string{
		"< View Diff >",
		"< Deploy Curated Configs >",
	}
	for idx, act := range actions {
		prefix := "  "
		text := m.styles.Label.Render(act)
		if idx == s.focusIdx {
			prefix = m.styles.CursorBar.Render("▎ ")
			text = m.styles.CursorText.Render(act)
		}
		out = append(out, prefix+text)
	}
	return out
}

func (m model) renderConfigManagerFooter() string {
	s := m.cfgMgr
	items := []keyHelp{
		{"1/2/3/[/]", "Tab"},
		{"↑/↓/Tab", "Field"},
	}
	if s != nil && s.activeTab == configTabPorts {
		items = []keyHelp{
			{"1/2/3/[/]", "Tab"},
			{"↑/↓/←/→", "Action"},
			{"Enter/Space", "Run"},
			{"F1", "Help"},
			{"Esc", "Close"},
		}
	} else {
		items = append(items,
			keyHelp{"←/→/Space", "Adjust"},
			keyHelp{"Ctrl+S", "Save"},
		)
		if s != nil && s.activeTab == configTabEngines {
			items = append(items, keyHelp{"Ctrl+R", "Reset"})
		}
		items = append(items,
			keyHelp{"F1", "Help"},
			keyHelp{"Esc", "Cancel"},
		)
	}
	footer := m.styles.FormatKeyHelp(items)
	if m.width > 0 {
		return "\n\n" + truncateANSI(strings.TrimPrefix(footer, "\n\n"), m.width)
	}
	return footer
}
