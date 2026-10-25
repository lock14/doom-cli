package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
)

type spinOption struct {
	value string
	label string
}

var skillOptions = []spinOption{
	{value: "", label: "Default"},
	{value: "1", label: "1: ITYTD"},
	{value: "2", label: "2: HNTR"},
	{value: "3", label: "3: HMP"},
	{value: "4", label: "4: Ultra-Violence"},
	{value: "5", label: "5: Nightmare!"},
}

var complevelOptions = []spinOption{
	{value: "", label: "Default"},
	{value: "2", label: "2: Vanilla"},
	{value: "3", label: "3: Ult. Doom"},
	{value: "4", label: "4: Final Doom"},
	{value: "9", label: "9: Boom"},
	{value: "11", label: "11: MBF"},
	{value: "21", label: "21: MBF21"},
}

type optionalFileToggle struct {
	spec    preset.FileSpec
	enabled bool
}

type launchOptionsState struct {
	presetID      string
	presetName    string
	base          preset.Preset
	existingOv    *preset.Override
	engines       []preset.EngineConfig
	engineIdx     int
	skillIdx      int
	warpInput     textinput.Model
	pistolStart   bool
	fast          bool
	noMonsters    bool
	respawn       bool
	soloNet       bool
	complevelIdx  int
	optionalFiles []optionalFileToggle
	extraInput    textinput.Model
	focusIdx      int
	statusMsg     string
}

const (
	launchFieldEngine      = 0
	launchFieldSkill       = 1
	launchFieldWarp        = 2
	launchFieldPistolStart = 3
	launchFieldFast        = 4
	launchFieldNoMonsters  = 5
	launchFieldRespawn     = 6
	launchFieldSoloNet     = 7
	launchFieldComplevel   = 8
	launchFieldOptBase     = 9
)

func (s *launchOptionsState) extraFieldIdx() int {
	return launchFieldOptBase + len(s.optionalFiles)
}

func (s *launchOptionsState) totalFields() int {
	return launchFieldOptBase + len(s.optionalFiles) + 1
}

func (s *launchOptionsState) selectedEngine() preset.EngineConfig {
	if len(s.engines) == 0 {
		return preset.EngineConfig{ID: "dsda-doom", DisplayName: "DSDA-Doom", ArgsStyle: "boom"}
	}
	if s.engineIdx < 0 || s.engineIdx >= len(s.engines) {
		return s.engines[0]
	}
	return s.engines[s.engineIdx]
}

func (s *launchOptionsState) isBoomEngine() bool {
	return s.selectedEngine().EffectiveArgsStyle() == "boom"
}

func (s *launchOptionsState) syncInputFocus() {
	if s.focusIdx == launchFieldWarp {
		s.warpInput.Focus()
	} else {
		s.warpInput.Blur()
	}
	if s.focusIdx == s.extraFieldIdx() {
		s.extraInput.Focus()
	} else {
		s.extraInput.Blur()
	}
}

func (m model) availableEngines() []preset.EngineConfig {
	seen := make(map[string]preset.EngineConfig)
	if m.catalog != nil {
		for k, e := range m.catalog.Engines {
			seen[strings.ToLower(k)] = e
		}
	}
	for _, builtinID := range []string{"dsda-doom", "uzdoom"} {
		if _, exists := seen[builtinID]; !exists {
			if eng, ok := m.catalog.FindEngine(builtinID); ok {
				seen[builtinID] = eng
			}
		}
	}

	var out []preset.EngineConfig
	for _, firstID := range []string{"dsda-doom", "uzdoom"} {
		if eng, ok := seen[firstID]; ok {
			out = append(out, eng)
			delete(seen, firstID)
		}
	}
	var restKeys []string
	for k := range seen {
		restKeys = append(restKeys, k)
	}
	sort.Strings(restKeys)
	for _, k := range restKeys {
		out = append(out, seen[k])
	}
	return out
}

