package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/display"
	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/templates"
)

const (
	configTabCLI     = 0
	configTabEngines = 1
	configTabPorts   = 2
)

const (
	cliFieldTheme         = 0
	cliFieldNerdFonts     = 1
	cliFieldWadsDir       = 2
	cliFieldExtraWadsDirs = 3
	cliFieldBinDir        = 4
	cliFieldSoundFontsDir = 5
	cliFieldCount         = 6
)

const (
	engFieldSelect      = 0
	engFieldBinary      = 1
	engFieldArgsStyle   = 2
	engFieldDefaultArgs = 3
	engFieldPrefixArgs  = 4
	engFieldBadge       = 5
	engFieldDisplayName = 6
	engFieldCount       = 7
)

const (
	portFieldViewDiff = 0
	portFieldDeploy   = 1
	portFieldCount    = 2
)

type configManagerState struct {
	activeTab int
	focusIdx  int

	// Saved baseline for reverting live preview on Esc
	savedTheme     Theme
	savedNerdFonts bool

	// Tab 1: CLI Settings
	themes           []Theme
	themeIdx         int
	initialThemeName string
	initialNerdFonts bool
	wadsDirInput     textinput.Model
	extraDirsInput   textinput.Model
	binDirInput      textinput.Model
	sfDirInput       textinput.Model
	initialWadsDir   string
	initialExtraDirs string
	initialBinDir    string
	initialSFDir     string
	themeSource      string
	nerdFontsSource  string
	wadsDirSource    string
	extraDirsSource  string
	binDirSource     string
	sfDirSource      string

	// Tab 2: Engine Overrides
	engines           []preset.EngineConfig
	engineIdx         int
	engBinaryInput    textinput.Model
	engArgsStyle      string
	engDefaultInput   textinput.Model
	engPrefixInput    textinput.Model
	engBadgeInput     textinput.Model
	engDisplayInput   textinput.Model
	initialEngBinary  string
	initialEngStyle   string
	initialEngDefault string
	initialEngPrefix  string
	initialEngBadge   string
	initialEngDisplay string

	// Tab 3: Port Configs
	detectedRes  string
	detectedRate int
	uzdoomStatus string
	dsdaStatus   string
	viewingDiff  bool
	diffViewport viewport.Model

	statusMsg string
}

func (s *configManagerState) applyStyles(styles ThemeStyles) {
	s.wadsDirInput.PlaceholderStyle = styles.Placeholder
	s.extraDirsInput.PlaceholderStyle = styles.Placeholder
	s.binDirInput.PlaceholderStyle = styles.Placeholder
	s.sfDirInput.PlaceholderStyle = styles.Placeholder
	s.engBinaryInput.PlaceholderStyle = styles.Placeholder
	s.engDefaultInput.PlaceholderStyle = styles.Placeholder
	s.engPrefixInput.PlaceholderStyle = styles.Placeholder
	s.engBadgeInput.PlaceholderStyle = styles.Placeholder
	s.engDisplayInput.PlaceholderStyle = styles.Placeholder
}

func newConfigTextInput(val, placeholder string, styles ThemeStyles) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.PlaceholderStyle = styles.Placeholder
	ti.CharLimit = 256
	ti.Width = 44
	ti.SetValue(val)
	return ti
}

func (m *model) ensurePathsAndConfig() {
	if m.paths == nil {
		m.paths = config.GetPaths()
		if len(m.wadRoots) > 0 {
			m.paths.WadsDir = m.wadRoots[0]
			if len(m.wadRoots) > 1 {
				m.paths.WadsDirs = append([]string(nil), m.wadRoots[1:]...)
			}
		}
	}
	if m.appConfig == nil {
		cfg, err := config.LoadConfig(m.paths)
		if err != nil || cfg == nil {
			cfg = &config.AppConfig{}
		}
		m.appConfig = cfg
	}
}

func detectSourceBadge(cfgSet bool) string {
	if cfgSet {
		return "config"
	}
	return "default"
}

