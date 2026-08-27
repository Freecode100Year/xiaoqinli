package main

import (
	"reflect"
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name: "kv flags",
			args: []string{"--file", "app.xql.json", "--target", "go", "--out", "out.go"},
			expected: map[string]string{
				"file":   "app.xql.json",
				"target": "go",
				"out":    "out.go",
			},
		},
		{
			name: "equal kv flags",
			args: []string{"--file=app.xql.json", "--target=rust"},
			expected: map[string]string{
				"file":   "app.xql.json",
				"target": "rust",
			},
		},
		{
			name: "boolean flag should not swallow subsequent arg",
			args: []string{"--no-strict-caps", "--file", "example.json"},
			expected: map[string]string{
				"no-strict-caps": "true",
				"file":           "example.json",
			},
		},
		{
			name: "boolean flag at the end",
			args: []string{"--file", "example.json", "--strict-caps"},
			expected: map[string]string{
				"file":        "example.json",
				"strict-caps": "true",
			},
		},
		{
			name: "unknown boolean flag fallback",
			args: []string{"--unknown-flag"},
			expected: map[string]string{
				"unknown-flag": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlags(tt.args)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseFlags(%v) = %v, want %v", tt.args, got, tt.expected)
			}
		})
	}
}
