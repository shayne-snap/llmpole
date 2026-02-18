// Package display handles CLI table and JSON output for system, models, and fit results.
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/olekukonko/tablewriter"
	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
	"github.com/shayne-snap/llmpole/internal/pole"
)

var (
	systemTpl *template.Template
	infoTpl   *template.Template
)

func init() {
	systemTpl = template.Must(template.New("system").Parse(
		`
=== System Specifications ===
CPU: {{.CPUName}} ({{.TotalCPUCores}} cores)
Total RAM: {{.TotalRAMGB}}
Available RAM: {{.AvailableRAMGB}}
Backend: {{.Backend}}
{{.GpuBlock}}

`))
	infoTpl = template.Must(template.New("info").Parse(
		`
=== {{.Name}} ===

Provider: {{.Provider}}
Parameters: {{.ParameterCount}}
Quantization: {{.Quantization}}
Best Quant: {{.BestQuant}}
Context Length: {{.ContextLength}} tokens
Use Case: {{.UseCase}}
Category: {{.Category}}

Score Breakdown:
  Overall Score: {{.Score}} / 100
  Quality: {{.Quality}}  Speed: {{.Speed}}  Fit: {{.Fit}}  Context: {{.ContextScore}}
  Estimated Speed: {{.EstimatedTPS}} tok/s

Resource Requirements:
{{.ResourceBlock}}
{{if .MoEBlock}}

MoE Architecture:
{{.MoEBlock}}
{{end}}

Fit Analysis:
  Status: {{.FitStatus}}
  Run Mode: {{.RunMode}}
  Memory Utilization: {{.UtilizationPct}} ({{.MemoryRequired}} / {{.MemoryAvailable}} GB)
{{if .NotesBlock}}

Notes:
{{.NotesBlock}}{{end}}

`))
}

// System prints system specs to out (table or JSON).
func System(out io.Writer, specs *hardware.SystemSpecs, useJSON bool) {
	if useJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]interface{}{
			"system": systemJSON(specs),
		})
		return
	}
	gpuBlock := buildSystemGpuBlock(specs)
	data := struct {
		CPUName, Backend, GpuBlock   string
		TotalCPUCores                int
		TotalRAMGB, AvailableRAMGB   string
	}{
		CPUName:        specs.CPUName,
		TotalCPUCores:  specs.TotalCPUCores,
		TotalRAMGB:     fmt.Sprintf("%.2f GB", specs.TotalRAMGB),
		AvailableRAMGB: fmt.Sprintf("%.2f GB", specs.AvailableRAMGB),
		Backend:        specs.Backend.String(),
		GpuBlock:       gpuBlock,
	}
	_ = systemTpl.Execute(out, data)
}