func (m *model) openConfigManager() {
	m.ensurePathsAndConfig()

	themes := append([]Theme(nil), ListBuiltinThemes()...)
	themes = append(themes, ListCustomThemes(m.paths.ThemesDir)...)
	themeIdx := 0
	for i, t := range themes {
		if strings.EqualFold(t.Name, m.theme.Name) {
			themeIdx = i
			break
		}
	}

	extraDirsStr := strings.Join(m.paths.WadsDirs, ", ")

	wadsInput := newConfigTextInput(m.paths.WadsDir, "Primary WADs directory...", m.styles)
	extraInput := newConfigTextInput(extraDirsStr, "Comma-separated extra WAD directories...", m.styles)
	binInput := newConfigTextInput(m.paths.BinDir, "Engine binaries directory...", m.styles)
	sfInput := newConfigTextInput(m.paths.SoundFontsDir, "SoundFonts directory...", m.styles)

	themeSrc := m.themeSource
	if themeSrc == "" {
		themeSrc = detectSourceBadge(strings.TrimSpace(m.appConfig.Theme) != "")
	}
	nerdSrc := m.nerdFontsSource
	if nerdSrc == "" {
		nerdSrc = detectSourceBadge(m.appConfig.HasNerdFonts())
	}
	wadsSrc := m.wadsDirSource
	if wadsSrc == "" {
		wadsSrc = detectSourceBadge(strings.TrimSpace(m.appConfig.WadsDir) != "")
	}
	extraSrc := m.wadsDirsSource
	if extraSrc == "" {
		extraSrc = detectSourceBadge(len(m.appConfig.WadsDirs) > 0)
	}
	binSrc := m.binDirSource
	if binSrc == "" {
		binSrc = detectSourceBadge(strings.TrimSpace(m.appConfig.BinDir) != "")
	}
	sfSrc := m.soundFontsSource
	if sfSrc == "" {
		sfSrc = detectSourceBadge(strings.TrimSpace(m.appConfig.SoundFontsDir) != "")
	}

	engines := m.availableEngines()
	res, rate := display.DetectDisplay()

	s := &configManagerState{
		activeTab:        configTabCLI,
		focusIdx:         0,
		savedTheme:       m.theme,
		savedNerdFonts:   m.nerdFonts,
		themes:           themes,
		themeIdx:         themeIdx,
		initialThemeName: m.theme.Name,
		initialNerdFonts: m.nerdFonts,
		wadsDirInput:     wadsInput,
		extraDirsInput:   extraInput,
		binDirInput:      binInput,
		sfDirInput:       sfInput,
		initialWadsDir:   m.paths.WadsDir,
		initialExtraDirs: extraDirsStr,
		initialBinDir:    m.paths.BinDir,
		initialSFDir:     m.paths.SoundFontsDir,
		themeSource:      themeSrc,
		nerdFontsSource:  nerdSrc,
		wadsDirSource:    wadsSrc,
		extraDirsSource:  extraSrc,
		binDirSource:     binSrc,
		sfDirSource:      sfSrc,
		engines:          engines,
		engineIdx:        0,
		detectedRes:      res,
		detectedRate:     rate,
	}

	s.loadEngineFields(0, m.styles)
	s.refreshPortConfigStatus(m.paths)
	s.syncFocus()

	m.cfgMgr = s
	m.viewingConfigMgr = true
}

func (s *configManagerState) loadEngineFields(idx int, styles ThemeStyles) {
	if len(s.engines) == 0 {
		return
	}
	if idx < 0 || idx >= len(s.engines) {
		idx = 0
	}
	s.engineIdx = idx
	eng := s.engines[idx]

	bin := eng.EffectiveBinary()
	style := eng.EffectiveArgsStyle()
	defArgs := strings.Join(eng.DefaultArgs, " ")
	prefArgs := strings.Join(eng.PrefixArgs, " ")
	badge := eng.BadgeText()
	dispName := eng.EffectiveDisplayName()

	s.engBinaryInput = newConfigTextInput(bin, "Executable name or path...", styles)
	s.engArgsStyle = style
	s.engDefaultInput = newConfigTextInput(defArgs, "Default engine flags...", styles)
	s.engPrefixInput = newConfigTextInput(prefArgs, "Prefix args (before -iwad)...", styles)
	s.engBadgeInput = newConfigTextInput(badge, "Badge label (up to 6 chars)...", styles)
	s.engDisplayInput = newConfigTextInput(dispName, "Human-friendly display name...", styles)

	s.initialEngBinary = bin
	s.initialEngStyle = style
	s.initialEngDefault = defArgs
	s.initialEngPrefix = prefArgs
	s.initialEngBadge = badge
	s.initialEngDisplay = dispName
}

