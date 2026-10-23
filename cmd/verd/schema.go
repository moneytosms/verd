package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/moneytosms/verd/internal/config"
)

// schemaCmd prints a JSON Schema for config.toml.
func schemaCmd(out io.Writer) error {
	schema := buildSchema()
	b, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n", b)
	return nil
}

// buildSchema constructs a JSON Schema for verd's config.toml.
func buildSchema() map[string]any {
	// Top-level properties
	props := make(map[string]any)

	// every key `verd config set` accepts, with its type, enum values and default
	def := config.Default()
	for _, k := range config.KeyInfos() {
		props[k.Name] = keyProp(k, def)
	}
	props["lang"] = langSchema()
	props["keys"] = keysSchema()
	props["themes"] = themesSchema()

	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"title":                "verd config.toml",
		"description":          "Configuration file for verd, a terminal Codeforces companion",
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
}

// keyProp is the schema for one settable top-level key.
func keyProp(k config.KeyInfo, def config.Config) map[string]any {
	p := map[string]any{"type": k.Type}
	if k.Type == "array" {
		p["items"] = map[string]any{"type": "string"}
	}
	if len(k.Options) > 0 {
		p["enum"] = k.Options
	}
	if v, ok := def.Get(k.Name); ok && v != "" {
		if d, ok := schemaDefault(k.Type, v); ok {
			p["default"] = d
		}
	}
	if k.Type == "number" || k.Type == "integer" {
		p["minimum"] = 0
	}
	return p
}

// schemaDefault converts a default config value to its JSON type. ok is false when it does not parse as a number.
func schemaDefault(kind, v string) (any, bool) {
	switch kind {
	case "boolean":
		return v == "true", true
	case "number", "integer":
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	case "array":
		return strings.Split(v, ","), true
	default:
		return v, true
	}
}

// langSchema is the [lang.<name>] table.
func langSchema() map[string]any {
	langProps := make(map[string]any)
	langProps["ext"] = map[string]any{
		"type":        "string",
		"description": "File extension without dot",
	}
	langProps["compile"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Compile command, placeholders: {src}, {bin}, {dir}",
	}
	langProps["run"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Run command, placeholders: {src}, {bin}, {dir}",
	}
	langProps["cf_compiler_id"] = map[string]any{
		"type":        "integer",
		"description": "Codeforces compiler ID",
	}
	for _, k := range []string{"time_multiplier", "float_eps", "memory_multiplier"} {
		langProps[k] = map[string]any{"type": "number", "minimum": 0}
	}
	langProps["template"] = map[string]any{"type": "string", "description": "Path to a custom Solution Template for this language"}

	return map[string]any{
		"type": "object",
		"patternProperties": map[string]any{
			"^[a-zA-Z0-9_]+$": map[string]any{
				"type":                 "object",
				"properties":           langProps,
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
}

// keysSchema is the [keys.<context>] table: each context maps action names to a key or keys.
func keysSchema() map[string]any {
	actionValues := map[string]any{
		"oneOf": []map[string]any{
			{"type": "string"},
			{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
	keysPatternProps := make(map[string]any)
	for _, ctx := range []string{"common", "tabs", "app", "problems", "contests", "contest", "stats", "picker", "settings", "problem", "editorial", "testmgr", "submission", "help", "editor"} {
		keysPatternProps[ctx] = map[string]any{
			"type":                 "object",
			"additionalProperties": actionValues,
		}
	}
	return map[string]any{
		"type": "object",
		"patternProperties": map[string]any{
			"^[a-zA-Z_]+$": map[string]any{
				"type":                 "object",
				"additionalProperties": actionValues,
			},
		},
		"additionalProperties": false,
	}
}

// themesSchema is the [themes.<name>] table, with [themes.<name>.dark|light] palettes.
func themesSchema() map[string]any {
	themeColorPattern := "^(#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|ansi[0-9]{1,3})$"

	// Color palette keys: accent, accent2, dim, good, bad, warn, surface, bar, on_accent
	paletteKeys := []string{"accent", "accent2", "dim", "good", "bad", "warn", "surface", "bar", "on_accent"}
	colorPaletteProps := make(map[string]any)
	for _, c := range paletteKeys {
		colorPaletteProps[c] = map[string]any{
			"type":    "string",
			"pattern": themeColorPattern,
		}
	}

	themeProps := make(map[string]any)
	themeProps["base"] = map[string]any{
		"type":        "string",
		"description": "Inherit from a built-in theme",
	}
	themeProps["glamour_dark"] = map[string]any{
		"type":        "string",
		"description": "Markdown style on dark background",
	}
	themeProps["glamour_light"] = map[string]any{
		"type":        "string",
		"description": "Markdown style on light background",
	}
	themeProps["dark"] = map[string]any{
		"type":                 "object",
		"properties":           colorPaletteProps,
		"additionalProperties": false,
	}
	themeProps["light"] = map[string]any{
		"type":                 "object",
		"properties":           colorPaletteProps,
		"additionalProperties": false,
	}

	return map[string]any{
		"type": "object",
		"patternProperties": map[string]any{
			"^[a-zA-Z0-9_-]+$": map[string]any{
				"type":                 "object",
				"properties":           themeProps,
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
}
