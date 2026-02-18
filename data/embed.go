// Package data holds embedded assets (e.g. the default model list) at repo root data/ for clarity.
package data

import _ "embed"

//go:embed hf_models.json
var HFModelsJSON []byte
