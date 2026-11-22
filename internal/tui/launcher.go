// Package tui provides an interactive terminal user interface for launching Doom presets.
package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/wad"
)

var defaultStyles = CompileStyles(ClassicTheme)

const (
	helpModeBrowser = iota
	helpModeLaunchOptions
	helpModeConfigManager
	helpModeReadme
)

// InitialModeWads opens the TUI in the interactive WAD browser (default).
const InitialModeWads = 0

// InitialModeConfig opens the TUI directly on the CLI settings tab of the configuration manager.
const InitialModeConfig = 1

// InitialModeEngines opens the TUI directly on the engine overrides tab of the configuration manager.
const InitialModeEngines = 2

// LauncherOptions holds optional runtime context for RunInteractiveLauncherWithOptions.
type LauncherOptions struct {
	Paths            *config.Paths
	AppConfig        *config.AppConfig
	CatalogDir       string
	ExtraArgs        []string
	ThemeSource      string
	NerdFontsSource  string
	WadsDirSource    string
	WadsDirsSource   string
	BinDirSource     string
	SoundFontsSource string
	InitialMode      int
	ShowAllPresets   bool
}

type model struct {
	theme            Theme
	styles           ThemeStyles
	nerdFonts        bool
	catalog          *catalog.Catalog
	baseCatalog      *catalog.Catalog
	wadRoots         []string
	paths            *config.Paths
	appConfig        *config.AppConfig
	customCatalogDir string
	extraArgs        []string
	themeSource      string
	nerdFontsSource  string
	wadsDirSource    string
	wadsDirsSource   string
	binDirSource     string
	soundFontsSource string
	initialMode      int
	showAllPresets   bool
	autoCatalog      bool
	explicitAllSet   bool
	visiblePresets   []catalog.Preset
	hiddenCount      int
	installedCount   int
	steamSearchRoots []string
	httpClient       *http.Client
	dirIndex         *catalog.DirIndex
	readmeCache      map[string]string
	searchTargets    []string
	input            textinput.Model
	filtered         []catalog.Preset
	cursor           int
	selected         *catalog.Preset
	width            int
	height           int
	quitting         bool
	statusMsg        string
	lastFetchOutput  string
	fetching         bool
	fetchCancel      context.CancelFunc
	fetchID          uint64
	detectedRes      string
	detectedRate     int

	// Readme viewer state
	viewingReadme bool
	readmeTitle   string
	viewport      viewport.Model

	// Launch Options & Per-Preset Config state (Ctrl+O / F4)
	viewingLaunchOpts bool
	launchOpts        *launchOptionsState

	// 3-Tab Config Manager state (Ctrl+G / F2)
	viewingConfigMgr bool
	cfgMgr           *configManagerState

	// Contextual Help state (F1)
	viewingHelp  bool
	helpMode     int
	helpViewport viewport.Model
}

func readmeCacheKey(p catalog.Preset) string {
	if p.ID != "" {
		return p.ID
	}
	return p.Name
}

func buildSearchTarget(p catalog.Preset, dirIndex *catalog.DirIndex) string {
	readiness := "missing"
	if dirIndex != nil && dirIndex.IsPresetInstalled(p) {
		readiness = "ready installed"
	}
	var files strings.Builder
	for _, f := range p.Files {
		if f.Optional {
			continue
		}
		base := filepath.Base(strings.TrimSpace(f.Name))
		if base == "" || strings.EqualFold(base, "extras.wad") {
			continue
		}
		files.WriteByte(' ')
		files.WriteString(base)
	}
	return strings.TrimSpace(fmt.Sprintf(
		"%s %s %s %s %s %s %s %s%s %s",
		p.Name, p.EffectiveGame(), p.Engine, p.Category, p.Genre, p.Difficulty, p.Quality,
		p.Compatibility, files.String(), readiness,
	))
}

func (m model) effectiveWadRoots() []string {
	if m.paths != nil {
		if roots := m.paths.WadRoots(); len(roots) > 0 {
			return roots
		}
	}
	return m.wadRoots
}

func (m model) wadsWriteDir() string {
	if m.paths != nil && strings.TrimSpace(m.paths.WadsDir) != "" {
		return m.paths.WadsDir
	}
	if len(m.wadRoots) > 0 {
		return m.wadRoots[0]
	}
	return ""
}

func (m model) extraWadRoots() []string {
	roots := m.effectiveWadRoots()
	if len(roots) > 1 {
		return roots[1:]
	}
	return nil
}

