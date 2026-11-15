package tui

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/portcfg"
)

func (m *model) adjustConfigField(delta int) {
	s := m.cfgMgr
	if s == nil {
		return
	}
	switch s.activeTab {
	case configTabCLI:
		switch s.focusIdx {
		case cliFieldTheme:
			if len(s.themes) > 0 {
				s.themeIdx = (s.themeIdx + delta + len(s.themes)) % len(s.themes)
				m.theme = s.themes[s.themeIdx]
				m.styles = CompileStyles(m.theme)
				m.input.PlaceholderStyle = m.styles.Placeholder
				s.applyStyles(m.styles)
			}
		case cliFieldNerdFonts:
			m.nerdFonts = !m.nerdFonts
		}
	case configTabEngines:
		switch s.focusIdx {
		case engFieldSelect:
			if len(s.engines) > 0 {
				if s.isTab2Dirty() && !s.confirmDiscardEng {
					s.confirmDiscardEng = true
					s.statusMsg = "Unsaved engine edits: press ←/→ again to discard, or Ctrl+S to save"
					return
				}
				s.statusMsg = ""
				nextIdx := (s.engineIdx + delta + len(s.engines)) % len(s.engines)
				s.loadEngineFields(nextIdx, m.styles)
			}
		case engFieldArgsStyle:
			s.confirmDiscardEng = false
			if s.engArgsStyle == "boom" {
				s.engArgsStyle = "zdoom"
			} else {
				s.engArgsStyle = "boom"
			}
		}
	case configTabPorts:
		s.focusIdx = (s.focusIdx + delta + portFieldCount) % portFieldCount
	}
}

func (m *model) activatePortConfigAction() {
	s := m.cfgMgr
	if s == nil {
		return
	}
	m.ensurePathsAndConfig()
	switch s.focusIdx {
	case portFieldViewDiff:
		var buf bytes.Buffer
		_ = portcfg.DiffConfigsWithDisplay(m.paths, s.detectedRes, s.detectedRate, &buf)
		_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
		s.diffViewport = viewport.New(vpWidth, vpHeight)
		s.diffViewport.SetContent(buf.String())
		s.viewingDiff = true
	case portFieldDeploy:
		var buf bytes.Buffer
		if err := portcfg.DeployConfigsWithDisplay(
			m.paths, s.detectedRes, s.detectedRate, true, &buf,
		); err != nil {
			s.statusMsg = fmt.Sprintf("✗ Deploy failed: %v", err)
			return
		}
		s.refreshPortConfigStatus(m.paths)
		s.statusMsg = "✓ Deployed curated autoexec.cfg and dsda-doom.cfg"
		m.statusMsg = s.statusMsg
	}
}