func (s *configManagerState) refreshPortConfigStatus(paths *config.Paths) {
	if paths == nil {
		s.uzdoomStatus = "[✗ Not Installed]"
		s.dsdaStatus = "[✗ Not Installed]"
		return
	}
	autoDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if diskData, err := os.ReadFile(autoDest); err != nil {
		s.uzdoomStatus = "[✗ Not Installed]"
	} else {
		expected := templates.RenderAutoexecWithRate(paths, s.detectedRate)
		if strings.TrimSpace(string(diskData)) == strings.TrimSpace(expected) {
			s.uzdoomStatus = "[✓ In Sync]"
		} else {
			s.uzdoomStatus = "[● Modified]"
		}
	}

	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	if diskData, err := os.ReadFile(dsdaDest); err != nil {
		s.dsdaStatus = "[✗ Not Installed]"
	} else {
		expected := templates.RenderDSDADoomWithRes(paths, s.detectedRes)
		if strings.TrimSpace(string(diskData)) == strings.TrimSpace(expected) {
			s.dsdaStatus = "[✓ In Sync]"
		} else {
			s.dsdaStatus = "[● Modified]"
		}
	}
}

func (s *configManagerState) isTab1Dirty(currentThemeName string, currentNerdFonts bool) bool {
	return !strings.EqualFold(currentThemeName, s.initialThemeName) ||
		currentNerdFonts != s.initialNerdFonts ||
		strings.TrimSpace(s.wadsDirInput.Value()) != strings.TrimSpace(s.initialWadsDir) ||
		strings.TrimSpace(s.extraDirsInput.Value()) != strings.TrimSpace(s.initialExtraDirs) ||
		strings.TrimSpace(s.binDirInput.Value()) != strings.TrimSpace(s.initialBinDir) ||
		strings.TrimSpace(s.sfDirInput.Value()) != strings.TrimSpace(s.initialSFDir)
}

func (s *configManagerState) isTab2Dirty() bool {
	return strings.TrimSpace(s.engBinaryInput.Value()) != strings.TrimSpace(s.initialEngBinary) ||
		s.engArgsStyle != s.initialEngStyle ||
		strings.TrimSpace(s.engDefaultInput.Value()) != strings.TrimSpace(s.initialEngDefault) ||
		strings.TrimSpace(s.engPrefixInput.Value()) != strings.TrimSpace(s.initialEngPrefix) ||
		strings.TrimSpace(s.engBadgeInput.Value()) != strings.TrimSpace(s.initialEngBadge) ||
		strings.TrimSpace(s.engDisplayInput.Value()) != strings.TrimSpace(s.initialEngDisplay)
}

func (s *configManagerState) fieldCountForTab() int {
	switch s.activeTab {
	case configTabCLI:
		return cliFieldCount
	case configTabEngines:
		return engFieldCount
	case configTabPorts:
		return portFieldCount
	default:
		return 1
	}
}

func (s *configManagerState) isTextFieldFocused() bool {
	switch s.activeTab {
	case configTabCLI:
		return s.focusIdx >= cliFieldWadsDir && s.focusIdx <= cliFieldSoundFontsDir
	case configTabEngines:
		return s.focusIdx == engFieldBinary ||
			s.focusIdx == engFieldDefaultArgs ||
			s.focusIdx == engFieldPrefixArgs ||
			s.focusIdx == engFieldBadge ||
			s.focusIdx == engFieldDisplayName
	default:
		return false
	}
}

func (s *configManagerState) syncFocus() {
	s.wadsDirInput.Blur()
	s.extraDirsInput.Blur()
	s.binDirInput.Blur()
	s.sfDirInput.Blur()
	s.engBinaryInput.Blur()
	s.engDefaultInput.Blur()
	s.engPrefixInput.Blur()
	s.engBadgeInput.Blur()
	s.engDisplayInput.Blur()

	switch s.activeTab {
	case configTabCLI:
		switch s.focusIdx {
		case cliFieldWadsDir:
			s.wadsDirInput.Focus()
		case cliFieldExtraWadsDirs:
			s.extraDirsInput.Focus()
		case cliFieldBinDir:
			s.binDirInput.Focus()
		case cliFieldSoundFontsDir:
			s.sfDirInput.Focus()
		}
	case configTabEngines:
		switch s.focusIdx {
		case engFieldBinary:
			s.engBinaryInput.Focus()
		case engFieldDefaultArgs:
			s.engDefaultInput.Focus()
		case engFieldPrefixArgs:
			s.engPrefixInput.Focus()
		case engFieldBadge:
			s.engBadgeInput.Focus()
		case engFieldDisplayName:
			s.engDisplayInput.Focus()
		}
	}
}