func (m *model) catalogDir() string {
	if strings.TrimSpace(m.customCatalogDir) != "" {
		return m.customCatalogDir
	}
	m.ensurePathsAndConfig()
	if m.paths != nil {
		return m.paths.CatalogDir
	}
	return ""
}

func (m model) resolveFile(name string) (string, bool) {
	if m.dirIndex != nil {
		return m.dirIndex.ResolveFile(name)
	}
	return "", false
}

func (m model) resolveReadme(p catalog.Preset) (string, bool) {
	if m.readmeCache != nil {
		if path, cached := m.readmeCache[readmeCacheKey(p)]; cached {
			return path, path != ""
		}
	}
	if m.dirIndex != nil {
		return m.dirIndex.ResolveReadme(p)
	}
	return "", false
}

func (m *model) rebuildIndexAndTargets() {
	if m.catalog == nil {
		m.catalog = &catalog.Catalog{}
	}
	var currentID string
	if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
		currentID = m.filtered[m.cursor].ID
	}

	m.dirIndex = catalog.NewDirIndex(m.effectiveWadRoots()...).WithCatalog(m.catalog)
	libPresets, hidden := m.catalog.LibraryPresets(m.dirIndex)
	m.installedCount = len(libPresets)
	if !m.explicitAllSet && !m.showAllPresets && len(libPresets) == 0 && len(m.catalog.Presets) > 0 {
		m.showAllPresets = true
		m.autoCatalog = true
	}
	if m.showAllPresets {
		m.visiblePresets = m.catalog.Presets
		m.hiddenCount = 0
	} else {
		m.visiblePresets = libPresets
		m.hiddenCount = hidden
	}

	m.readmeCache = make(map[string]string, len(m.visiblePresets))
	m.searchTargets = make([]string, 0, len(m.visiblePresets))
	for _, p := range m.visiblePresets {
		m.searchTargets = append(m.searchTargets, buildSearchTarget(p, m.dirIndex))
		if path, ok := m.dirIndex.ResolveReadme(p); ok {
			m.readmeCache[readmeCacheKey(p)] = path
		} else {
			m.readmeCache[readmeCacheKey(p)] = ""
		}
	}

	m.updateFiltered(m.input.Value())
	if currentID != "" {
		for i, p := range m.filtered {
			if p.ID == currentID {
				m.cursor = i
				break
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *model) reloadCatalogAndIndex() {
	catDir := m.catalogDir()
	if cat, err := catalog.MergeUserDir(m.baseCatalog, catDir); err == nil && cat != nil {
		m.catalog = cat
	}
	m.rebuildIndexAndTargets()
}

func initialModel(
	cat *catalog.Catalog,
	wadRoots []string,
	theme Theme,
	nerdFonts bool,
	initialPreset ...string,
) model {
	if cat == nil {
		cat = &catalog.Catalog{}
	}
	if theme.Name == "" {
		theme = ClassicTheme
	}
	styles := CompileStyles(theme)

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		w = 100
		h = 24
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Type to search presets..."
	ti.PlaceholderStyle = styles.Placeholder
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = calculateSearchWidth(w)

	m := model{
		theme:       theme,
		styles:      styles,
		nerdFonts:   nerdFonts,
		catalog:     cat,
		baseCatalog: cat.Clone(),
		wadRoots:    append([]string(nil), wadRoots...),
		input:       ti,
		width:       w,
		height:      h,
	}
	m.rebuildIndexAndTargets()
	if len(initialPreset) > 0 && initialPreset[0] != "" {
		for i, p := range m.filtered {
			if strings.EqualFold(p.Name, initialPreset[0]) || strings.EqualFold(p.ID, initialPreset[0]) {
				m.cursor = i
				break
			}
		}
	}
	return m
}

type fetchCompleteMsg struct {
	id        uint64
	preset    catalog.Preset
	output    string
	steamErr  error
	dlErr     error
	cancelled bool
}

func (m *model) openHelp(mode int) {
	m.input.Blur()
	_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
	m.helpMode = mode
	m.helpViewport = viewport.New(vpWidth, vpHeight)
	m.helpViewport.SetContent(m.buildHelpContent(mode))
	m.viewingHelp = true
}

func (m *model) restoreFocusCmd() tea.Cmd {
	if m.viewingLaunchOpts && m.launchOpts != nil {
		return m.launchOpts.syncInputFocus()
	}
	if m.viewingConfigMgr && m.cfgMgr != nil {
		if m.cfgMgr.viewingDiff {
			return nil
		}
		return m.cfgMgr.syncFocus()
	}
	if m.viewingReadme || m.viewingHelp {
		return nil
	}
	return m.input.Focus()
}

func (m model) areAllPresetFilesPresent(p catalog.Preset) bool {
	if strings.TrimSpace(p.IWAD) != "" {
		if _, ok := m.resolveFile(p.IWAD); !ok {
			return false
		}
	}
	for _, f := range p.Files {
		if _, ok := m.dirIndex.ResolveFileSpec(f); !ok {
			return false
		}
	}
	return true
}

func (m *model) handleFetchAssets() tea.Cmd {
	if m.fetching {
		return nil
	}
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	cur := m.filtered[m.cursor]
	if m.areAllPresetFilesPresent(cur) {
		m.statusMsg = fmt.Sprintf("✓ Ready: %s", cur.Name)
		return nil
	}

	writeDir := m.wadsWriteDir()
	roots := append([]string(nil), m.effectiveWadRoots()...)
	if writeDir == "" && len(roots) == 0 {
		m.statusMsg = "✗ Fetch failed: no WAD directory configured"
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.fetchID++
	reqID := m.fetchID
	m.fetching = true
	m.fetchCancel = cancel
	m.explicitAllSet = true
	m.autoCatalog = false
	m.statusMsg = fmt.Sprintf("Fetching %s... (Esc to cancel)", cur.Name)

	var singleCat *catalog.Catalog
	if m.catalog != nil {
		singleCat = m.catalog.ForPresets(cur)
	}
	presetCopy := cur
	presetCopy.Files = append([]catalog.FileSpec(nil), cur.Files...)
	presetCopy.Sources = append([]catalog.Source(nil), cur.Sources...)
	needsSteam := m.dirIndex != nil && m.dirIndex.NeedsSteamDiscovery(cur) && writeDir != ""
	searchRoots := append([]string(nil), m.steamSearchRoots...)
	extraRoots := append([]string(nil), m.extraWadRoots()...)
	httpClient := m.httpClient

	return func() tea.Msg {
		defer cancel()
		var buf bytes.Buffer
		var steamErr error
		if needsSteam {
			_, steamErr = steam.DiscoverAndExtractContext(
				ctx, singleCat, searchRoots, writeDir, false, &buf, extraRoots...,
			)
		}

		var dlErr error
		if ctx.Err() == nil && len(roots) > 0 && len(presetCopy.Sources) > 0 {
			dl := wad.NewDownloader(roots, false, &buf)
			dl.Ctx = ctx
			if httpClient != nil {
				dl.Client = httpClient
			}
			dlErr = dl.DownloadPreset(presetCopy)
		}

		return fetchCompleteMsg{
			id:        reqID,
			preset:    presetCopy,
			output:    buf.String(),
			steamErr:  steamErr,
			dlErr:     dlErr,
			cancelled: ctx.Err() != nil,
		}
	}
}

func (m *model) applyFetchComplete(msg fetchCompleteMsg) {
	if msg.id != m.fetchID {
		return
	}
	m.fetching = false
	m.fetchCancel = nil
	m.rebuildIndexAndTargets()
	m.lastFetchOutput = msg.output

	if msg.cancelled || errors.Is(msg.dlErr, context.Canceled) || errors.Is(msg.steamErr, context.Canceled) {
		m.statusMsg = fmt.Sprintf("✗ Fetch cancelled for %s", msg.preset.Name)
		return
	}
	if msg.dlErr != nil {
		m.statusMsg = fmt.Sprintf("✗ Fetch failed: %v", msg.dlErr)
		return
	}
	if msg.steamErr != nil {
		m.statusMsg = fmt.Sprintf("✗ Steam/GOG scan failed: %v", msg.steamErr)
		return
	}
	if m.dirIndex != nil && m.dirIndex.IsPresetInstalled(msg.preset) {
		m.statusMsg = fmt.Sprintf("✓ Ready: %s", msg.preset.Name)
		return
	}
	m.statusMsg = fmt.Sprintf("✗ Missing required files for %s", msg.preset.Name)
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if doneMsg, ok := msg.(fetchCompleteMsg); ok {
		m.applyFetchComplete(doneMsg)
		return m, nil
	}

	if m.viewingHelp {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.input.Width = calculateSearchWidth(m.width)
			_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
			m.helpViewport.Width = vpWidth
			m.helpViewport.Height = vpHeight
			if m.helpViewport.PastBottom() {
				m.helpViewport.GotoBottom()
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
			case "esc", "f1", "alt+?", "?", "q", "Q":
				m.viewingHelp = false
				return m, m.restoreFocusCmd()
			case "g", "home":
				m.helpViewport.GotoTop()
				return m, nil
			case "G", "end":
				m.helpViewport.GotoBottom()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.helpViewport, cmd = m.helpViewport.Update(msg)
		return m, cmd
	}

	if m.viewingLaunchOpts {
		return m.updateLaunchOptions(msg)
	}

	if m.viewingConfigMgr {
		return m.updateConfigManager(msg)
	}

	if m.viewingReadme {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.input.Width = calculateSearchWidth(m.width)
			_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
			if m.viewport.PastBottom() {
				m.viewport.GotoBottom()
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
				m.openHelp(helpModeReadme)
				return m, nil
			case "esc", "q", "Q", "tab", "ctrl+r":
				m.viewingReadme = false
				return m, m.input.Focus()
			case "g", "home":
				m.viewport.GotoTop()
				return m, nil
			case "G", "end":
				m.viewport.GotoBottom()
				return m, nil
			case "enter":
				if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
					cur := m.filtered[m.cursor]
					if !m.isPresetReady(cur) {
						m.viewingReadme = false
						m.statusMsg = fmt.Sprintf("✗ Missing required files for %s (press Ctrl+F to fetch)", cur.Name)
						return m, m.input.Focus()
					}
					m.selected = &cur
					return m, tea.Quit
				}
			}
		}

		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = calculateSearchWidth(m.width)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.fetchCancel != nil {
				m.fetchCancel()
			}
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.fetching {
				if m.fetchCancel != nil {
					m.fetchCancel()
					m.fetchCancel = nil
				}
				m.fetching = false
				m.fetchID++
				m.statusMsg = "✗ Fetch cancelled"
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "f1", "alt+?":
			m.openHelp(helpModeBrowser)
			return m, nil

		case "ctrl+o", "f4":
			return m, m.openLaunchOptions()

		case "ctrl+g", "f2":
			return m, m.openConfigManager()

		case "ctrl+f":
			return m, m.handleFetchAssets()

		case "f3", "ctrl+t":
			totalPresets := 0
			if m.catalog != nil {
				totalPresets = len(m.catalog.Presets)
			}
			if totalPresets > 0 && m.installedCount == totalPresets {
				m.statusMsg = "✓ All catalog presets are installed"
				return m, nil
			}
			m.explicitAllSet = true
			m.autoCatalog = false
			m.showAllPresets = !m.showAllPresets
			m.rebuildIndexAndTargets()
			if m.showAllPresets {
				m.statusMsg = "Showing full catalog (Ctrl+F to fetch)"
			} else {
				m.statusMsg = fmt.Sprintf("Showing installed library (%d ready)", m.installedCount)
			}
			return m, nil

		case "enter":
			if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
				cur := m.filtered[m.cursor]
				if !m.isPresetReady(cur) {
					m.statusMsg = fmt.Sprintf("✗ Missing required files for %s (press Ctrl+F to fetch)", cur.Name)
					return m, nil
				}
				m.selected = &cur
				return m, tea.Quit
			}

		case "tab", "ctrl+r":
			if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
				cur := m.filtered[m.cursor]
				if txtPath, ok := m.resolveReadme(cur); ok {
					content, err := catalog.ReadReadme(txtPath)
					if err == nil {
						m.input.Blur()
						m.viewingReadme = true
						m.readmeTitle = fmt.Sprintf("README: %s (%s)", cur.Name, filepath.Base(txtPath))
						_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
						m.viewport = viewport.New(vpWidth, vpHeight)
						m.viewport.SetContent(content)
						return m, nil
					}
				}
			}
			return m, nil

		case "up", "ctrl+p", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			} else if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, nil

		case "down", "ctrl+n", "ctrl+j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
			return m, nil

		case "pgup", "ctrl+u":
			if len(m.filtered) > 0 {
				step := m.computeLayout().maxVisible
				if msg.String() == "ctrl+u" {
					step /= 2
				}
				if step < 1 {
					step = 1
				}
				m.cursor -= step
				if m.cursor < 0 {
					m.cursor = 0
				}
			}
			return m, nil

		case "pgdown", "ctrl+d":
			if len(m.filtered) > 0 {
				step := m.computeLayout().maxVisible
				if msg.String() == "ctrl+d" {
					step /= 2
				}
				if step < 1 {
					step = 1
				}
				m.cursor += step
				if m.cursor >= len(m.filtered) {
					m.cursor = len(m.filtered) - 1
				}
			}
			return m, nil
		}
	}

	oldVal := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	newVal := m.input.Value()

	if oldVal != newVal {
		m.updateFiltered(newVal)
		m.cursor = 0
	}

	return m, cmd
}

