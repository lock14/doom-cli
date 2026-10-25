package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/tui"
	"github.com/lock14/doom-cli/internal/wad"
)

func newWadsCmd() *cobra.Command {
	wadsCmd := &cobra.Command{
		Use:   "wads",
		Short: "Browse, launch, fetch, and configure WAD presets",
		Args:  cobra.NoArgs,
		RunE:  runWadsTUI,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all available WAD presets with readiness and engine badges",
		Args:  cobra.NoArgs,
		RunE:  runWadsList,
	}

	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Display detailed metadata, file resolution, and launch preview for a WAD preset",
		Args:  cobra.ExactArgs(1),
		RunE:  runWadsShow,
	}

	var fetchForce bool
	fetchCmd := &cobra.Command{
		Use:   "fetch [name|all]",
		Short: "Extract commercial IWADs from Steam/GOG and download community megawads",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			out := cmdOut(cmd)

			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			if strings.EqualFold(target, "all") {
				searchRoots := steam.GetSearchRoots()
				if len(searchRoots) > 0 {
					if _, err := steam.DiscoverAndExtract(
						cat, searchRoots, paths.WadsDir, fetchForce, out, paths.WadsDirs...,
					); err != nil {
						return err
					}
				}
				dl := wad.NewDownloader(paths.WadRoots(), fetchForce, out)
				return dl.DownloadAll(cat)
			}

			p := findPreset(cat, target)
			if p == nil {
				return fmt.Errorf(
					"WAD preset %q not found. Run 'doom wads list' to see available WADs",
					target,
				)
			}

			idx := preset.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
			alreadyInstalled := strings.TrimSpace(p.IWAD) != "" && idx.IsPresetInstalled(*p)
			if !alreadyInstalled || fetchForce {
				if fetchForce || idx.NeedsSteamDiscovery(*p) {
					if _, err := steam.DiscoverAndExtract(
						cat, nil, paths.WadsDir, fetchForce, out, paths.WadsDirs...,
					); err != nil {
						return err
					}
				}
			}

			if len(p.Sources) == 0 {
				idx = preset.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
				if !idx.IsPresetInstalled(*p) {
					fmt.Fprintf(
						out,
						"%s is a commercial game and could not be found in local Steam or GOG libraries.\n",
						p.Name,
					)
				} else if alreadyInstalled && !fetchForce {
					fmt.Fprintf(out, "✓ %s already installed\n", p.Name)
				}
				return nil
			}

			dl := wad.NewDownloader(paths.WadRoots(), fetchForce, out)
			return dl.DownloadPreset(*p)
		},
	}
	fetchCmd.Flags().BoolVar(
		&fetchForce, "force", false, "Re-download and re-extract WADs even if already installed",
	)

	var scanForce bool
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan local Steam and GOG libraries to extract commercial IWADs and expansions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			_, err = steam.DiscoverAndExtract(
				cat, nil, paths.WadsDir, scanForce, cmdOut(cmd), paths.WadsDirs...,
			)
			return err
		},
	}
	scanCmd.Flags().BoolVar(
		&scanForce, "force", false, "Overwrite existing WAD files in destination directory",
	)

	var addFlags wadOptionFlags
	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom WAD or preset to your personal catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("preset name cannot be empty")
			}

			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			curated, err := preset.LoadCuratedCatalog()
			if err != nil {
				return err
			}

			var extraArgs []string
			if strings.TrimSpace(addFlags.args) != "" {
				extraArgs = strings.Fields(addFlags.args)
			}

			p := preset.Preset{
				ID:          addFlags.id,
				Name:        name,
				Engine:      addFlags.engine,
				IWAD:        addFlags.iwad,
				Files:       parseFileSpecs(addFlags.files),
				Args:        extraArgs,
				Description: addFlags.description,
			}

			outPath, err := preset.AddPreset(paths.CatalogDir, curated, cat, p)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmdOut(cmd), "✓ Successfully added custom preset %q to %s\n", p.Name, outPath)
			return nil
		},
	}
	bindWadCommonFlags(addCmd, &addFlags, "dsda-doom", "DOOM2.WAD")
	addCmd.Flags().StringVar(
		&addFlags.id, "id", "", "Explicit slug ID for this preset (defaults to slugified name)",
	)
	addCmd.Flags().StringVar(&addFlags.description, "description", "", "Short description of the mapset")

	var setFlags wadOptionFlags
	setCmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Configure per-preset overrides (engine, iwad, extra args, extra files, excluded files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}

			var req preset.OverridePatchRequest
			changed := false
			if cmd.Flags().Changed("engine") || setFlags.engine != "" {
				v := setFlags.engine
				req.Engine = &v
				changed = true
			}
			if cmd.Flags().Changed("iwad") || setFlags.iwad != "" {
				v := setFlags.iwad
				req.IWAD = &v
				changed = true
			}
			if cmd.Flags().Changed("args") || setFlags.args != "" {
				v := strings.Fields(setFlags.args)
				req.AppendArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("files") || len(setFlags.files) > 0 {
				specs := parseFileSpecs(setFlags.files)
				req.AppendFiles = &specs
				changed = true
			}
			if cmd.Flags().Changed("exclude-file") || len(setFlags.excludeFiles) > 0 {
				var excluded []string
				for _, f := range setFlags.excludeFiles {
					if trimmed := strings.TrimSpace(f); trimmed != "" {
						excluded = append(excluded, trimmed)
					}
				}
				req.ExcludeFiles = &excluded
				changed = true
			}
			if !changed {
				return fmt.Errorf(
					"no option flags provided (use --engine, --args, --iwad, --files, or --exclude-file, "+
						"or run 'doom wads reset %s')",
					name,
				)
			}

			targetKey := name
			if p := findPreset(cat, name); p != nil {
				targetKey = p.ID
			}
			outPath, target, err := preset.ConfigPreset(paths.CatalogDir, cat, targetKey, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmdOut(cmd), "✓ Configured launch options for %q in %s\n", target.Name, outPath)
			return nil
		},
	}
	bindWadCommonFlags(setCmd, &setFlags, "", "")
	setCmd.Flags().StringSliceVar(
		&setFlags.excludeFiles, "exclude-file", nil, "Optional or default PWAD files to exclude from this preset",
	)

	resetCmd := &cobra.Command{
		Use:   "reset <name>",
		Short: "Clear custom launch overrides for a WAD preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			targetKey := name
			if p := findPreset(cat, name); p != nil {
				targetKey = p.ID
			}
			out := cmdOut(cmd)
			target, removed, err := preset.ResetPresetOverride(paths.CatalogDir, cat, targetKey)
			if err != nil {
				return err
			}
			if removed {
				fmt.Fprintf(out, "✓ Reset custom launch options for %q\n", target.Name)
				return nil
			}
			fmt.Fprintf(out, "ℹ No custom launch options configured for %q\n", target.Name)
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom WAD preset from your personal catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			curated, err := preset.LoadCuratedCatalog()
			if err != nil {
				return err
			}

			targetKey := name
			if p := findPreset(cat, name); p != nil {
				targetKey = p.ID
			}
			removed, err := preset.RemovePreset(paths.CatalogDir, curated, cat, targetKey)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmdOut(cmd), "✓ Removed custom preset %q (%s)\n", removed.Name, removed.ID)
			return nil
		},
	}

	lintCmd := &cobra.Command{
		Use:   "lint [root]",
		Short: "Validate curated and user catalog entities, schema invariants, and README parity",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := ""
			if len(args) > 0 {
				root = args[0]
			} else if info, err := os.Stat(filepath.Join(".", "catalog")); err == nil && info.IsDir() {
				root = "."
			}
			paths := resolveAppPaths()
			diags, err := preset.LintCatalog(root, paths.CatalogDir)
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			for _, d := range diags {
				fmt.Fprintf(out, "[%s] %s\n", strings.ToUpper(d.Severity), d.String())
			}
			if preset.HasErrors(diags) {
				return fmt.Errorf("catalog lint failed with errors")
			}
			fmt.Fprintln(out, "✓ Catalog lint passed")
			return nil
		},
	}

	buildCmd := &cobra.Command{
		Use:    "build [root]",
		Short:  "Synchronize catalog/*.json into README.md",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if err := preset.SyncReadme(root); err != nil {
				return err
			}
			fmt.Fprintln(cmdOut(cmd), "✓ Successfully synchronized README.md with catalog/*.json")
			return nil
		},
	}

	wadsCmd.AddCommand(
		listCmd,
		showCmd,
		fetchCmd,
		scanCmd,
		addCmd,
		setCmd,
		resetCmd,
		removeCmd,
		lintCmd,
		buildCmd,
	)
	return wadsCmd
}