func buildSystemGpuBlock(specs *hardware.SystemSpecs) string {
	if len(specs.Gpus) == 0 {
		return "GPU: Not detected"
	}
	var lines []string
	for i, g := range specs.Gpus {
		prefix := "GPU: "
		if len(specs.Gpus) > 1 {
			prefix = fmt.Sprintf("GPU %d: ", i+1)
		}
		var line string
		if g.UnifiedMemory {
			v := 0.0
			if g.VRAMGB != nil {
				v = *g.VRAMGB
			}
			line = fmt.Sprintf("%s%s (unified memory, %.2f GB shared, %s)", prefix, g.Name, v, g.Backend.String())
		} else if g.VRAMGB != nil && *g.VRAMGB > 0 {
			if g.Count > 1 {
				line = fmt.Sprintf("%s%s x%d (%.2f GB VRAM total, %s)", prefix, g.Name, g.Count, *g.VRAMGB, g.Backend.String())
			} else {
				line = fmt.Sprintf("%s%s (%.2f GB VRAM, %s)", prefix, g.Name, *g.VRAMGB, g.Backend.String())
			}
		} else if g.VRAMGB != nil {
			line = fmt.Sprintf("%s%s (shared system memory, %s)", prefix, g.Name, g.Backend.String())
		} else {
			line = fmt.Sprintf("%s%s (VRAM unknown, %s)", prefix, g.Name, g.Backend.String())
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func systemJSON(specs *hardware.SystemSpecs) map[string]interface{} {
	gpus := make([]map[string]interface{}, 0, len(specs.Gpus))
	for _, g := range specs.Gpus {
		m := map[string]interface{}{
			"name":            g.Name,
			"backend":         g.Backend.String(),
			"count":           g.Count,
			"unified_memory":  g.UnifiedMemory,
		}
		if g.VRAMGB != nil {
			m["vram_gb"] = round2(*g.VRAMGB)
		}
		gpus = append(gpus, m)
	}
	m := map[string]interface{}{
		"total_ram_gb":     round2(specs.TotalRAMGB),
		"available_ram_gb": round2(specs.AvailableRAMGB),
		"cpu_cores":        specs.TotalCPUCores,
		"cpu_name":         specs.CPUName,
		"has_gpu":          specs.HasGPU,
		"gpu_count":        specs.GpuCount,
		"unified_memory":   specs.UnifiedMemory,
		"backend":          specs.Backend.String(),
		"gpus":             gpus,
	}
	if specs.GpuVRAMGB != nil {
		m["gpu_vram_gb"] = round2(*specs.GpuVRAMGB)
	}
	if specs.GpuName != nil {
		m["gpu_name"] = *specs.GpuName
	}
	return m
}

// List prints all models as table to out.
func List(out io.Writer, modelList []*models.LlmModel) {
	fmt.Fprintln(out, "\n=== Available LLM Models ===")
	fmt.Fprintf(out, "Total models: %d\n\n", len(modelList))
	tbl := tablewriter.NewWriter(out)
	tbl.Header("Status", "Model", "Provider", "Size", "Score", "tok/s", "Quant", "Mode", "Mem %", "Context")
	for _, m := range modelList {
		tbl.Append([]string{"--", m.Name, m.Provider, m.ParameterCount, "-", "-", m.Quantization, "-", "-", fmt.Sprintf("%dk", m.ContextLength/1000)})
	}
	_ = tbl.Render()
}

// Pole prints pole/fit analysis to out (table or JSON).
func Pole(out io.Writer, specs *hardware.SystemSpecs, fits []*pole.ModelFit, useJSON bool) {
	if useJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]interface{}{
			"system": systemJSON(specs),
			"models": fitsToJSON(fits),
		})
		return
	}
	if len(fits) == 0 {
		fmt.Fprintln(out, "\nNo compatible models found for your system.")
		return
	}
	fmt.Fprintln(out, "\n=== Pole Analysis ===")
	fmt.Fprintf(out, "Found %d compatible model(s)\n\n", len(fits))
	tbl := tablewriter.NewWriter(out)
	tbl.Header("Status", "Model", "Provider", "Size", "Score", "tok/s", "Quant", "Mode", "Mem %", "Context")
	for _, f := range fits {
		tbl.Append([]string{
			f.FitEmoji() + " " + f.FitText(),
			f.Model.Name,
			f.Model.Provider,
			f.Model.ParameterCount,
			fmt.Sprintf("%.0f", f.Score),
			fmt.Sprintf("%.1f", f.EstimatedTPS),
			f.BestQuant,
			f.RunModeText(),
			fmt.Sprintf("%.1f%%", f.UtilizationPct),
			fmt.Sprintf("%dk", f.Model.ContextLength/1000),
		})
	}
	_ = tbl.Render()
}

// Search prints search results table to out.
func Search(out io.Writer, results []*models.LlmModel, query string) {
	if len(results) == 0 {
		fmt.Fprintf(out, "\nNo models found matching '%s'\n", query)
		return
	}
	fmt.Fprintf(out, "\n=== Search Results for '%s' ===\n", query)
	fmt.Fprintf(out, "Found %d model(s)\n\n", len(results))
	tbl := tablewriter.NewWriter(out)
	tbl.Header("Status", "Model", "Provider", "Size", "Score", "tok/s", "Quant", "Mode", "Mem %", "Context")
	for _, m := range results {
		tbl.Append([]string{"--", m.Name, m.Provider, m.ParameterCount, "-", "-", m.Quantization, "-", "-", fmt.Sprintf("%dk", m.ContextLength/1000)})
	}
	_ = tbl.Render()
}

// infoData holds template data for Info view.
type infoData struct {
	Name, Provider, ParameterCount, Quantization, BestQuant, UseCase, Category string
	ContextLength                                                              string
	Score, Quality, Speed, Fit, ContextScore, EstimatedTPS                     string
	ResourceBlock, MoEBlock, FitStatus, RunMode, UtilizationPct                 string
	MemoryRequired, MemoryAvailable, NotesBlock                                string
}