func (m model) isPresetReady(p catalog.Preset) bool {
	return strings.TrimSpace(p.IWAD) != "" && m.dirIndex != nil && m.dirIndex.IsPresetInstalled(p)
}

func (m *model) updateFiltered(query string) {
	if m.catalog == nil {
		m.filtered = nil
		return
	}
	query = strings.TrimSpace(query)
	if query == "" {
		m.filtered = m.visiblePresets
		return
	}

	targets := m.searchTargets
	if len(targets) != len(m.visiblePresets) {
		targets = make([]string, 0, len(m.visiblePresets))
		for _, p := range m.visiblePresets {
			targets = append(targets, buildSearchTarget(p, m.dirIndex))
		}
	}

	matches := fuzzy.Find(query, targets)
	var result []catalog.Preset
	for _, match := range matches {
		result = append(result, m.visiblePresets[match.Index])
	}
	m.filtered = result
}

func (m *model) applyOptions(opts LauncherOptions) {
	m.paths = opts.Paths
	m.appConfig = opts.AppConfig
	m.customCatalogDir = opts.CatalogDir
	m.extraArgs = append([]string(nil), opts.ExtraArgs...)
	m.themeSource = opts.ThemeSource
	m.nerdFontsSource = opts.NerdFontsSource
	m.wadsDirSource = opts.WadsDirSource
	m.wadsDirsSource = opts.WadsDirsSource
	m.binDirSource = opts.BinDirSource
	m.soundFontsSource = opts.SoundFontsSource
	m.initialMode = opts.InitialMode
	if opts.ShowAllPresets {
		m.showAllPresets = true
		m.autoCatalog = false
		m.explicitAllSet = true
		m.rebuildIndexAndTargets()
	}

	switch opts.InitialMode {
	case InitialModeConfig:
		m.openConfigManager()
		if m.cfgMgr != nil {
			m.cfgMgr.activeTab = configTabCLI
			m.cfgMgr.focusIdx = 0
			m.cfgMgr.syncFocus()
		}
	case InitialModeEngines:
		m.openConfigManager()
		if m.cfgMgr != nil {
			m.cfgMgr.activeTab = configTabEngines
			m.cfgMgr.focusIdx = 0
			m.cfgMgr.syncFocus()
		}
	}
}

