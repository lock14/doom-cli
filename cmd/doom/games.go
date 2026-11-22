package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/catalog"
)

func exactArgsWithHint(n int, zeroArgsHint string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New(zeroArgsHint)
		}
		return cobra.ExactArgs(n)(cmd, args)
	}
}

type gameScope struct {
	allowed          map[string]bool
	includedOptional []string
	skippedOptional  []string
}

func splitCSVTokens(rawSlice []string) []string {
	var tokens []string
	for _, entry := range rawSlice {
		for _, part := range strings.Split(entry, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				tokens = append(tokens, trimmed)
			}
		}
	}
	return tokens
}

func resolveGameScope(
	cat *catalog.Catalog,
	idx *catalog.DirIndex,
	gamesFlag []string,
	gamesChanged bool,
) (gameScope, error) {
	optionalGames := cat.OptionalGames()
	validOptional := make(map[string]bool, len(optionalGames))
	for _, g := range optionalGames {
		validOptional[g] = true
	}

	allowed := map[string]bool{catalog.CoreGame: true}
	if !gamesChanged {
		for _, g := range cat.InstalledOptionalGames(idx) {
			allowed[g] = true
		}
	} else {
		tokens := splitCSVTokens(gamesFlag)
		if len(tokens) == 0 {
			return gameScope{}, fmt.Errorf(
				"flag --games cannot be empty (specify %s, 'all', or 'doom')",
				strings.Join(optionalGames, ", "),
			)
		}
		seenAll := false
		seenNone := false
		var explicitOpt []string
		for _, rawTok := range tokens {
			tok := strings.ToLower(rawTok)
			switch {
			case tok == "all":
				seenAll = true
			case tok == "none":
				seenNone = true
			case tok == catalog.CoreGame:
				// Core Doom is always included.
			case validOptional[tok]:
				explicitOpt = append(explicitOpt, tok)
			default:
				return gameScope{}, fmt.Errorf(
					"unknown game %q (valid optional games: %s, or 'all' / 'doom')",
					rawTok,
					strings.Join(optionalGames, ", "),
				)
			}
		}
		if seenAll && len(tokens) > 1 {
			return gameScope{}, errors.New("cannot combine 'all' with specific game names in --games")
		}
		if seenNone && len(tokens) > 1 {
			return gameScope{}, errors.New("cannot combine 'none' with other game names in --games")
		}
		if seenAll {
			for _, g := range optionalGames {
				allowed[g] = true
			}
		} else if !seenNone {
			for _, g := range explicitOpt {
				allowed[g] = true
			}
		}
	}

	var scope gameScope
	scope.allowed = allowed
	for _, g := range optionalGames {
		if allowed[g] {
			scope.includedOptional = append(scope.includedOptional, g)
		} else {
			scope.skippedOptional = append(scope.skippedOptional, g)
		}
	}
	return scope, nil
}

type wadSelectionMode int

const (
	wadsNone wadSelectionMode = iota
	wadsAll
	wadsSelected
)

type wadSelection struct {
	mode     wadSelectionMode
	selected []catalog.Preset
}

func resolveWadSelection(
	cat *catalog.Catalog,
	allowedGames map[string]bool,
	wadsFlag []string,
	wadsChanged bool,
) (wadSelection, error) {
	if !wadsChanged {
		return wadSelection{mode: wadsNone}, nil
	}
	tokens := splitCSVTokens(wadsFlag)
	if len(tokens) == 0 {
		return wadSelection{}, errors.New(
			"flag --wads cannot be empty (specify preset names, 'all', or 'none')",
		)
	}

	seenAll := false
	seenNone := false
	for _, tok := range tokens {
		switch strings.ToLower(tok) {
		case "all":
			seenAll = true
		case "none":
			seenNone = true
		}
	}
	if seenAll && len(tokens) > 1 {
		return wadSelection{}, errors.New("cannot combine 'all' with specific WAD names in --wads")
	}
	if seenNone && len(tokens) > 1 {
		return wadSelection{}, errors.New("cannot combine 'none' with specific WAD names in --wads")
	}
	if seenAll {
		return wadSelection{mode: wadsAll}, nil
	}
	if seenNone {
		return wadSelection{mode: wadsNone}, nil
	}

	seenIDs := make(map[string]bool)
	var selected []catalog.Preset
	for _, tok := range tokens {
		p, err := cat.FindExact(tok)
		if err != nil {
			return wadSelection{}, err
		}
		if !p.IsCustom() && !allowedGames[p.EffectiveGame()] {
			return wadSelection{}, fmt.Errorf(
				"preset %q belongs to optional game %q; also pass --games %s or --games all",
				p.Name,
				p.EffectiveGame(),
				p.EffectiveGame(),
			)
		}
		key := strings.ToLower(p.ID)
		if key == "" {
			key = strings.ToLower(p.Name)
		}
		if seenIDs[key] {
			continue
		}
		seenIDs[key] = true
		selected = append(selected, *p)
	}
	return wadSelection{mode: wadsSelected, selected: selected}, nil
}

func parsePresetTargetList(
	cat *catalog.Catalog,
	raw string,
	allowAll bool,
) (bool, []catalog.Preset, error) {
	tokens := splitCSVTokens([]string{raw})
	if len(tokens) == 0 {
		return false, nil, errors.New("preset name cannot be empty")
	}

	seenAll := false
	for _, tok := range tokens {
		if strings.EqualFold(tok, "all") {
			seenAll = true
		}
	}
	if seenAll {
		if !allowAll {
			return false, nil, errors.New(
				"'all' is not supported here; specify explicit WAD preset names",
			)
		}
		if len(tokens) > 1 {
			return false, nil, errors.New("cannot combine 'all' with specific WAD preset names")
		}
		return true, nil, nil
	}

	seenIDs := make(map[string]bool)
	var targets []catalog.Preset
	for _, tok := range tokens {
		p, exactErr := cat.FindExact(tok)
		if exactErr != nil {
			if strings.Contains(exactErr.Error(), "is ambiguous") {
				return false, nil, exactErr
			}
			p = cat.Find(tok)
			if p == nil {
				return false, nil, fmt.Errorf(
					"WAD preset %q not found; run 'doom wads list --all' to see available WADs",
					tok,
				)
			}
		}
		key := strings.ToLower(p.ID)
		if key == "" {
			key = strings.ToLower(p.Name)
		}
		if seenIDs[key] {
			continue
		}
		seenIDs[key] = true
		targets = append(targets, *p)
	}
	return false, targets, nil
}