// Info prints single model detail to out (table or JSON).
func Info(out io.Writer, specs *hardware.SystemSpecs, fit *pole.ModelFit, useJSON bool) {
	if useJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]interface{}{
			"system": systemJSON(specs),
			"models": fitsToJSON([]*pole.ModelFit{fit}),
		})
		return
	}
	m := fit.Model
	data := infoData{
		Name:           m.Name,
		Provider:       m.Provider,
		ParameterCount: m.ParameterCount,
		Quantization:   m.Quantization,
		BestQuant:      fit.BestQuant,
		ContextLength:  fmt.Sprintf("%d", m.ContextLength),
		UseCase:        m.UseCase,
		Category:       fit.UseCase.String(),
		Score:          fmt.Sprintf("%.1f", fit.Score),
		Quality:        fmt.Sprintf("%.0f", fit.ScoreComponents.Quality),
		Speed:          fmt.Sprintf("%.0f", fit.ScoreComponents.Speed),
		Fit:            fmt.Sprintf("%.0f", fit.ScoreComponents.Fit),
		ContextScore:   fmt.Sprintf("%.0f", fit.ScoreComponents.Context),
		EstimatedTPS:   fmt.Sprintf("%.1f", fit.EstimatedTPS),
		ResourceBlock:  buildInfoResourceBlock(m),
		FitStatus:      fit.FitEmoji() + " " + fit.FitText(),
		RunMode:        fit.RunModeText(),
		UtilizationPct: fmt.Sprintf("%.1f%%", fit.UtilizationPct),
		MemoryRequired: fmt.Sprintf("%.1f", fit.MemoryRequiredGB),
		MemoryAvailable: fmt.Sprintf("%.1f", fit.MemoryAvailableGB),
	}
	if m.IsMoE {
		data.MoEBlock = buildInfoMoEBlock(m, fit)
	}
	if len(fit.Notes) > 0 {
		data.NotesBlock = "  " + strings.Join(fit.Notes, "\n  ")
	}
	_ = infoTpl.Execute(out, data)
}

func buildInfoResourceBlock(m *models.LlmModel) string {
	var lines []string
	if m.MinVRAMGB != nil {
		lines = append(lines, fmt.Sprintf("  Min VRAM: %.1f GB", *m.MinVRAMGB))
	}
	lines = append(lines, fmt.Sprintf("  Min RAM: %.1f GB (CPU inference)", m.MinRAMGB))
	lines = append(lines, fmt.Sprintf("  Recommended RAM: %.1f GB", m.RecommendedRAMGB))
	return strings.Join(lines, "\n")
}

func buildInfoMoEBlock(m *models.LlmModel, fit *pole.ModelFit) string {
	var lines []string
	if m.NumExperts != nil && m.ActiveExperts != nil {
		lines = append(lines, fmt.Sprintf("  Experts: %d active / %d total per token", *m.ActiveExperts, *m.NumExperts))
	}
	if m.MoeActiveVRAMGB() != nil && m.MinVRAMGB != nil {
		lines = append(lines, fmt.Sprintf("  Active VRAM: %.1f GB (vs %.1f GB full model)", *m.MoeActiveVRAMGB(), *m.MinVRAMGB))
	}
	if fit.MoeOffloadedGB != nil {
		lines = append(lines, fmt.Sprintf("  Offloaded: %.1f GB inactive experts in RAM", *fit.MoeOffloadedGB))
	}
	return strings.Join(lines, "\n")
}

// Recommend prints recommendation list to out (table or JSON).
func Recommend(out io.Writer, specs *hardware.SystemSpecs, fits []*pole.ModelFit, useJSON bool) {
	if useJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]interface{}{
			"system": systemJSON(specs),
			"models": fitsToJSON(fits),
		})
		return
	}
	if len(fits) > 0 {
		System(out, specs, false)
	}
	Pole(out, specs, fits, false)
}

func fitsToJSON(fits []*pole.ModelFit) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(fits))
	for _, f := range fits {
		out = append(out, fitToJSON(f))
	}
	return out
}

func fitToJSON(f *pole.ModelFit) map[string]interface{} {
	m := f.Model
	obj := map[string]interface{}{
		"name":              m.Name,
		"provider":          m.Provider,
		"parameter_count":   m.ParameterCount,
		"params_b":          round2(m.ParamsB()),
		"context_length":    m.ContextLength,
		"use_case":          m.UseCase,
		"category":          f.UseCase.String(),
		"is_moe":            m.IsMoE,
		"fit_level":         f.FitText(),
		"run_mode":          f.RunModeText(),
		"score":             round1(f.Score),
		"score_components": map[string]interface{}{
			"quality": round1(f.ScoreComponents.Quality),
			"speed":   round1(f.ScoreComponents.Speed),
			"fit":     round1(f.ScoreComponents.Fit),
			"context": round1(f.ScoreComponents.Context),
		},
		"estimated_tps":      round1(f.EstimatedTPS),
		"best_quant":         f.BestQuant,
		"memory_required_gb": round2(f.MemoryRequiredGB),
		"memory_available_gb": round2(f.MemoryAvailableGB),
		"utilization_pct":    round1(f.UtilizationPct),
		"notes":              f.Notes,
	}
	return obj
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