func (m model) updateConfigManager(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.cfgMgr == nil {
		m.viewingConfigMgr = false
		return m, nil
	}
	s := m.cfgMgr

	if s.viewingDiff {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.input.Width = calculateSearchWidth(m.width)
			_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
			s.diffViewport.Width = vpWidth
			s.diffViewport.Height = vpHeight
			return m, nil
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "f1":
				m.openHelp(helpModeConfigManager)
				return m, nil
			case "esc", "q", "Q", "enter":
				s.viewingDiff = false
				return m, nil
			case "g", "home":
				s.diffViewport.GotoTop()
				return m, nil
			case "G", "end":
				s.diffViewport.GotoBottom()
				return m, nil
			}
		}
		var cmd tea.Cmd
		s.diffViewport, cmd = s.diffViewport.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = calculateSearchWidth(m.width)
		return m, nil

	case tea.KeyMsg:
		isText := s.isTextFieldFocused()
		keyStr := msg.String()

		switch keyStr {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			m.theme = s.savedTheme
			m.styles = CompileStyles(s.savedTheme)
			m.input.PlaceholderStyle = m.styles.Placeholder
			m.nerdFonts = s.savedNerdFonts
			m.viewingConfigMgr = false
			m.cfgMgr = nil
			if m.initialMode == InitialModeConfig || m.initialMode == InitialModeEngines {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil

		case "f1":
			m.openHelp(helpModeConfigManager)
			return m, nil

		case "[":
			s.activeTab = (s.activeTab + 2) % 3
			s.focusIdx = 0
			s.syncFocus()
			return m, nil

		case "]":
			s.activeTab = (s.activeTab + 1) % 3
			s.focusIdx = 0
			s.syncFocus()
			return m, nil

		case "1", "2", "3":
			if !isText {
				s.activeTab = int(keyStr[0] - '1')
				s.focusIdx = 0
				s.syncFocus()
				return m, nil
			}

		case "ctrl+s":
			m.saveActiveConfigTab()
			return m, nil

		case "ctrl+r":
			if s.activeTab == configTabEngines {
				m.resetEngineConfigOverride()
			}
			return m, nil

		case "up", "ctrl+p", "ctrl+k", "shift+tab":
			count := s.fieldCountForTab()
			s.focusIdx = (s.focusIdx - 1 + count) % count
			s.syncFocus()
			return m, nil

		case "down", "ctrl+n", "ctrl+j", "tab":
			count := s.fieldCountForTab()
			s.focusIdx = (s.focusIdx + 1) % count
			s.syncFocus()
			return m, nil

		case "left":
			if !isText {
				m.adjustConfigField(-1)
				return m, nil
			}

		case "right":
			if !isText {
				m.adjustConfigField(1)
				return m, nil
			}

		case " ":
			if !isText {
				if s.activeTab == configTabPorts {
					m.activatePortConfigAction()
				} else {
					m.adjustConfigField(1)
				}
				return m, nil
			}

		case "enter":
			if s.activeTab == configTabPorts {
				m.activatePortConfigAction()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch s.activeTab {
	case configTabCLI:
		switch s.focusIdx {
		case cliFieldWadsDir:
			s.wadsDirInput, cmd = s.wadsDirInput.Update(msg)
		case cliFieldExtraWadsDirs:
			s.extraDirsInput, cmd = s.extraDirsInput.Update(msg)
		case cliFieldBinDir:
			s.binDirInput, cmd = s.binDirInput.Update(msg)
		case cliFieldSoundFontsDir:
			s.sfDirInput, cmd = s.sfDirInput.Update(msg)
		}
	case configTabEngines:
		switch s.focusIdx {
		case engFieldBinary:
			s.engBinaryInput, cmd = s.engBinaryInput.Update(msg)
		case engFieldDefaultArgs:
			s.engDefaultInput, cmd = s.engDefaultInput.Update(msg)
		case engFieldPrefixArgs:
			s.engPrefixInput, cmd = s.engPrefixInput.Update(msg)
		case engFieldBadge:
			s.engBadgeInput, cmd = s.engBadgeInput.Update(msg)
		case engFieldDisplayName:
			s.engDisplayInput, cmd = s.engDisplayInput.Update(msg)
		}
	}
	return m, cmd
}

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
				nextIdx := (s.engineIdx + delta + len(s.engines)) % len(s.engines)
				s.loadEngineFields(nextIdx, m.styles)
			}
		case engFieldArgsStyle:
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
		_ = templates.DiffConfigs(m.paths, &buf)
		_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
		s.diffViewport = viewport.New(vpWidth, vpHeight)
		s.diffViewport.SetContent(buf.String())
		s.viewingDiff = true
	case portFieldDeploy:
		var buf bytes.Buffer
		if err := templates.DeployConfigsTo(m.paths, &buf); err != nil {
			s.statusMsg = fmt.Sprintf("✗ Deploy failed: %v", err)
			return
		}
		s.refreshPortConfigStatus(m.paths)
		s.statusMsg = "✓ Deployed curated autoexec.cfg and dsda-doom.cfg"
		m.statusMsg = s.statusMsg
	}
}