func (m model) View() string {
	if m.selected != nil {
		return fmt.Sprintf("Launching %s...\n", m.selected.Name)
	}
	if m.quitting {
		if m.initialMode != InitialModeWads {
			return ""
		}
		return "Cancelled.\n"
	}
	if m.viewingHelp {
		return m.renderHelpView()
	}
	if m.viewingLaunchOpts {
		return m.renderLaunchOptionsView()
	}
	if m.viewingConfigMgr {
		return m.renderConfigManagerView()
	}
	if m.viewingReadme {
		return m.renderReadmeView()
	}

	geom := m.computeLayout()
	listLines := m.renderListLines(geom)
	previewLines, hasReadme := m.renderPreviewLines(geom)
	panels := m.renderPanels(geom, listLines, previewLines)

	header := m.renderHeader()
	footer := m.renderFooter(hasReadme)

	return "\n" + header + panels + footer + "\n"
}

// RunInteractiveLauncherWithOptions runs the interactive Bubble Tea UI launcher with full CLI context.
func RunInteractiveLauncherWithOptions(
	cat *catalog.Catalog,
	wadRoots []string,
	theme Theme,
	nerdFonts bool,
	opts LauncherOptions,
) (*catalog.Preset, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		if opts.InitialMode != InitialModeWads {
			return nil, nil
		}
		menuCat := cat
		if !opts.ShowAllPresets && cat != nil {
			dirIdx := catalog.NewDirIndex(wadRoots...).WithCatalog(cat)
			if libCat, _ := cat.FilterLibrary(dirIdx); libCat != nil && len(libCat.Presets) > 0 {
				menuCat = libCat
			}
		}
		return RunNumberedMenu(menuCat, os.Stdin, os.Stdout)
	}

	m := initialModel(cat, wadRoots, theme, nerdFonts)
	m.applyOptions(opts)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	res := finalModel.(model)
	return res.selected, nil
}