func (m *model) openLaunchOptions() {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return
	}
	cur := m.filtered[m.cursor]
	catDir := m.catalogDir()

	base, ov, err := preset.LoadPresetBaseAndOverride(catDir, cur.ID)
	if m.baseCatalog != nil {
		if bp, findErr := m.baseCatalog.FindExact(cur.ID); findErr == nil && bp != nil && !bp.Overridden {
			baseCopy := *bp
			baseCopy.Files = append([]preset.FileSpec(nil), bp.Files...)
			baseCopy.Args = append([]string(nil), bp.Args...)
			base = &baseCopy
		}
	}
	if err != nil || base == nil {
		baseCopy := cur
		baseCopy.Files = append([]preset.FileSpec(nil), cur.Files...)
		baseCopy.Args = append([]string(nil), cur.Args...)
		base = &baseCopy
	}
	if ov == nil && catDir != "" {
		ovPath := filepath.Join(catDir, fmt.Sprintf("override.preset.%s.json", base.ID))
		if raw, readErr := os.ReadFile(ovPath); readErr == nil {
			if ent, decErr := preset.DecodeEntity(raw); decErr == nil && ent.Override != nil {
				ov = ent.Override
			}
		}
	}

	engines := m.availableEngines()
	engineIdx := 0
	activeEngine := cur.Engine
	if ov != nil && ov.Set != nil && ov.Set.Engine != nil && *ov.Set.Engine != "" {
		activeEngine = *ov.Set.Engine
	}
	for i, e := range engines {
		if strings.EqualFold(e.ID, activeEngine) {
			engineIdx = i
			break
		}
	}

	excludedNorm := make(map[string]bool)
	if ov != nil {
		for _, ex := range ov.ExcludeFiles {
			if norm := preset.NormalizeFilename(ex); norm != "" {
				excludedNorm[norm] = true
			}
			if baseNorm := preset.NormalizeFilename(filepath.Base(ex)); baseNorm != "" {
				excludedNorm[baseNorm] = true
			}
		}
	}

	var optFiles []optionalFileToggle
	for _, f := range base.Files {
		if !f.Optional {
			continue
		}
		norm := preset.NormalizeFilename(f.Name)
		baseNorm := preset.NormalizeFilename(filepath.Base(f.Name))
		enabled := !excludedNorm[norm] && !excludedNorm[baseNorm]
		optFiles = append(optFiles, optionalFileToggle{
			spec:    f,
			enabled: enabled,
		})
	}

	warpInput := textinput.New()
	warpInput.Prompt = ""
	warpInput.Placeholder = "e.g. 01 or 1 1"
	warpInput.PlaceholderStyle = m.styles.Placeholder
	warpInput.CharLimit = 16
	warpInput.Width = 20

	extraInput := textinput.New()
	extraInput.Prompt = ""
	extraInput.Placeholder = "Additional flags..."
	extraInput.PlaceholderStyle = m.styles.Placeholder
	extraInput.CharLimit = 120
	extraInput.Width = 40

	state := &launchOptionsState{
		presetID:      base.ID,
		presetName:    base.Name,
		base:          *base,
		existingOv:    ov,
		engines:       engines,
		engineIdx:     engineIdx,
		warpInput:     warpInput,
		optionalFiles: optFiles,
		extraInput:    extraInput,
		focusIdx:      0,
	}

	var overrideArgs []string
	if ov != nil {
		if ov.Set != nil && ov.Set.Args != nil {
			overrideArgs = append(overrideArgs, (*ov.Set.Args)...)
		}
		overrideArgs = append(overrideArgs, ov.AppendArgs...)
	} else if len(cur.Args) > len(base.Args) {
		overrideArgs = cur.Args[len(base.Args):]
	}
	state.parseArgsIntoFields(overrideArgs)
	state.syncInputFocus()

	m.launchOpts = state
	m.viewingLaunchOpts = true
}