func (m *model) saveActiveConfigTab() {
	s := m.cfgMgr
	if s == nil {
		return
	}
	m.ensurePathsAndConfig()

	switch s.activeTab {
	case configTabCLI:
		m.appConfig.Theme = m.theme.Name
		m.appConfig.SetNerdFonts(m.nerdFonts)

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
			return
		}

		s.savedTheme = m.theme
		s.savedNerdFonts = m.nerdFonts
		s.initialThemeName = m.theme.Name
		s.initialNerdFonts = m.nerdFonts
		s.initialWadsDir = wadsVal
		s.initialExtraDirs = extraVal
		s.initialBinDir = binVal
		s.initialSFDir = sfVal
		if s.themeSource == "default" {
			s.themeSource = "config"
		}
		if s.nerdFontsSource == "default" {
			s.nerdFontsSource = "config"
		}

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

	case configTabEngines:
		if len(s.engines) == 0 {
			return
		}
		eng := s.engines[s.engineIdx]
		bin := strings.TrimSpace(s.engBinaryInput.Value())
		style := strings.TrimSpace(s.engArgsStyle)
		defArgs := strings.Fields(s.engDefaultInput.Value())
		prefArgs := strings.Fields(s.engPrefixInput.Value())
		badge := strings.TrimSpace(s.engBadgeInput.Value())
		dispName := strings.TrimSpace(s.engDisplayInput.Value())
		emptyAppend := []string{}

		opts := preset.EngineOverrideOptions{
			Binary:      &bin,
			ArgsStyle:   &style,
			DefaultArgs: &defArgs,
			AppendArgs:  &emptyAppend,
			PrefixArgs:  &prefArgs,
			Badge:       &badge,
			DisplayName: &dispName,
		}
		catDir := m.catalogDir()
		if _, err := preset.ConfigEngine(catDir, m.catalog, eng.ID, opts); err != nil {
			s.statusMsg = fmt.Sprintf("✗ Save failed: %v", err)
			return
		}
		m.reloadCatalogAndIndex()
		s.engines = m.availableEngines()
		s.loadEngineFields(s.engineIdx, m.styles)
		s.syncFocus()
		s.statusMsg = fmt.Sprintf("✓ Saved engine override for %s", eng.ID)
		m.statusMsg = s.statusMsg

	case configTabPorts:
		m.activatePortConfigAction()
	}
}

func (m *model) resetEngineConfigOverride() {
	s := m.cfgMgr
	if s == nil || len(s.engines) == 0 {
		return
	}
	eng := s.engines[s.engineIdx]
	catDir := m.catalogDir()
	if err := preset.ResetEngineOverride(catDir, m.catalog, eng.ID); err != nil {
		s.statusMsg = fmt.Sprintf("✗ Reset failed: %v", err)
		return
	}
	m.reloadCatalogAndIndex()
	s.engines = m.availableEngines()
	s.loadEngineFields(s.engineIdx, m.styles)
	s.syncFocus()
	s.statusMsg = fmt.Sprintf("✓ Reset engine override for %s", eng.ID)
	m.statusMsg = s.statusMsg
}

