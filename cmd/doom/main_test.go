package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractEngineArgs(t *testing.T) {
	tests := []struct {
		name       string
		subcommand string
		rawArgs    []string
		expected   []string
	}{
		{
			name:       "play with dash-dash arguments",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--", "-nomonsters", "-fast"},
			expected:   []string{"-nomonsters", "-fast"},
		},
		{
			name:       "play with --once and --dry-run flags",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--once", "--dry-run", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "play with known flags and values",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--engine", "dsda-doom", "--wads-dir", "/tmp/wads", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "launch skips preset name argument",
			subcommand: "launch",
			rawArgs:    []string{"doom", "launch", "Eviternity II", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "bare doom without extra args",
			subcommand: "play",
			rawArgs:    []string{"doom"},
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEngineArgs(tt.subcommand, tt.rawArgs)
			if len(result) == 0 && len(tt.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("extractEngineArgs(%q, %v) = %v, expected %v",
					tt.subcommand, tt.rawArgs, result, tt.expected)
			}
		})
	}
}

func TestWaitForEnter(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "newline immediately unblocks",
			input: "\n",
		},
		{
			name:  "text with newline",
			input: "hello\n",
		},
		{
			name:  "empty input / EOF unblocks without panic",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			waitForEnter(reader)
		})
	}
}
