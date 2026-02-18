package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shayne-snap/llmpole/data"
)

// CachePath returns the user cache file path for the model list (XDG-style: config dir/llmpole/models.json).
func CachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "llmpole", "models.json"), nil
}

func entryToModel(e *hfModelEntry) *LlmModel {
	return &LlmModel{
		Name:             e.Name,
		Provider:         e.Provider,
		ParameterCount:   e.ParameterCount,
		ParametersRaw:    e.ParametersRaw,
		MinRAMGB:         e.MinRAMGB,
		RecommendedRAMGB: e.RecommendedRAMGB,
		MinVRAMGB:        e.MinVRAMGB,
		Quantization:     e.Quantization,
		ContextLength:    e.ContextLength,
		UseCase:          e.UseCase,
		IsMoE:            e.IsMoE,
		NumExperts:       e.NumExperts,
		ActiveExperts:    e.ActiveExperts,
		ActiveParameters: e.ActiveParameters,
	}
}

// loadEmbedded returns models from the embedded JSON.
func loadEmbedded() ([]*LlmModel, error) {
	var entries []hfModelEntry
	if err := json.Unmarshal(data.HFModelsJSON, &entries); err != nil {
		return nil, err
	}
	models := make([]*LlmModel, 0, len(entries))
	for _, e := range entries {
		models = append(models, entryToModel(&e))
	}
	return models, nil
}

// mergeModels merges overlay into base by name (overlay overwrites or appends). Returns a new slice.
func mergeModels(base, overlay []*LlmModel) []*LlmModel {
	byName := make(map[string]*LlmModel)
	for _, m := range base {
		byName[m.Name] = m
	}
	for _, m := range overlay {
		byName[m.Name] = m
	}
	out := make([]*LlmModel, 0, len(byName))
	for _, m := range base {
		out = append(out, byName[m.Name])
	}
	seen := make(map[string]bool)
	for _, m := range base {
		seen[m.Name] = true
	}
	for _, m := range overlay {
		if !seen[m.Name] {
			out = append(out, m)
			seen[m.Name] = true
		}
	}
	return out
}

// NewDB loads model database from embedded JSON and optional user cache (merged by name).
func NewDB() (*ModelDatabase, error) {
	base, err := loadEmbedded()
	if err != nil {
		return nil, err
	}
	cachePath, err := CachePath()
	if err != nil {
		return &ModelDatabase{models: base}, nil
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return &ModelDatabase{models: base}, nil
	}
	var entries []hfModelEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Fprintf(os.Stderr, "llmpole: could not parse cache %s: %v (using embedded list)\n", cachePath, err)
		return &ModelDatabase{models: base}, nil
	}
	overlay := make([]*LlmModel, 0, len(entries))
	for i := range entries {
		overlay = append(overlay, entryToModel(&entries[i]))
	}
	models := mergeModels(base, overlay)
	return &ModelDatabase{models: models}, nil
}

// GetAllModels returns all models (slice of pointers for compatibility with FindModel).
func (db *ModelDatabase) GetAllModels() []*LlmModel {
	return db.models
}

// FindModel returns models whose name, provider, or parameter_count contains the query (case-insensitive).
func (db *ModelDatabase) FindModel(query string) []*LlmModel {
	q := strings.ToLower(query)
	var out []*LlmModel
	for _, m := range db.models {
		if strings.Contains(strings.ToLower(m.Name), q) ||
			strings.Contains(strings.ToLower(m.Provider), q) ||
			strings.Contains(strings.ToLower(m.ParameterCount), q) {
			out = append(out, m)
		}
	}
	return out
}

// WriteCacheFile writes raw JSON bytes to the user cache path (e.g. for update-list). Creates parent dir if needed.
func WriteCacheFile(body []byte) error {
	cachePath, err := CachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(cachePath, body, 0644)
}

// AppendModelToCache reads the current cache file (overlay-only), adds or replaces m by name, writes back.
func AppendModelToCache(m *LlmModel) error {
	cachePath, err := CachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var overlay []*LlmModel
	data, err := os.ReadFile(cachePath)
	if err == nil {
		if err := json.Unmarshal(data, &overlay); err != nil {
			overlay = nil
		}
	}
	if overlay == nil {
		overlay = make([]*LlmModel, 0)
	}
	found := false
	for i, existing := range overlay {
		if existing.Name == m.Name {
			overlay[i] = m
			found = true
			break
		}
	}
	if !found {
		overlay = append(overlay, m)
	}
	data, err = json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}