func (m model) renderConfigManagerView() string {
	s := m.cfgMgr
	if s.viewingDiff {
		boxWidth, _, _ := calculateReadmeDimensions(m.width, m.height)
		leftPart := m.styles.RenderBrandPill(m.nerdFonts) +
			m.styles.FilterPrompt.Render("  Port Configs Diff")
		rightPart := m.styles.RenderScrollPill(s.diffViewport.ScrollPercent(), m.nerdFonts)
		header := renderHeaderBar(leftPart, rightPart, m.width)
		box := renderBoxWithTitle(s.diffViewport.View(), boxWidth, "Configuration Diff", true, m.styles)
		footer := m.styles.FormatKeyHelp([]keyHelp{
			{"↑/↓/PgUp/PgDn/g/G", "Scroll"},
			{"F1", "Help"},
			{"Esc/q", "Back to Port Configs"},
		})
		if m.width > 0 {
			footer = "\n\n" + truncateANSI(strings.TrimPrefix(footer, "\n\n"), m.width)
		}
		return "\n" + header + box + footer + "\n"
	}

	boxWidth, _, interiorHeight := calculateReadmeDimensions(m.width, m.height)
	contentWidth := boxWidth - 2

	leftPart := m.styles.RenderBrandPill(m.nerdFonts) +
		m.styles.FilterPrompt.Render("  Configuration Manager")
	tabBadge := fmt.Sprintf(" Tab %d / 3 ", s.activeTab+1)
	rightPart := m.styles.StatsBody.Render(tabBadge)
	if m.nerdFonts {
		rightPart = renderCapsule(m.styles.StatsCap, m.styles.StatsBody, tabBadge)
	}
	header := renderHeaderBar(leftPart, rightPart, m.width)

	tab1Title := "1. CLI Settings"
	if s.isTab1Dirty(m.theme.Name, m.nerdFonts) {
		tab1Title += " ●"
	}
	tab2Title := "2. Engine Overrides"
	if s.isTab2Dirty() {
		tab2Title += " ●"
	}
	tab3Title := "3. Port Configs"

	tabTitles := []string{tab1Title, tab2Title, tab3Title}
	var tabParts []string
	for idx, title := range tabTitles {
		if idx == s.activeTab {
			tabParts = append(tabParts, m.styles.CursorText.Render("["+title+"]"))
		} else {
			tabParts = append(tabParts, m.styles.TitleInactive.Render(" "+title+" "))
		}
	}
	tabBar := "  " + strings.Join(tabParts, "   ")

	var bodyLines []string
	focusLine := s.focusIdx
	switch s.activeTab {
	case configTabCLI:
		bodyLines = m.renderConfigTabCLI()
	case configTabEngines:
		bodyLines = m.renderConfigTabEngines()
	case configTabPorts:
		bodyLines = m.renderConfigTabPorts()
		focusLine = 4 + s.focusIdx
	}

	bodyBudget := interiorHeight - 2
	if s.statusMsg != "" {
		bodyBudget--
	}
	if bodyBudget < 1 {
		bodyBudget = 1
	}
	if len(bodyLines) > bodyBudget {
		startIdx := 0
		if focusLine >= bodyBudget {
			startIdx = focusLine - bodyBudget + 1
		}
		endIdx := startIdx + bodyBudget
		if endIdx > len(bodyLines) {
			endIdx = len(bodyLines)
			startIdx = endIdx - bodyBudget
			if startIdx < 0 {
				startIdx = 0
			}
		}
		bodyLines = bodyLines[startIdx:endIdx]
	}

	var lines []string
	lines = append(lines, tabBar, "")
	lines = append(lines, bodyLines...)

	if s.statusMsg != "" {
		for len(lines) < interiorHeight-1 {
			lines = append(lines, "")
		}
		statusStyled := m.styles.StatusReady.Render("  " + s.statusMsg)
		if strings.HasPrefix(s.statusMsg, "✗") {
			statusStyled = m.styles.StatusMissing.Render("  " + s.statusMsg)
		}
		if len(lines) >= interiorHeight {
			lines[interiorHeight-1] = statusStyled
		} else {
			lines = append(lines, statusStyled)
		}
	}

	boxContent := formatBoxContent(lines, contentWidth, interiorHeight)
	box := renderBoxWithTitle(boxContent, boxWidth, "Doom CLI & Engine Configuration", true, m.styles)
	footer := m.renderConfigManagerFooter()
	return "\n" + header + box + footer + "\n"
}

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
		items = append(items,
			keyHelp{"Enter/Space", "Run Action"},
			keyHelp{"F1", "Help"},
			keyHelp{"Esc", "Close"},
		)
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
