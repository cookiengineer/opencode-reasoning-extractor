// Package tools ships a static registry of the OpenCode built-in tool/function
// schemas.
//
// OpenCode builds tool schemas at runtime from code and never persists them, so
// the extractor cannot read them from opencode.db. This registry is a static
// snapshot (generated from opencode v1.18.29) used to render the tools preamble
// when sessions are turned into training data.
package tools

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"opencode-reasoning-extractor/internal/model"
)

//go:embed tools.json
var registryJSON []byte

var registry []model.ToolSchema
var byName map[string]model.ToolSchema

func init() {
	if err := json.Unmarshal(registryJSON, &registry); err != nil {
		panic(fmt.Sprintf("tools: invalid tools.json: %v", err))
	}
	byName = make(map[string]model.ToolSchema, len(registry))
	for _, t := range registry {
		byName[t.Name] = t
	}
}

// All returns every known tool schema in registry order.
func All() []model.ToolSchema {
	return append([]model.ToolSchema(nil), registry...)
}

// For returns the schemas for the given names in first-seen order, skipping
// unknown names.
func For(names []string) []model.ToolSchema {
	var out []model.ToolSchema
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		if t, ok := byName[n]; ok {
			out = append(out, t)
			seen[n] = true
		}
	}
	return out
}