func (s *launchOptionsState) parseArgsIntoFields(args []string) {
	s.skillIdx = 0
	s.warpInput.SetValue("")
	s.pistolStart = false
	s.fast = false
	s.noMonsters = false
	s.respawn = false
	s.soloNet = false
	s.complevelIdx = 0

	var extra []string
	for i := 0; i < len(args); i++ {
		tok := args[i]
		switch strings.ToLower(tok) {
		case "-skill":
			if i+1 < len(args) {
				val := args[i+1]
				matched := false
				for idx, opt := range skillOptions {
					if opt.value == val {
						s.skillIdx = idx
						matched = true
						break
					}
				}
				if matched {
					i++
					continue
				}
			}
		case "-warp":
			var warpParts []string
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && len(warpParts) < 2 {
				i++
				warpParts = append(warpParts, args[i])
			}
			if len(warpParts) > 0 {
				s.warpInput.SetValue(strings.Join(warpParts, " "))
				continue
			}
		case "-pistolstart":
			s.pistolStart = true
			continue
		case "-fast":
			s.fast = true
			continue
		case "-nomonsters":
			s.noMonsters = true
			continue
		case "-respawn":
			s.respawn = true
			continue
		case "-solonet":
			s.soloNet = true
			continue
		case "-complevel":
			if i+1 < len(args) {
				val := args[i+1]
				matched := false
				for idx, opt := range complevelOptions {
					if opt.value == val {
						s.complevelIdx = idx
						matched = true
						break
					}
				}
				if matched {
					i++
					continue
				}
			}
		}
		extra = append(extra, tok)
	}
	s.extraInput.SetValue(strings.Join(extra, " "))
}

func (s *launchOptionsState) synthesizeAppendArgs() []string {
	var args []string
	if s.skillIdx > 0 && s.skillIdx < len(skillOptions) {
		args = append(args, "-skill", skillOptions[s.skillIdx].value)
	}
	if warpFields := strings.Fields(s.warpInput.Value()); len(warpFields) > 0 {
		args = append(args, "-warp")
		args = append(args, warpFields...)
	}
	if s.pistolStart {
		args = append(args, "-pistolstart")
	}
	if s.fast {
		args = append(args, "-fast")
	}
	if s.noMonsters {
		args = append(args, "-nomonsters")
	}
	if s.respawn {
		args = append(args, "-respawn")
	}
	if s.soloNet {
		args = append(args, "-solonet")
	}
	if s.complevelIdx > 0 && s.complevelIdx < len(complevelOptions) {
		args = append(args, "-complevel", complevelOptions[s.complevelIdx].value)
	}
	if extraFields := strings.Fields(s.extraInput.Value()); len(extraFields) > 0 {
		args = append(args, extraFields...)
	}
	return args
}

func (s *launchOptionsState) excludedFileNames() []string {
	var excluded []string
	optNorms := make(map[string]bool, len(s.optionalFiles))
	for _, opt := range s.optionalFiles {
		norm := preset.NormalizeFilename(opt.spec.Name)
		optNorms[norm] = true
		if !opt.enabled {
			excluded = append(excluded, opt.spec.Name)
		}
	}
	if s.existingOv != nil {
		for _, ex := range s.existingOv.ExcludeFiles {
			if !optNorms[preset.NormalizeFilename(ex)] {
				excluded = append(excluded, ex)
			}
		}
	}
	return excluded
}

func (s *launchOptionsState) buildLaunchPreset() preset.Preset {
	p := s.base
	p.Files = append([]preset.FileSpec(nil), s.base.Files...)
	p.Args = append([]string(nil), s.base.Args...)

	if s.existingOv != nil {
		if s.existingOv.Set != nil {
			if s.existingOv.Set.IWAD != nil && *s.existingOv.Set.IWAD != "" {
				p.IWAD = *s.existingOv.Set.IWAD
			}
			if s.existingOv.Set.Files != nil {
				p.Files = append([]preset.FileSpec(nil), (*s.existingOv.Set.Files)...)
			}
			if s.existingOv.Set.Args != nil {
				p.Args = nil
			}
		}
		if len(s.existingOv.AppendFiles) > 0 {
			p.Files = append(p.Files, s.existingOv.AppendFiles...)
		}
	}

	eng := s.selectedEngine()
	p.Engine = eng.ID
	p.EngineOverridden = true

	excluded := s.excludedFileNames()
	if len(excluded) > 0 {
		exMap := make(map[string]bool, len(excluded)*2)
		for _, ex := range excluded {
			if norm := preset.NormalizeFilename(ex); norm != "" {
				exMap[norm] = true
			}
			if baseNorm := preset.NormalizeFilename(filepath.Base(ex)); baseNorm != "" {
				exMap[baseNorm] = true
			}
		}
		var kept []preset.FileSpec
		for _, f := range p.Files {
			norm := preset.NormalizeFilename(f.Name)
			baseNorm := preset.NormalizeFilename(filepath.Base(f.Name))
			if (norm != "" && exMap[norm]) || (baseNorm != "" && exMap[baseNorm]) {
				continue
			}
			kept = append(kept, f)
		}
		p.Files = kept
	}

	appendArgs := s.synthesizeAppendArgs()
	if len(appendArgs) > 0 {
		p.Args = append(p.Args, appendArgs...)
	}
	return p
}