type wadOptionFlags struct {
	id           string
	engine       string
	iwad         string
	args         string
	description  string
	files        []string
	excludeFiles []string
}

func bindWadCommonFlags(cmd *cobra.Command, f *wadOptionFlags, defaultEngine, defaultIWAD string) {
	cmd.Flags().StringVarP(&f.engine, "engine", "e", defaultEngine, "Engine for this preset")
	cmd.Flags().StringVar(&f.iwad, "iwad", defaultIWAD, "Base IWAD required by this preset")
	cmd.Flags().StringSliceVar(
		&f.files, "files", nil, "Comma-separated list of PWAD and DeHackEd files",
	)
	cmd.Flags().StringVar(&f.args, "args", "", "Additional launch arguments")
}

func parseFileSpecs(raw []string) []preset.FileSpec {
	var specs []preset.FileSpec
	for _, f := range raw {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			specs = append(specs, preset.FileSpec{Name: trimmed})
		}
	}
	return specs
}

func runWadsTUI(cmd *cobra.Command, _ []string) error {
	paths, cfg, cat, err := loadAppState()
	if err != nil {
		return err
	}

	theme, nerdFonts, launcherOpts := buildLauncherOptions(
		paths, cfg, nil, tui.InitialModeWads,
	)

	selected, err := tui.RunInteractiveLauncherWithOptions(
		cat,
		paths.WadRoots(),
		theme,
		nerdFonts,
		launcherOpts,
	)
	if err != nil {
		return err
	}
	if selected == nil {
		return nil
	}

	if refreshedPaths, _, refreshedCat, reloadErr := loadAppState(); reloadErr == nil {
		paths = refreshedPaths
		cat = refreshedCat
	}

	var engOverride string
	if selected.EngineOverridden {
		engOverride = selected.Engine
	}
	out := cmdOut(cmd)
	errOut := cmdErr(cmd)
	opts := engine.LaunchOptions{
		EngineOverride: engOverride,
		WadRoots:       paths.WadRoots(),
		BinDir:         paths.BinDir,
		Out:            out,
		Catalog:        cat,
	}

	plan, err := engine.PrepareLaunch(*selected, opts)
	if err != nil {
		return err
	}

	return engine.Execute(plan, out, errOut)
}

