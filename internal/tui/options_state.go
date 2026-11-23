package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/catalog"
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
	spec    catalog.FileSpec
	enabled bool
}

type launchOptionsState struct {
	presetID      string
	presetName    string
	base          catalog.Preset
	existingOv    *catalog.Override
	engines       []catalog.EngineConfig
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

func (s *launchOptionsState) selectedEngine() catalog.EngineConfig {
	if len(s.engines) == 0 {
		return catalog.EngineConfig{ID: "dsda-doom", DisplayName: "DSDA-Doom", ArgsStyle: "boom"}
	}
	if s.engineIdx < 0 || s.engineIdx >= len(s.engines) {
		return s.engines[0]
	}
	return s.engines[s.engineIdx]
}

func (s *launchOptionsState) isBoomEngine() bool {
	return s.selectedEngine().EffectiveArgsStyle() == "boom"
}

func (s *launchOptionsState) isFieldDisabled(idx int) bool {
	if !s.isBoomEngine() {
		return idx == launchFieldSoloNet || idx == launchFieldComplevel
	}
	return false
}

func (s *launchOptionsState) moveFocus(delta int) {
	total := s.totalFields()
	if total <= 0 {
		s.focusIdx = 0
		return
	}
	for range total {
		s.focusIdx = (s.focusIdx + delta + total) % total
		if !s.isFieldDisabled(s.focusIdx) {
			return
		}
	}
}

func (s *launchOptionsState) syncInputFocus() tea.Cmd {
	var cmds []tea.Cmd
	if s.focusIdx == launchFieldWarp {
		cmds = append(cmds, s.warpInput.Focus())
	} else {
		s.warpInput.Blur()
	}
	if s.focusIdx == s.extraFieldIdx() {
		cmds = append(cmds, s.extraInput.Focus())
	} else {
		s.extraInput.Blur()
	}
	return tea.Batch(cmds...)
}

func clonePresetShallowSlices(src catalog.Preset) catalog.Preset {
	cp := src
	cp.Files = append([]catalog.FileSpec(nil), src.Files...)
	cp.Args = append([]string(nil), src.Args...)
	if src.Complevel != nil {
		cl := *src.Complevel
		cp.Complevel = &cl
	}
	if len(src.EngineArgs) > 0 {
		cp.EngineArgs = make(map[string][]string, len(src.EngineArgs))
		for k, v := range src.EngineArgs {
			cp.EngineArgs[k] = append([]string(nil), v...)
		}
	}
	return cp
}

func fileNormSet(names []string) map[string]bool {
	out := make(map[string]bool, len(names)*2)
	for _, raw := range names {
		if norm := catalog.NormalizeFilename(raw); norm != "" {
			out[norm] = true
		}
		if baseNorm := catalog.NormalizeFilename(filepath.Base(raw)); baseNorm != "" {
			out[baseNorm] = true
		}
	}
	return out
}

func matchesFileNorm(set map[string]bool, name string) bool {
	norm := catalog.NormalizeFilename(name)
	baseNorm := catalog.NormalizeFilename(filepath.Base(name))
	return (norm != "" && set[norm]) || (baseNorm != "" && set[baseNorm])
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

func containsFlag(args []string, flag string) bool {
	for _, tok := range args {
		if strings.EqualFold(strings.TrimSpace(tok), flag) {
			return true
		}
	}
	return false
}

func filterOutFlag(args []string, flag string) []string {
	var out []string
	for _, tok := range args {
		if !strings.EqualFold(strings.TrimSpace(tok), flag) {
			out = append(out, tok)
		}
	}
	return out
}

func (s *launchOptionsState) selectedComplevelOverride() (*int, bool) {
	if !s.isBoomEngine() || s.complevelIdx <= 0 || s.complevelIdx >= len(complevelOptions) {
		return nil, true
	}
	cl, err := strconv.Atoi(complevelOptions[s.complevelIdx].value)
	if err != nil {
		return nil, true
	}
	if s.base.Complevel != nil && *s.base.Complevel == cl {
		return nil, true
	}
	return &cl, false
}

func (s *launchOptionsState) complevelDisplayLabel() string {
	if s.complevelIdx == 0 && s.base.Complevel != nil {
		return fmt.Sprintf("Default (%s)", catalog.ComplevelLabel(*s.base.Complevel))
	}
	if s.complevelIdx >= 0 && s.complevelIdx < len(complevelOptions) {
		return complevelOptions[s.complevelIdx].label
	}
	return complevelOptions[0].label
}

func (s *launchOptionsState) synthesizeAppendEngineArgs() map[string][]string {
	out := make(map[string][]string)
	engID := s.selectedEngine().ID
	if s.existingOv != nil {
		for k, slice := range s.existingOv.AppendEngineArgs {
			if strings.EqualFold(k, engID) {
				kept := filterOutFlag(slice, "-solonet")
				if len(kept) > 0 {
					out[engID] = kept
				}
			} else if len(slice) > 0 {
				out[k] = append([]string(nil), slice...)
			}
		}
	}
	if s.isBoomEngine() && s.soloNet {
		out[engID] = append(out[engID], "-solonet")
	} else if s.existingOv != nil {
		for k, slice := range s.existingOv.AppendEngineArgs {
			if strings.EqualFold(k, engID) && containsFlag(slice, "-solonet") {
				if _, exists := out[engID]; !exists {
					out[engID] = []string{}
				}
			}
		}
	}
	return out
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
	if extraFields := strings.Fields(s.extraInput.Value()); len(extraFields) > 0 {
		args = append(args, extraFields...)
	}
	return args
}

func (s *launchOptionsState) includedFileNames() []string {
	var included []string
	optNorms := make(map[string]bool, len(s.optionalFiles))
	for _, opt := range s.optionalFiles {
		optNorms[catalog.NormalizeFilename(opt.spec.Name)] = true
		if opt.spec.Disabled && opt.enabled {
			included = append(included, opt.spec.Name)
		}
	}
	if s.existingOv != nil {
		for _, inc := range s.existingOv.IncludeFiles {
			if !optNorms[catalog.NormalizeFilename(inc)] {
				included = append(included, inc)
			}
		}
	}
	return included
}

func (s *launchOptionsState) excludedFileNames() []string {
	var excluded []string
	optNorms := make(map[string]bool, len(s.optionalFiles))
	for _, opt := range s.optionalFiles {
		optNorms[catalog.NormalizeFilename(opt.spec.Name)] = true
		if !opt.spec.Disabled && !opt.enabled {
			excluded = append(excluded, opt.spec.Name)
		}
	}
	if s.existingOv != nil {
		for _, ex := range s.existingOv.ExcludeFiles {
			if !optNorms[catalog.NormalizeFilename(ex)] {
				excluded = append(excluded, ex)
			}
		}
	}
	return excluded
}

func (s *launchOptionsState) buildLaunchPreset() catalog.Preset {
	p := clonePresetShallowSlices(s.base)

	if s.existingOv != nil {
		if s.existingOv.Set != nil {
			if s.existingOv.Set.IWAD != nil && *s.existingOv.Set.IWAD != "" {
				p.IWAD = *s.existingOv.Set.IWAD
			}
			if s.existingOv.Set.Files != nil {
				p.Files = append([]catalog.FileSpec(nil), (*s.existingOv.Set.Files)...)
			}
			if s.existingOv.Set.EngineArgs != nil {
				if p.EngineArgs == nil {
					p.EngineArgs = make(map[string][]string)
				}
				for k, v := range *s.existingOv.Set.EngineArgs {
					if len(v) == 0 {
						delete(p.EngineArgs, k)
					} else {
						p.EngineArgs[k] = append([]string(nil), v...)
					}
				}
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
	if clOverride, clearCL := s.selectedComplevelOverride(); !clearCL && clOverride != nil {
		cl := *clOverride
		p.Complevel = &cl
	}
	appendEngArgs := s.synthesizeAppendEngineArgs()
	for k, slice := range appendEngArgs {
		if len(slice) == 0 {
			continue
		}
		if p.EngineArgs == nil {
			p.EngineArgs = make(map[string][]string)
		}
		p.EngineArgs[k] = append(append([]string(nil), p.EngineArgs[k]...), slice...)
	}

	if included := s.includedFileNames(); len(included) > 0 {
		incSet := fileNormSet(included)
		for i := range p.Files {
			if p.Files[i].Disabled && matchesFileNorm(incSet, p.Files[i].Name) {
				p.Files[i].Disabled = false
			}
		}
	}

	if excluded := s.excludedFileNames(); len(excluded) > 0 {
		exSet := fileNormSet(excluded)
		var kept []catalog.FileSpec
		for _, f := range p.Files {
			if matchesFileNorm(exSet, f.Name) {
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
