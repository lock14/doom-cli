package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/repolint"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/tui"
	"github.com/lock14/doom-cli/internal/wad"
)

func newWadsCmd() *cobra.Command {
	var wadsAll bool
	wadsCmd := &cobra.Command{
		Use:          "wads",
		Short:        "Browse, launch, fetch, and configure WAD presets",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWadsTUIWithOptions(cmd, args, wadsAll)
		},
	}
	wadsCmd.Flags().BoolVar(
		&wadsAll,
		"all",
		false,
		"Show all catalog presets including uninstalled WADs and optional non-Doom games",
	)

	var listAll bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List installed WAD presets (or pass --all for the full catalog)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWadsListWithOptions(cmd, listAll)
		},
	}
	listCmd.Flags().BoolVar(
		&listAll,
		"all",
		false,
		"List all catalog presets including uninstalled WADs and optional non-Doom games",
	)

	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Display detailed metadata, file resolution, and launch preview for a WAD preset",
		Args: exactArgsWithHint(
			1,
			"specify a WAD preset to inspect (e.g. 'doom wads show eviternity'); "+
				"run 'doom wads list --all' to see all presets",
		),
		RunE: runWadsShow,
	}

	var (
		fetchForce bool
		fetchFrom  []string
		fetchGames []string
	)
	fetchCmd := &cobra.Command{
		Use:   "fetch <name[,name...]|all>",
		Short: "Extract commercial IWADs from Steam/GOG and download community megawads",
		Args: exactArgsWithHint(
			1,
			"specify a WAD preset to fetch (e.g. 'doom wads fetch eviternity'), "+
				"or run 'doom wads fetch all' to download all curated megawads",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			ctx := cmd.Context()

			isAll, targets, err := parsePresetTargetList(cat, args[0], true)
			if err != nil {
				return err
			}
			scanRoots, err := resolveScanRoots(fetchFrom, cmd.Flags().Changed("from"))
			if err != nil {
				return err
			}

			if isAll {
				dirIdx := catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
				scope, scopeErr := resolveGameScope(cat, dirIdx, fetchGames, cmd.Flags().Changed("games"))
				if scopeErr != nil {
					return scopeErr
				}
				scopedCat := cat.FilterByGames(scope.allowed)
				if len(scanRoots) > 0 {
					if _, err := steam.DiscoverAndExtractContext(
						ctx, scopedCat, scanRoots, paths.WadsDir, fetchForce, out, paths.WadsDirs...,
					); err != nil {
						return err
					}
				}
				dl := wad.NewDownloader(paths.WadRoots(), fetchForce, out)
				dl.Ctx = ctx
				return dl.DownloadAll(scopedCat)
			}

			if cmd.Flags().Changed("games") {
				return errors.New("flag --games only applies to 'doom wads fetch all'")
			}

			idx := catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
			alreadyInstalled := make(map[string]bool, len(targets))
			anyNeedsSteam := fetchForce
			for _, t := range targets {
				alreadyInstalled[t.ID] = strings.TrimSpace(t.IWAD) != "" && idx.IsPresetInstalled(t)
				if idx.NeedsSteamDiscovery(t) {
					anyNeedsSteam = true
				}
			}

			var extracted int
			if anyNeedsSteam {
				var discErr error
				extracted, discErr = steam.DiscoverAndExtractContext(
					ctx, cat.ForPresets(targets...), scanRoots, paths.WadsDir, fetchForce, out, paths.WadsDirs...,
				)
				if discErr != nil {
					return discErr
				}
			}

			for _, p := range targets {
				if len(p.Sources) == 0 {
					idx = catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
					if !idx.IsPresetInstalled(p) {
						if !p.IsCustom() {
							fmt.Fprintf(
								out,
								"%s is a commercial game and could not be found in local Steam or GOG libraries.\n",
								p.Name,
							)
						} else {
							fmt.Fprintf(
								out,
								"%s has no download sources configured and required WAD files are missing.\n",
								p.Name,
							)
						}
						return fmt.Errorf("missing required files for %s", p.Name)
					}
					if alreadyInstalled[p.ID] && !fetchForce && extracted == 0 {
						fmt.Fprintf(out, "✓ %s already installed\n", p.Name)
					}
					continue
				}

				dl := wad.NewDownloader(paths.WadRoots(), fetchForce, out)
				dl.Ctx = ctx
				if err := dl.DownloadPreset(p); err != nil {
					return err
				}
				idx = catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
				if !idx.IsPresetInstalled(p) {
					return fmt.Errorf(
						"missing required files for %s (ensure base IWAD %s is installed)",
						p.Name,
						p.IWAD,
					)
				}
			}
			return nil
		},
	}
	fetchCmd.Flags().BoolVar(
		&fetchForce, "force", false, "Re-download and re-extract WADs even if already installed",
	)
	fetchCmd.Flags().StringSliceVar(
		&fetchFrom,
		"from",
		nil,
		"Custom game or Steam/GOG directories to scan for commercial IWADs",
	)
	fetchCmd.Flags().StringSliceVar(
		&fetchGames,
		"games",
		nil,
		"Include optional non-Doom games when fetching all (heretic, hexen, strife, all, or doom)",
	)

	uninstallCmd := &cobra.Command{
		Use:   "uninstall <name[,name...]>",
		Short: "Remove downloaded PWAD, PK3, DeHackEd, and README files for a WAD preset",
		Args: exactArgsWithHint(
			1,
			"specify one or more WAD presets to uninstall (e.g. 'doom wads uninstall sunder'); "+
				"run 'doom wads list' to see installed presets",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			_, targets, err := parsePresetTargetList(cat, args[0], false)
			if err != nil {
				return err
			}
			for _, p := range targets {
				if _, err := wad.UninstallPreset(p, cat, paths.WadsDir, paths.WadsDirs, out); err != nil {
					return err
				}
			}
			return nil
		},
	}

	var (
		scanForce bool
		scanFrom  []string
		scanGames []string
	)
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan local Steam and GOG libraries to extract commercial IWADs and expansions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			dirIdx := catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
			scope, err := resolveGameScope(cat, dirIdx, scanGames, cmd.Flags().Changed("games"))
			if err != nil {
				return err
			}
			scanRoots, err := resolveScanRoots(scanFrom, cmd.Flags().Changed("from"))
			if err != nil {
				return err
			}
			scopedCat := cat.FilterByGames(scope.allowed)
			_, err = steam.DiscoverAndExtractContext(
				cmd.Context(), scopedCat, scanRoots, paths.WadsDir, scanForce, out, paths.WadsDirs...,
			)
			if err != nil {
				return err
			}
			if len(scope.skippedOptional) > 0 {
				fmt.Fprintf(
					out,
					"Optional non-Doom games skipped: %s (pass --games <list|all> to include).\n",
					strings.Join(scope.skippedOptional, ", "),
				)
			}
			return nil
		},
	}
	scanCmd.Flags().BoolVar(
		&scanForce, "force", false, "Overwrite existing WAD files in destination directory",
	)
	scanCmd.Flags().StringSliceVar(
		&scanFrom,
		"from",
		nil,
		"Custom game or Steam/GOG directories to scan instead of default locations",
	)
	scanCmd.Flags().StringSliceVar(
		&scanGames,
		"games",
		nil,
		"Include optional non-Doom games (heretic, hexen, strife, all, or doom)",
	)

	var addFlags wadOptionFlags
	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom WAD or preset to your personal catalog",
		Args: exactArgsWithHint(
			1,
			"specify a name for the custom WAD preset (e.g. 'doom wads add \"My Map\" --files mymap.wad')",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("preset name cannot be empty")
			}

			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}
			curated, err := catalog.LoadCurated()
			if err != nil {
				return err
			}

			var extraArgs []string
			if strings.TrimSpace(addFlags.args) != "" {
				extraArgs, err = splitCommandArgs(addFlags.args)
				if err != nil {
					return err
				}
			}
			var complevel *int
			if strings.TrimSpace(addFlags.complevel) != "" {
				v, clErr := strconv.Atoi(strings.TrimSpace(addFlags.complevel))
				if clErr != nil {
					return fmt.Errorf("invalid --complevel %q: %w", addFlags.complevel, clErr)
				}
				if clErr := catalog.ValidateComplevel(v); clErr != nil {
					return clErr
				}
				complevel = &v
			}
			var engineArgs map[string][]string
			if len(addFlags.engineArgs) > 0 {
				engineArgs, err = parseEngineArgsFlags(addFlags.engineArgs)
				if err != nil {
					return err
				}
			}

			p := catalog.Preset{
				ID:          addFlags.id,
				Name:        name,
				Engine:      addFlags.engine,
				IWAD:        addFlags.iwad,
				Files:       parseFileSpecs(addFlags.files),
				Complevel:   complevel,
				EngineArgs:  engineArgs,
				Args:        extraArgs,
				Description: addFlags.description,
			}

			outPath, err := catalog.AddPreset(paths.CatalogDir, curated, cat, p)
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
		Args: exactArgsWithHint(
			1,
			"specify a WAD preset to configure (e.g. 'doom wads set sunlust -e uzdoom --args \"-skill 4\"')",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}

			var req catalog.OverridePatchRequest
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
			if cmd.Flags().Changed("complevel") || setFlags.complevel != "" {
				rawCL := strings.ToLower(strings.TrimSpace(setFlags.complevel))
				switch rawCL {
				case "", "default":
					req.ClearComplevel = true
				case "none", "-1":
					unset := catalog.ComplevelUnset
					req.Complevel = &unset
				default:
					v, clErr := strconv.Atoi(rawCL)
					if clErr != nil {
						return fmt.Errorf(
							"invalid --complevel %q (expected 0..17, 21, default, or none)",
							setFlags.complevel,
						)
					}
					if clErr := catalog.ValidateComplevel(v); clErr != nil {
						return clErr
					}
					req.Complevel = &v
				}
				changed = true
			}
			if cmd.Flags().Changed("engine-args") || len(setFlags.engineArgs) > 0 {
				parsedEngArgs, parseErr := parseEngineArgsFlags(setFlags.engineArgs)
				if parseErr != nil {
					return parseErr
				}
				req.SetEngineArgs = &parsedEngArgs
				changed = true
			}
			if cmd.Flags().Changed("args") || setFlags.args != "" {
				v, splitErr := splitCommandArgs(setFlags.args)
				if splitErr != nil {
					return splitErr
				}
				req.AppendArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("files") || len(setFlags.files) > 0 {
				specs := parseFileSpecs(setFlags.files)
				req.AppendFiles = &specs
				changed = true
			}
			if cmd.Flags().Changed("include-file") || len(setFlags.includeFiles) > 0 {
				var included []string
				for _, f := range setFlags.includeFiles {
					if trimmed := strings.TrimSpace(f); trimmed != "" {
						included = append(included, trimmed)
					}
				}
				req.IncludeFiles = &included
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
					"no option flags provided (use --engine, --complevel, --engine-args, --args, "+
						"--iwad, --files, --include-file, or --exclude-file, or run 'doom wads reset %s')",
					name,
				)
			}

			outPath, target, err := catalog.ConfigPreset(paths.CatalogDir, cat, name, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmdOut(cmd), "✓ Configured launch options for %q in %s\n", target.Name, outPath)
			return nil
		},
	}
	bindWadCommonFlags(setCmd, &setFlags, "", "")
	setCmd.Flags().StringSliceVar(
		&setFlags.includeFiles,
		"include-file",
		nil,
		"Disabled or excluded optional PWAD files to enable for this preset",
	)
	setCmd.Flags().StringSliceVar(
		&setFlags.excludeFiles, "exclude-file", nil, "Optional or default PWAD files to exclude from this preset",
	)

	resetCmd := &cobra.Command{
		Use:   "reset <name>",
		Short: "Clear custom launch overrides for a WAD preset",
		Args:  exactArgsWithHint(1, "specify a WAD preset to reset (e.g. 'doom wads reset sunlust')"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			target, removed, err := catalog.ResetPresetOverride(paths.CatalogDir, cat, name)
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
		Args: exactArgsWithHint(
			1,
			"specify a custom WAD preset to remove (e.g. 'doom wads remove my-map'), "+
				"or use 'doom wads uninstall <name>' to delete downloaded WAD files",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
			if err != nil {
				return err
			}
			curated, err := catalog.LoadCurated()
			if err != nil {
				return err
			}

			removed, err := catalog.RemovePreset(paths.CatalogDir, curated, cat, name)
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
			diags, err := repolint.LintCatalog(root, paths.CatalogDir)
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			for _, d := range diags {
				fmt.Fprintf(out, "[%s] %s\n", strings.ToUpper(d.Severity), d.String())
			}
			if repolint.HasErrors(diags) {
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
			if err := repolint.SyncReadme(root); err != nil {
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
		uninstallCmd,
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
	complevel    string
	args         string
	description  string
	files        []string
	engineArgs   []string
	includeFiles []string
	excludeFiles []string
}

func bindWadCommonFlags(cmd *cobra.Command, f *wadOptionFlags, defaultEngine, defaultIWAD string) {
	cmd.Flags().StringVarP(&f.engine, "engine", "e", defaultEngine, "Engine for this preset")
	cmd.Flags().StringVar(&f.iwad, "iwad", defaultIWAD, "Base IWAD required by this preset")
	cmd.Flags().StringSliceVar(
		&f.files, "files", nil, "Comma-separated list of PWAD and DeHackEd files",
	)
	cmd.Flags().StringVar(
		&f.complevel, "complevel", "", "Boom/MBF compatibility level (e.g. 2, 3, 4, 9, 11, 21)",
	)
	cmd.Flags().StringArrayVar(
		&f.engineArgs,
		"engine-args",
		nil,
		"Per-engine launch arguments as <engine>=<flags> (repeatable)",
	)
	cmd.Flags().StringVar(&f.args, "args", "", "Universal launch arguments shared across engines")
}

func runWadsTUI(cmd *cobra.Command, args []string) error {
	return runWadsTUIWithOptions(cmd, args, false)
}

func runWadsTUIWithOptions(cmd *cobra.Command, _ []string, showAll bool) error {
	errOut := cmdErr(cmd)
	paths, cfg, cat, err := loadAppStateWithErr(errOut)
	if err != nil {
		return err
	}

	theme, nerdFonts, launcherOpts := buildLauncherOptions(
		paths, cfg, nil, tui.InitialModeWads,
	)
	launcherOpts.ShowAllPresets = showAll

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

	if refreshedPaths, _, refreshedCat, reloadErr := loadAppStateWithErr(errOut); reloadErr == nil {
		paths = refreshedPaths
		cat = refreshedCat
	}

	var engOverride string
	if selected.EngineOverridden {
		engOverride = selected.Engine
	}
	out := cmdOut(cmd)
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

func runWadsListWithOptions(cmd *cobra.Command, showAll bool) error {
	paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
	if err != nil {
		return err
	}
	dirIdx := catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
	out := cmdOut(cmd)

	var presets []catalog.Preset
	hiddenCount := 0
	if showAll {
		fmt.Fprintln(out, "=== Available Doom Presets (All) ===")
		presets = cat.Presets
	} else {
		fmt.Fprintln(out, "=== Installed Doom Presets ===")
		presets, hiddenCount = cat.LibraryPresets(dirIdx)
		if len(presets) == 0 {
			fmt.Fprintf(out, "No installed presets found (%d presets available in catalog).\n", len(cat.Presets))
			fmt.Fprintln(
				out,
				"Run 'doom setup' to install core assets, or 'doom wads list --all' to browse all presets.",
			)
			return nil
		}
	}

	for i, p := range presets {
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
		var metaParts []string
		if p.Genre != "" {
			metaParts = append(metaParts, p.Genre)
		}
		if p.Difficulty != "" {
			metaParts = append(metaParts, p.Difficulty)
		}
		if p.Quality != "" {
			metaParts = append(metaParts, p.Quality)
		}
		metaSuffix := p.Description
		if len(metaParts) > 0 && metaSuffix != "" {
			metaSuffix = fmt.Sprintf("%s (%s)", metaSuffix, strings.Join(metaParts, " | "))
		} else if len(metaParts) > 0 {
			metaSuffix = strings.Join(metaParts, " | ")
		}
		fmt.Fprintf(
			out, "%2d. %-30s %s %s %s%s - %s\n",
			i+1, p.Name, tag, catTag, installStatus, statusFlag, metaSuffix,
		)
	}

	if !showAll && hiddenCount > 0 {
		noun := "presets"
		if hiddenCount == 1 {
			noun = "preset"
		}
		fmt.Fprintf(
			out,
			"\n%d uninstalled %s hidden. Run 'doom wads list --all' to show all.\n",
			hiddenCount,
			noun,
		)
	}
	return nil
}

func runWadsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	paths, _, cat, err := loadAppStateWithErr(cmdErr(cmd))
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
	gameLabel := catalog.GameDisplayName(p.EffectiveGame())
	if !p.IsCoreGame() {
		gameLabel += " (optional)"
	}
	fmt.Fprintf(out, "  Game:          %s\n", gameLabel)
	if p.Author != "" {
		fmt.Fprintf(out, "  Author:        %s\n", p.Author)
	}
	if p.ReleaseDate != "" {
		fmt.Fprintf(out, "  Released:      %s\n", p.ReleaseDate)
	}
	if p.Category != "" {
		fmt.Fprintf(out, "  Category:      %s\n", p.Category)
	}
	if p.Genre != "" {
		fmt.Fprintf(out, "  Genre:         %s\n", p.Genre)
	}
	if p.Difficulty != "" {
		fmt.Fprintf(out, "  Difficulty:    %s\n", p.Difficulty)
	}
	if p.Quality != "" {
		fmt.Fprintf(out, "  Quality:       %s\n", p.Quality)
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

	idx := catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
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
			if f.Disabled {
				fStatus = "[Disabled]"
			} else if fFound {
				fStatus = fmt.Sprintf("[✓ Found: %s]", fpath)
			} else if f.Optional {
				fStatus = "[Optional]"
			}
			fmt.Fprintf(out, "    - %-20s %s\n", f.Name, fStatus)
		}
	}

	if p.Complevel != nil {
		fmt.Fprintf(out, "  Complevel:     %s\n", catalog.ComplevelLabel(*p.Complevel))
	}
	if len(p.EngineArgs) > 0 {
		engKeys := make([]string, 0, len(p.EngineArgs))
		for k := range p.EngineArgs {
			engKeys = append(engKeys, k)
		}
		sort.Strings(engKeys)
		fmt.Fprintln(out, "  Engine Args:")
		for _, k := range engKeys {
			if len(p.EngineArgs[k]) > 0 {
				fmt.Fprintf(out, "    - %-12s %s\n", k+":", strings.Join(p.EngineArgs[k], " "))
			}
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

func findPreset(cat *catalog.Catalog, query string) *catalog.Preset {
	return cat.Find(query)
}