func runWadsList(cmd *cobra.Command, _ []string) error {
	paths, _, cat, err := loadAppState()
	if err != nil {
		return err
	}
	dirIdx := preset.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
	out := cmdOut(cmd)

	fmt.Fprintln(out, "=== Available Doom Presets ===")
	for i, p := range cat.Presets {
		tag := fmt.Sprintf("[%-8s]", cat.EngineBadge(p.Engine))
		catLabel := p.Category
		if catLabel == "" && p.IsCustom() {
			catLabel = "Custom"
		}
		catTag := fmt.Sprintf("[%-18s]", catLabel)
		installStatus := "[✗ Missing]"
		if dirIdx.IsPresetInstalled(p) {
			installStatus = "[✓ Ready]  "
		}
		statusFlag := ""
		if p.IsCustom() {
			statusFlag = " (custom)"
		} else if p.Overridden {
			statusFlag = " (overridden)"
		}
		fmt.Fprintf(
			out, "%2d. %-30s %s %s %s%s - %s\n",
			i+1, p.Name, tag, catTag, installStatus, statusFlag, p.Description,
		)
	}
	return nil
}

func runWadsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	paths, _, cat, err := loadAppState()
	if err != nil {
		return err
	}
	p := findPreset(cat, name)
	if p == nil {
		return fmt.Errorf("preset %q not found", name)
	}
	out := cmdOut(cmd)

	fmt.Fprintf(out, "=== Preset: %s ===\n", p.Name)
	if p.IsCustom() {
		fmt.Fprintf(out, "  Type:          Custom WAD\n")
	}
	if p.Author != "" {
		fmt.Fprintf(out, "  Author:        %s\n", p.Author)
	}
	if p.ReleaseDate != "" {
		fmt.Fprintf(out, "  Released:      %s\n", p.ReleaseDate)
	}
	if p.Category != "" {
		fmt.Fprintf(out, "  Category:      %s\n", p.Category)
	}
	if p.Compatibility != "" {
		fmt.Fprintf(out, "  Compatibility: %s\n", p.Compatibility)
	}
	if p.Description != "" {
		fmt.Fprintf(out, "  Description:   %s\n", p.Description)
	}

	engStr := p.Engine
	if engCfg, ok := cat.FindEngine(p.Engine); ok {
		engStr = engCfg.FormatLabel()
	}
	engineOverridden := ""
	if p.EngineOverridden {
		engineOverridden = " [Overridden in user config]"
	}
	fmt.Fprintf(out, "  Engine:        %s%s\n", engStr, engineOverridden)

	idx := preset.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
	iwadPath, iwadFound := idx.ResolveFile(p.IWAD)
	iwadStatus := "[✗ Missing]"
	if iwadFound {
		iwadStatus = fmt.Sprintf("[✓ Found: %s]", iwadPath)
	}
	fmt.Fprintf(out, "  Base IWAD:     %s %s\n", p.IWAD, iwadStatus)

	if len(p.Files) > 0 {
		fmt.Fprintln(out, "  Files:")
		for _, f := range p.Files {
			fpath, fFound := idx.ResolveFileSpec(f)
			fStatus := "[✗ Missing]"
			if fFound {
				fStatus = fmt.Sprintf("[✓ Found: %s]", fpath)
			} else if f.Optional {
				fStatus = "[Optional]"
			}
			fmt.Fprintf(out, "    - %-20s %s\n", f.Name, fStatus)
		}
	}

	if len(p.Args) > 0 {
		fmt.Fprintf(out, "  Launch Args:   %s\n", strings.Join(p.Args, " "))
	}

	fmt.Fprintf(out, "  Launch Cmd:    %s\n", engine.FormatLaunchPreview(*p, cat, idx, nil))

	if len(p.Sources) > 0 {
		fmt.Fprintln(out, "  Sources:")
		for _, s := range p.Sources {
			if s.SHA256 != "" {
				fmt.Fprintf(out, "    - %s (sha256: %s)\n", s.URL, s.SHA256)
			} else {
				fmt.Fprintf(out, "    - %s\n", s.URL)
			}
		}
	}

	if len(p.Links) > 0 {
		fmt.Fprintln(out, "  Links:")
		for _, l := range p.Links {
			if l.Rel != "" {
				fmt.Fprintf(out, "    - [%s] %s\n", l.Rel, l.URL)
			} else {
				fmt.Fprintf(out, "    - %s\n", l.URL)
			}
		}
	}

	return nil
}

func findPreset(cat *preset.Catalog, query string) *preset.Preset {
	if p := cat.Find(query); p != nil {
		return p
	}
	normQuery := preset.NormalizeFilename(query)
	romanQuery := preset.NormalizePresetQuery(query)
	if normQuery == "" {
		return nil
	}
	for i := range cat.Presets {
		if strings.Contains(preset.NormalizeFilename(cat.Presets[i].Name), normQuery) ||
			strings.Contains(preset.NormalizePresetQuery(cat.Presets[i].Name), romanQuery) {
			return &cat.Presets[i]
		}
	}
	return nil
}