func (m *model) saveActiveConfigTab() tea.Cmd {
	s := m.cfgMgr
	if s == nil {
		return nil
	}
	m.ensurePathsAndConfig()

	switch s.activeTab {
	case configTabCLI:
		themeChanged := !strings.EqualFold(m.theme.Name, s.initialThemeName)
		if themeChanged || s.themeSource == "config" {
			m.appConfig.Theme = m.theme.Name
			if themeChanged && s.themeSource == "default" {
				s.themeSource = "config"
			}
		}
		nerdChanged := m.nerdFonts != s.initialNerdFonts
		if nerdChanged || s.nerdFontsSource == "config" {
			m.appConfig.SetNerdFonts(m.nerdFonts)
			if nerdChanged && s.nerdFontsSource == "default" {
				s.nerdFontsSource = "config"
			}
		}

		defPaths := config.GetPaths()
		wadsVal := strings.TrimSpace(s.wadsDirInput.Value())
		extraVal := strings.TrimSpace(s.extraDirsInput.Value())
		binVal := strings.TrimSpace(s.binDirInput.Value())
		sfVal := strings.TrimSpace(s.sfDirInput.Value())

		if wadsVal != strings.TrimSpace(s.initialWadsDir) {
			m.appConfig.WadsDir = wadsVal
			if wadsVal != "" {
				m.paths.WadsDir = wadsVal
				if s.wadsDirSource == "default" {
					s.wadsDirSource = "config"
				}
			} else {
				m.paths.WadsDir = defPaths.WadsDir
				s.wadsDirInput.SetValue(defPaths.WadsDir)
				wadsVal = defPaths.WadsDir
				if s.wadsDirSource == "config" {
					s.wadsDirSource = "default"
				}
			}
		}
		if extraVal != strings.TrimSpace(s.initialExtraDirs) {
			parsed := config.ParsePathList(extraVal)
			m.appConfig.WadsDirs = parsed
			m.paths.WadsDirs = parsed
			if len(parsed) > 0 && s.extraDirsSource == "default" {
				s.extraDirsSource = "config"
			} else if len(parsed) == 0 && s.extraDirsSource == "config" {
				s.extraDirsSource = "default"
			}
		}
		if binVal != strings.TrimSpace(s.initialBinDir) {
			m.appConfig.BinDir = binVal
			if binVal != "" {
				m.paths.SetBinDir(binVal)
				if s.binDirSource == "default" {
					s.binDirSource = "config"
				}
			} else {
				m.paths.SetBinDir(defPaths.BinDir)
				s.binDirInput.SetValue(defPaths.BinDir)
				binVal = defPaths.BinDir
				if s.binDirSource == "config" {
					s.binDirSource = "default"
				}
			}
		}
		if sfVal != strings.TrimSpace(s.initialSFDir) {
			m.appConfig.SoundFontsDir = sfVal
			if sfVal != "" {
				m.paths.SetSoundFontsDir(sfVal)
				if s.sfDirSource == "default" {
					s.sfDirSource = "config"
				}
			} else {
				m.paths.SetSoundFontsDir(defPaths.SoundFontsDir)
				s.sfDirInput.SetValue(defPaths.SoundFontsDir)
				sfVal = defPaths.SoundFontsDir
				if s.sfDirSource == "config" {
					s.sfDirSource = "default"
				}
			}
		}

		if err := config.SaveConfig(m.paths, m.appConfig); err != nil {
			s.statusMsg = fmt.Sprintf("✗ Save failed: %v", err)
			return nil
		}

		s.savedTheme = m.theme
		s.savedNerdFonts = m.nerdFonts
		s.initialThemeName = m.theme.Name
		s.initialNerdFonts = m.nerdFonts
		s.initialWadsDir = wadsVal
		s.initialExtraDirs = extraVal
		s.initialBinDir = binVal
		s.initialSFDir = sfVal

		m.themeSource = s.themeSource
		m.nerdFontsSource = s.nerdFontsSource
		m.wadsDirSource = s.wadsDirSource
		m.wadsDirsSource = s.extraDirsSource
		m.binDirSource = s.binDirSource
		m.soundFontsSource = s.sfDirSource

		m.wadRoots = m.paths.WadRoots()
		m.rebuildIndexAndTargets()
		s.refreshPortConfigStatus(m.paths)
		s.statusMsg = fmt.Sprintf("✓ Saved CLI settings to %s", filepath.Base(m.paths.ConfigFile))
		m.statusMsg = s.statusMsg
		return nil

	case configTabEngines:
		if len(s.engines) == 0 {
			return nil
		}
		eng := s.engines[s.engineIdx]
		bin := strings.TrimSpace(s.engBinaryInput.Value())
		style := strings.TrimSpace(s.engArgsStyle)
		defArgs := strings.Fields(s.engDefaultInput.Value())
		prefArgs := strings.Fields(s.engPrefixInput.Value())
		badge := strings.TrimSpace(s.engBadgeInput.Value())
		dispName := strings.TrimSpace(s.engDisplayInput.Value())
		emptyAppend := []string{}

		var opts catalog.EngineOverrideOptions
		if bin != strings.TrimSpace(s.initialEngBinary) {
			opts.Binary = &bin
		}
		if style != strings.TrimSpace(s.initialEngStyle) {
			opts.ArgsStyle = &style
		}
		if strings.Join(defArgs, " ") != strings.Join(strings.Fields(s.initialEngDefault), " ") {
			opts.DefaultArgs = &defArgs
			opts.AppendArgs = &emptyAppend
		}
		if strings.Join(prefArgs, " ") != strings.Join(strings.Fields(s.initialEngPrefix), " ") {
			opts.PrefixArgs = &prefArgs
		}
		if badge != strings.TrimSpace(s.initialEngBadge) {
			opts.Badge = &badge
		}
		if dispName != strings.TrimSpace(s.initialEngDisplay) {
			opts.DisplayName = &dispName
		}

		catDir := m.catalogDir()
		if opts.Binary != nil ||
			opts.ArgsStyle != nil ||
			opts.DefaultArgs != nil ||
			opts.PrefixArgs != nil ||
			opts.Badge != nil ||
			opts.DisplayName != nil ||
			eng.Overridden {
			if _, err := catalog.ConfigEngine(catDir, m.catalog, eng.ID, opts); err != nil {
				s.statusMsg = fmt.Sprintf("✗ Save failed: %v", err)
				return nil
			}
		}
		m.reloadCatalogAndIndex()
		s.engines = m.availableEngines()
		s.loadEngineFields(s.engineIdx, m.styles)
		cmd := s.syncFocus()
		s.statusMsg = fmt.Sprintf("✓ Saved engine override for %s", eng.ID)
		m.statusMsg = s.statusMsg
		return cmd

	case configTabPorts:
		m.activatePortConfigAction()
		return nil
	}
	return nil
}

func (m *model) resetEngineConfigOverride() tea.Cmd {
	s := m.cfgMgr
	if s == nil || len(s.engines) == 0 {
		return nil
	}
	eng := s.engines[s.engineIdx]
	catDir := m.catalogDir()
	if err := catalog.ResetEngineOverride(catDir, m.catalog, eng.ID); err != nil {
		s.statusMsg = fmt.Sprintf("✗ Reset failed: %v", err)
		return nil
	}
	m.reloadCatalogAndIndex()
	s.engines = m.availableEngines()
	s.loadEngineFields(s.engineIdx, m.styles)
	cmd := s.syncFocus()
	s.statusMsg = fmt.Sprintf("✓ Reset engine override for %s", eng.ID)
	m.statusMsg = s.statusMsg
	return cmd
}
