package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/display"
	"github.com/lock14/doom-cli/internal/portcfg"
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
	engines           []catalog.EngineConfig
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
	confirmDiscardEng bool

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

func (m *model) openConfigManager() tea.Cmd {
	m.input.Blur()
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
		themeSrc = config.DirSource(m.appConfig.Theme)
	}
	nerdSrc := m.nerdFontsSource
	if nerdSrc == "" {
		_, nerdSrc = config.ResolveNerdFontsWithSource(m.appConfig)
	}
	wadsSrc := m.wadsDirSource
	if wadsSrc == "" {
		wadsSrc = config.DirSource(m.appConfig.WadsDir)
	}
	extraSrc := m.wadsDirsSource
	if extraSrc == "" {
		extraSrc = config.WadsDirsSource(m.appConfig)
	}
	binSrc := m.binDirSource
	if binSrc == "" {
		binSrc = config.DirSource(m.appConfig.BinDir)
	}
	sfSrc := m.soundFontsSource
	if sfSrc == "" {
		sfSrc = config.DirSource(m.appConfig.SoundFontsDir)
	}

	engines := m.availableEngines()
	if m.detectedRes == "" || m.detectedRate <= 0 {
		m.detectedRes, m.detectedRate = display.DetectDisplay()
	}
	res, rate := m.detectedRes, m.detectedRate

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
	cmd := s.syncFocus()

	m.cfgMgr = s
	m.viewingConfigMgr = true
	return cmd
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
	s.confirmDiscardEng = false
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
		expected := portcfg.RenderAutoexecWithRate(paths, s.detectedRate)
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
		expected := portcfg.RenderDSDADoomWithRes(paths, s.detectedRes)
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

func (s *configManagerState) syncFocus() tea.Cmd {
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
			return s.wadsDirInput.Focus()
		case cliFieldExtraWadsDirs:
			return s.extraDirsInput.Focus()
		case cliFieldBinDir:
			return s.binDirInput.Focus()
		case cliFieldSoundFontsDir:
			return s.sfDirInput.Focus()
		}
	case configTabEngines:
		switch s.focusIdx {
		case engFieldBinary:
			return s.engBinaryInput.Focus()
		case engFieldDefaultArgs:
			return s.engDefaultInput.Focus()
		case engFieldPrefixArgs:
			return s.engPrefixInput.Focus()
		case engFieldBadge:
			return s.engBadgeInput.Focus()
		case engFieldDisplayName:
			return s.engDisplayInput.Focus()
		}
	}
	return nil
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
			if s.diffViewport.PastBottom() {
				s.diffViewport.GotoBottom()
			}
			return m, nil
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				if m.fetchCancel != nil {
					m.fetchCancel()
				}
				m.quitting = true
				return m, tea.Quit
			case "f1", "alt+?", "?":
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
			if m.fetchCancel != nil {
				m.fetchCancel()
			}
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
			return m, m.input.Focus()

		case "f1", "alt+?":
			m.openHelp(helpModeConfigManager)
			return m, nil

		case "?":
			if !isText {
				m.openHelp(helpModeConfigManager)
				return m, nil
			}

		case "[":
			s.activeTab = (s.activeTab + 2) % 3
			s.focusIdx = 0
			return m, s.syncFocus()

		case "]":
			s.activeTab = (s.activeTab + 1) % 3
			s.focusIdx = 0
			return m, s.syncFocus()

		case "1", "2", "3":
			if !isText {
				s.activeTab = int(keyStr[0] - '1')
				s.focusIdx = 0
				return m, s.syncFocus()
			}

		case "ctrl+s":
			return m, m.saveActiveConfigTab()

		case "ctrl+r":
			if s.activeTab == configTabEngines {
				return m, m.resetEngineConfigOverride()
			}
			return m, nil

		case "up", "ctrl+p", "ctrl+k", "shift+tab":
			count := s.fieldCountForTab()
			s.focusIdx = (s.focusIdx - 1 + count) % count
			return m, s.syncFocus()

		case "down", "ctrl+n", "ctrl+j", "tab":
			count := s.fieldCountForTab()
			s.focusIdx = (s.focusIdx + 1) % count
			return m, s.syncFocus()

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
		s.confirmDiscardEng = false
	}
	return m, cmd
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