func (m model) updateLaunchOptions(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.launchOpts == nil {
		m.viewingLaunchOpts = false
		return m, nil
	}
	s := m.launchOpts

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = calculateSearchWidth(m.width)
		return m, nil

	case tea.KeyMsg:
		isTextField := s.focusIdx == launchFieldWarp || s.focusIdx == s.extraFieldIdx()
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			m.viewingLaunchOpts = false
			m.launchOpts = nil
			return m, nil

		case "f1":
			m.openHelp(helpModeLaunchOptions)
			return m, nil

		case "enter":
			launched := s.buildLaunchPreset()
			if !m.isPresetReady(launched) {
				m.viewingLaunchOpts = false
				m.launchOpts = nil
				m.statusMsg = fmt.Sprintf("✗ Missing required files for %s (press Ctrl+F to fetch)", launched.Name)
				return m, nil
			}
			m.selected = &launched
			m.viewingLaunchOpts = false
			m.quitting = true
			return m, tea.Quit

		case "ctrl+s":
			m.saveLaunchOptionsOverride()
			return m, nil

		case "ctrl+r":
			m.resetLaunchOptionsOverride()
			return m, nil

		case "up", "ctrl+p", "shift+tab":
			s.focusIdx--
			if s.focusIdx < 0 {
				s.focusIdx = s.totalFields() - 1
			}
			s.syncInputFocus()
			return m, nil

		case "down", "ctrl+n", "tab":
			s.focusIdx++
			if s.focusIdx >= s.totalFields() {
				s.focusIdx = 0
			}
			s.syncInputFocus()
			return m, nil

		case "ctrl+k":
			if !isTextField {
				s.focusIdx--
				if s.focusIdx < 0 {
					s.focusIdx = s.totalFields() - 1
				}
				s.syncInputFocus()
				return m, nil
			}

		case "ctrl+j":
			if !isTextField {
				s.focusIdx++
				if s.focusIdx >= s.totalFields() {
					s.focusIdx = 0
				}
				s.syncInputFocus()
				return m, nil
			}

		case "left":
			if !isTextField {
				s.adjustField(-1)
				return m, nil
			}

		case "right":
			if !isTextField {
				s.adjustField(1)
				return m, nil
			}

		case " ":
			if !isTextField {
				s.adjustField(1)
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch s.focusIdx {
	case launchFieldWarp:
		s.warpInput, cmd = s.warpInput.Update(msg)
	case s.extraFieldIdx():
		s.extraInput, cmd = s.extraInput.Update(msg)
	}
	return m, cmd
}

func (s *launchOptionsState) adjustField(delta int) {
	switch s.focusIdx {
	case launchFieldEngine:
		if len(s.engines) > 0 {
			s.engineIdx = (s.engineIdx + delta + len(s.engines)) % len(s.engines)
		}
	case launchFieldSkill:
		s.skillIdx = (s.skillIdx + delta + len(skillOptions)) % len(skillOptions)
	case launchFieldPistolStart:
		s.pistolStart = !s.pistolStart
	case launchFieldFast:
		s.fast = !s.fast
	case launchFieldNoMonsters:
		s.noMonsters = !s.noMonsters
	case launchFieldRespawn:
		s.respawn = !s.respawn
	case launchFieldSoloNet:
		s.soloNet = !s.soloNet
	case launchFieldComplevel:
		s.complevelIdx = (s.complevelIdx + delta + len(complevelOptions)) % len(complevelOptions)
	default:
		optIdx := s.focusIdx - launchFieldOptBase
		if optIdx >= 0 && optIdx < len(s.optionalFiles) {
			s.optionalFiles[optIdx].enabled = !s.optionalFiles[optIdx].enabled
		}
	}
}

func (m *model) saveLaunchOptionsOverride() {
	if m.launchOpts == nil {
		return
	}
	s := m.launchOpts
	catDir := m.catalogDir()
	engID := s.selectedEngine().ID
	appendArgs := s.synthesizeAppendArgs()
	excludeFiles := s.excludedFileNames()

	req := preset.OverridePatchRequest{
		Engine:       &engID,
		AppendArgs:   &appendArgs,
		ExcludeFiles: &excludeFiles,
	}

	if catDir != "" {
		if _, _, err := preset.ConfigPreset(catDir, m.catalog, s.presetID, req); err != nil {
			s.statusMsg = fmt.Sprintf("✗ Save failed: %v", err)
			return
		}
		if _, ov, err := preset.LoadPresetBaseAndOverride(catDir, s.presetID); err == nil {
			s.existingOv = ov
		}
	}
	m.reloadCatalogAndIndex()
	s.statusMsg = fmt.Sprintf("✓ Saved override for %s", s.presetName)
	m.statusMsg = s.statusMsg
}

func (m *model) resetLaunchOptionsOverride() {
	if m.launchOpts == nil {
		return
	}
	s := m.launchOpts
	catDir := m.catalogDir()
	if catDir != "" {
		if _, _, err := preset.ResetPresetOverride(catDir, m.catalog, s.presetID); err != nil {
			s.statusMsg = fmt.Sprintf("✗ Reset failed: %v", err)
			return
		}
	}
	s.existingOv = nil
	for i, e := range s.engines {
		if strings.EqualFold(e.ID, s.base.Engine) {
			s.engineIdx = i
			break
		}
	}
	s.parseArgsIntoFields(nil)
	for i := range s.optionalFiles {
		s.optionalFiles[i].enabled = true
	}
	m.reloadCatalogAndIndex()
	s.statusMsg = fmt.Sprintf("✓ Reset %s to curated defaults", s.presetName)
	m.statusMsg = s.statusMsg
}

func formatCheckbox(checked bool) string {
	if checked {
		return "[X]"
	}
	return "[ ]"
}

func (m model) renderLaunchOptionsView() string {
	boxWidth, _, interiorHeight := calculateReadmeDimensions(m.width, m.height)
	contentWidth := boxWidth - 2
	s := m.launchOpts

	leftPart := m.styles.RenderBrandPill(m.nerdFonts) +
		m.styles.FilterPrompt.Render("  Launch Options: "+s.presetName)
	rightLabel := " Default "
	if s.existingOv != nil {
		rightLabel = " Override Active "
	}
	rightPart := m.styles.StatsBody.Render(rightLabel)
	if m.nerdFonts {
		rightPart = renderCapsule(m.styles.StatsCap, m.styles.StatsBody, rightLabel)
	}
	header := renderHeaderBar(leftPart, rightPart, m.width)

	dimStyle := m.styles.Help.Faint(true)
	dimFocusStyle := m.styles.CursorText.Faint(true)
	isBoom := s.isBoomEngine()

	type formRow struct {
		label string
		value string
		dim   bool
	}
	rows := []formRow{
		{
			label: "Engine:",
			value: fmt.Sprintf("< %s >", s.selectedEngine().EffectiveDisplayName()),
		},
		{
			label: "Skill (-skill):",
			value: fmt.Sprintf("< %s >", skillOptions[s.skillIdx].label),
		},
		{
			label: "Warp (-warp):",
			value: s.warpInput.View(),
		},
		{
			label: "Pistol Start (-pistolstart):",
			value: formatCheckbox(s.pistolStart),
		},
		{
			label: "Fast Monsters (-fast):",
			value: formatCheckbox(s.fast),
		},
		{
			label: "No Monsters (-nomonsters):",
			value: formatCheckbox(s.noMonsters),
		},
		{
			label: "Respawn (-respawn):",
			value: formatCheckbox(s.respawn),
		},
		{
			label: "Solo-Net (-solonet):",
			value: formatCheckbox(s.soloNet),
			dim:   !isBoom,
		},
		{
			label: "Complevel (-complevel):",
			value: fmt.Sprintf("< %s >", complevelOptions[s.complevelIdx].label),
			dim:   !isBoom,
		},
	}
	for _, opt := range s.optionalFiles {
		rows = append(rows, formRow{
			label: "Optional File:",
			value: fmt.Sprintf("%s %s (Optional)", formatCheckbox(opt.enabled), opt.spec.Name),
		})
	}
	rows = append(rows, formRow{
		label: "Extra Args:",
		value: s.extraInput.View(),
	})

	var renderedRows []string
	for idx, r := range rows {
		paddedLabel := padOrTruncateName(r.label, 28)
		prefix := "  "
		labelStr := m.styles.Label.Render(paddedLabel)
		valStr := r.value
		if idx == s.focusIdx {
			prefix = m.styles.CursorBar.Render("▎ ")
			labelStr = m.styles.CursorText.Render(paddedLabel)
			if idx != launchFieldWarp && idx != s.extraFieldIdx() {
				valStr = m.styles.ValueBold.Render(r.value)
			}
		}
		if r.dim {
			if idx == s.focusIdx {
				labelStr = dimFocusStyle.Render(paddedLabel)
			} else {
				labelStr = dimStyle.Render(paddedLabel)
			}
			valStr = dimStyle.Render(r.value + " (Boom)")
		}
		renderedRows = append(renderedRows, prefix+labelStr+" "+valStr)
	}

	formBudget := interiorHeight - 2
	if formBudget < 1 {
		formBudget = 1
	}
	startIdx := 0
	if s.focusIdx >= formBudget {
		startIdx = s.focusIdx - formBudget + 1
	}
	endIdx := startIdx + formBudget
	if endIdx > len(renderedRows) {
		endIdx = len(renderedRows)
		startIdx = endIdx - formBudget
		if startIdx < 0 {
			startIdx = 0
		}
	}

	lines := make([]string, 0, interiorHeight)
	lines = append(lines, renderedRows[startIdx:endIdx]...)
	for len(lines) < formBudget {
		lines = append(lines, "")
	}

	previewHeader := m.styles.Label.Render("Command Preview:")
	if s.statusMsg != "" {
		statusStyled := m.styles.StatusReady.Render(s.statusMsg)
		if strings.HasPrefix(s.statusMsg, "✗") {
			statusStyled = m.styles.StatusMissing.Render(s.statusMsg)
		}
		previewHeader += "  " + statusStyled
	}
	previewCmd := engine.FormatLaunchPreview(s.buildLaunchPreset(), m.catalog, m.dirIndex, m.extraArgs)
	lines = append(lines, previewHeader, "  "+previewCmd)

	boxContent := formatBoxContent(lines, contentWidth, interiorHeight)
	boxTitle := fmt.Sprintf("Launch Options — %s", s.presetName)
	box := renderBoxWithTitle(boxContent, boxWidth, boxTitle, true, m.styles)
	footer := m.renderLaunchOptionsFooter()
	return "\n" + header + box + footer + "\n"
}

func (m model) renderLaunchOptionsFooter() string {
	items := []keyHelp{
		{"↑/↓/Tab", "Field"},
		{"←/→/Space", "Adjust"},
		{"Enter", "Launch"},
		{"Ctrl+S", "Save"},
		{"Ctrl+R", "Reset"},
		{"F1", "Help"},
		{"Esc", "Back"},
	}
	footer := m.styles.FormatKeyHelp(items)
	if m.width > 0 {
		return "\n\n" + truncateANSI(strings.TrimPrefix(footer, "\n\n"), m.width)
	}
	return footer
}
