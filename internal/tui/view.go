package tui

import (
	"fmt"
	"strings"

	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/pole"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleBorder  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleNormal  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleMagenta = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleStatus  = lipgloss.NewStyle().Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")).Bold(true)
)

// Render returns the full TUI view for the app.
func Render(app *App) string {
	w := app.Width
	if w <= 0 {
		w = 80
	}
	h := app.Height
	if h <= 0 {
		h = 24
	}

	sysBar := renderSystemBar(app)
	searchBar := renderSearchAndFilters(app)
	mainArea := 3 + 3
	statusHeight := 1
	mainHeight := h - mainArea - statusHeight
	if mainHeight < 5 {
		mainHeight = 5
	}

	var main string
	if app.ShowDetail {
		main = renderDetail(app, w, mainHeight)
	} else {
		main = renderTable(app, w, mainHeight)
	}
	statusBar := renderStatusBar(app)

	body := lipgloss.JoinVertical(lipgloss.Left, sysBar, searchBar, main, statusBar)
	if app.InputMode == InputModeProviderPopup {
		popup := renderProviderPopup(app, w, h)
		bodyLines := strings.Split(body, "\n")
		popupLines := strings.Split(popup, "\n")
		if len(popupLines) > 0 && len(bodyLines) >= len(popupLines) {
			startRow := (len(bodyLines) - len(popupLines)) / 2
			popupW := 0
			for _, l := range popupLines {
				if len(l) > popupW {
					popupW = len(l)
				}
			}
			padLeft := (w - popupW) / 2
			if padLeft < 0 {
				padLeft = 0
			}
			for i, pl := range popupLines {
				idx := startRow + i
				if idx < len(bodyLines) {
					bodyLines[idx] = strings.Repeat(" ", padLeft) + pl
				}
			}
			body = strings.Join(bodyLines, "\n")
		}
	}
	return body
}

func renderSystemBar(app *App) string {
	specs := app.Specs
	gpuInfo := "GPU: none (" + specs.Backend.String() + ")"
	if len(specs.Gpus) > 0 {
		primary := &specs.Gpus[0]
		backend := primary.Backend.String()
		vram := 0.0
		if primary.VRAMGB != nil {
			vram = *primary.VRAMGB
		}
		var primaryStr string
		if primary.UnifiedMemory {
			primaryStr = fmt.Sprintf("%s (%.1f GB shared, %s)", primary.Name, vram, backend)
		} else {
			if vram > 0 {
				if primary.Count > 1 {
					primaryStr = fmt.Sprintf("%s x%d (%.1f GB, %s)", primary.Name, primary.Count, vram, backend)
				} else {
					primaryStr = fmt.Sprintf("%s (%.1f GB, %s)", primary.Name, vram, backend)
				}
			} else {
				primaryStr = fmt.Sprintf("%s (shared, %s)", primary.Name, backend)
			}
		}
		extra := len(specs.Gpus) - 1
		if extra > 0 {
			gpuInfo = "GPU: " + primaryStr + " +" + fmt.Sprintf("%d", extra) + " more"
		} else {
			gpuInfo = "GPU: " + primaryStr
		}
	}
	wslSuffix := ""
	if hardware.IsRunningInWSL() {
		wslSuffix = " (WSL)"
	}
	ramStr := fmt.Sprintf("%.1f GB avail / %.1f GB total%s", specs.AvailableRAMGB, specs.TotalRAMGB, wslSuffix)
	line := styleDim.Render(" CPU: ") +
		styleNormal.Render(fmt.Sprintf("%s (%d cores)", specs.CPUName, specs.TotalCPUCores)) +
		styleDim.Render("  │  ") +
		styleDim.Render("RAM: ") +
		styleCyan.Render(ramStr) +
		styleDim.Render("  │  ") +
		styleYellow.Render(gpuInfo)
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	title := styleTitle.Render(" llmpole ")
	return block.Render(title + " " + line)
}

func renderSearchAndFilters(app *App) string {
	searchTitle := " Search "
	if app.InputMode == InputModeSearch {
		searchTitle = styleYellow.Render(searchTitle)
	} else {
		searchTitle = styleDim.Render(searchTitle)
	}
	searchContent := "Press / to search..."
	if app.InputMode == InputModeSearch || app.SearchQuery != "" {
		searchContent = styleNormal.Render(app.SearchQuery)
	} else {
		searchContent = styleDim.Render(searchContent)
	}
	searchBlock := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	searchBox := searchBlock.Render(searchTitle + " " + searchContent)

	activeCount := 0
	for _, s := range app.SelectedProviders {
		if s {
			activeCount++
		}
	}
	totalCount := len(app.Providers)
	providerText := "All"
	if activeCount != totalCount {
		providerText = fmt.Sprintf("%d/%d", activeCount, totalCount)
	}
	providerStyle := styleGreen
	if activeCount == 0 {
		providerStyle = styleRed
	} else if activeCount < totalCount {
		providerStyle = styleYellow
	}
	providerBlock := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(22)
	providerBox := providerBlock.Render(styleDim.Render(" Providers (p) ") + " " + providerStyle.Render(providerText))

	fitLabel := app.FitFilter.Label()
	fitStyle := styleNormal
	switch app.FitFilter {
	case FitFilterRunnable, FitFilterPerfect:
		fitStyle = styleGreen
	case FitFilterGood:
		fitStyle = styleYellow
	case FitFilterMarginal:
		fitStyle = styleMagenta
	}
	fitBlock := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(18)
	fitBox := fitBlock.Render(styleDim.Render(" Fit [f] ") + " " + fitStyle.Render(fitLabel))

	return lipgloss.JoinHorizontal(lipgloss.Top, searchBox, " ", providerBox, " ", fitBox)
}

func fitColor(level pole.FitLevel) lipgloss.Style {
	switch level {
	case pole.FitPerfect:
		return styleGreen
	case pole.FitGood:
		return styleYellow
	case pole.FitMarginal:
		return styleMagenta
	case pole.FitTooTight:
		return styleRed
	default:
		return styleNormal
	}
}

func runModeColor(mode pole.RunMode) lipgloss.Style {
	switch mode {
	case pole.RunModeGpu:
		return styleGreen
	case pole.RunModeMoeOffload:
		return styleCyan
	case pole.RunModeCpuOffload:
		return styleYellow
	case pole.RunModeCpuOnly:
		return styleDim
	default:
		return styleNormal
	}
}

func renderTable(app *App, width, height int) string {
	headers := []string{"", "Model", "Provider", "Params", "Score", "tok/s", "Quant", "Mode", "Mem%", "Ctx", "Fit", "Use Case"}
	colWidths := []int{2, 20, 12, 8, 6, 6, 7, 7, 6, 5, 10, 12}
	headerLine := ""
	for i, h := range headers {
		w := colWidths[i]
		if i < len(colWidths) {
			headerLine += truncPad(h, w) + " "
		}
	}
	headerLine = styleCyan.Bold(true).Render(headerLine)

	var rows []string
	start := 0
	end := len(app.FilteredFits)
	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	if end > visible {
		if app.SelectedRow >= end-visible {
			start = end - visible
		} else if app.SelectedRow > 0 {
			start = app.SelectedRow
		}
		end = start + visible
		if end > len(app.FilteredFits) {
			end = len(app.FilteredFits)
		}
	}
	for rowIdx := start; rowIdx < end; rowIdx++ {
		idx := app.FilteredFits[rowIdx]
		fit := app.AllFits[idx]
		indicator := "●"
		cellStyle := fitColor(fit.FitLevel)
		scoreStyle := styleNormal
		if fit.Score >= 70 {
			scoreStyle = styleGreen
		} else if fit.Score >= 50 {
			scoreStyle = styleYellow
		} else {
			scoreStyle = styleRed
		}
		tpsStr := fmt.Sprintf("%.1f", fit.EstimatedTPS)
		if fit.EstimatedTPS >= 100 {
			tpsStr = fmt.Sprintf("%.0f", fit.EstimatedTPS)
		}
		cells := []string{
			cellStyle.Render(indicator),
			styleNormal.Render(truncPad(fit.Model.Name, colWidths[1])),
			styleDim.Render(truncPad(fit.Model.Provider, colWidths[2])),
			styleNormal.Render(truncPad(fit.Model.ParameterCount, colWidths[3])),
			scoreStyle.Render(truncPad(fmt.Sprintf("%.0f", fit.Score), colWidths[4])),
			styleNormal.Render(truncPad(tpsStr, colWidths[5])),
			styleDim.Render(truncPad(fit.BestQuant, colWidths[6])),
			runModeColor(fit.RunMode).Render(truncPad(fit.RunModeText(), colWidths[7])),
			cellStyle.Render(truncPad(fmt.Sprintf("%.0f%%", fit.UtilizationPct), colWidths[8])),
			styleDim.Render(truncPad(fmt.Sprintf("%dk", fit.Model.ContextLength/1000), colWidths[9])),
			cellStyle.Render(truncPad(fit.FitText(), colWidths[10])),
			styleDim.Render(truncPad(fit.UseCase.String(), colWidths[11])),
		}
		line := ""
		for i, c := range cells {
			line += lipgloss.NewStyle().Width(colWidths[i]).Render(c) + " "
		}
		if rowIdx == app.SelectedRow {
			line = lipgloss.NewStyle().Background(lipgloss.Color("8")).Bold(true).Render("▶ "+line) 
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}

	title := fmt.Sprintf(" Models (%d/%d) ", len(app.FilteredFits), len(app.AllFits))
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	body := headerLine + "\n" + strings.Join(rows, "\n")
	return block.Render(styleNormal.Render(title) + "\n" + body)
}

func truncPad(s string, w int) string {
	runes := []rune(s)
	if len(runes) <= w {
		return s + strings.Repeat(" ", w-len(runes))
	}
	return string(runes[:w-1]) + "…"
}

func renderStatusBar(app *App) string {
	var keys, modeText string
	switch app.InputMode {
	case InputModeNormal:
		detailKey := "Enter:detail"
		if app.ShowDetail {
			detailKey = "Enter:table"
		}
		keys = fmt.Sprintf(" ↑↓/jk:navigate  %s  /:search  f:fit filter  p:providers  q:quit", detailKey)
		modeText = "NORMAL"
	case InputModeSearch:
		keys = "  Type to search  Esc:done  Ctrl-U:clear"
		modeText = "SEARCH"
	case InputModeProviderPopup:
		keys = "  ↑↓/jk:navigate  Space:toggle  a:all/none  Esc:close"
		modeText = "PROVIDERS"
	}
	return styleStatus.Render(" "+modeText+" ") + styleDim.Render(keys)
}

func renderDetail(app *App, width, height int) string {
	fit := app.SelectedFit()
	if fit == nil {
		block := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		return block.Render(" No model selected ")
	}
	cellStyle := fitColor(fit.FitLevel)
	var lines []string
	lines = append(lines, "")
	lines = append(lines, styleDim.Render("  Model:       ")+styleNormal.Bold(true).Render(fit.Model.Name))
	lines = append(lines, styleDim.Render("  Provider:    ")+styleNormal.Render(fit.Model.Provider))
	lines = append(lines, styleDim.Render("  Parameters:  ")+styleNormal.Render(fit.Model.ParameterCount))
	lines = append(lines, styleDim.Render("  Quantization:")+styleNormal.Render(" "+fit.Model.Quantization))
	lines = append(lines, styleDim.Render("  Best Quant:  ")+styleGreen.Render(fmt.Sprintf(" %s (for this hardware)", fit.BestQuant)))
	lines = append(lines, styleDim.Render("  Context:     ")+styleNormal.Render(fmt.Sprintf("%d tokens", fit.Model.ContextLength)))
	lines = append(lines, styleDim.Render("  Use Case:    ")+styleNormal.Render(fit.Model.UseCase))
	lines = append(lines, styleDim.Render("  Category:    ")+styleCyan.Render(fit.UseCase.String()))
	lines = append(lines, "")
	lines = append(lines, styleCyan.Render("  ── Score Breakdown ──"))
	lines = append(lines, "")
	scoreStyle := styleNormal
	if fit.Score >= 70 {
		scoreStyle = styleGreen
	} else if fit.Score >= 50 {
		scoreStyle = styleYellow
	} else {
		scoreStyle = styleRed
	}
	lines = append(lines, styleDim.Render("  Overall:     ")+scoreStyle.Bold(true).Render(fmt.Sprintf("%.1f / 100", fit.Score)))
	lines = append(lines, styleDim.Render("  Quality:     ")+styleNormal.Render(fmt.Sprintf("%.0f", fit.ScoreComponents.Quality))+
		styleDim.Render("  Speed: ")+styleNormal.Render(fmt.Sprintf("%.0f", fit.ScoreComponents.Speed))+
		styleDim.Render("  Fit: ")+styleNormal.Render(fmt.Sprintf("%.0f", fit.ScoreComponents.Fit))+
		styleDim.Render("  Context: ")+styleNormal.Render(fmt.Sprintf("%.0f", fit.ScoreComponents.Context)))
	lines = append(lines, styleDim.Render("  Est. Speed:  ")+styleNormal.Render(fmt.Sprintf("%.1f tok/s", fit.EstimatedTPS)))

	if fit.Model.IsMoE {
		lines = append(lines, "")
		lines = append(lines, styleCyan.Render("  ── MoE Architecture ──"))
		lines = append(lines, "")
		if fit.Model.NumExperts != nil && fit.Model.ActiveExperts != nil {
			lines = append(lines, styleDim.Render("  Experts:     ")+styleCyan.Render(fmt.Sprintf("%d active / %d total per token", *fit.Model.ActiveExperts, *fit.Model.NumExperts)))
		}
		if v := fit.Model.MoeActiveVRAMGB(); v != nil {
			minV := 0.0
			if fit.Model.MinVRAMGB != nil {
				minV = *fit.Model.MinVRAMGB
			}
			lines = append(lines, styleDim.Render("  Active VRAM: ")+styleCyan.Render(fmt.Sprintf("%.1f GB", *v))+styleDim.Render(fmt.Sprintf("  (vs %.1f GB full model)", minV)))
		}
		if fit.MoeOffloadedGB != nil {
			lines = append(lines, styleDim.Render("  Offloaded:   ")+styleYellow.Render(fmt.Sprintf("%.1f GB inactive experts in RAM", *fit.MoeOffloadedGB)))
		}
		if fit.RunMode == pole.RunModeMoeOffload {
			lines = append(lines, styleDim.Render("  Strategy:    ")+styleGreen.Render("Expert offloading (active in VRAM, inactive in RAM)"))
		} else if fit.RunMode == pole.RunModeGpu {
			lines = append(lines, styleDim.Render("  Strategy:    ")+styleGreen.Render("All experts loaded in VRAM (optimal)"))
		}
	}

	lines = append(lines, "")
	lines = append(lines, styleCyan.Render("  ── System Fit ──"))
	lines = append(lines, "")
	lines = append(lines, styleDim.Render("  Fit Level:   ")+cellStyle.Bold(true).Render(fmt.Sprintf("● %s", fit.FitText())))
	lines = append(lines, styleDim.Render("  Run Mode:    ")+styleNormal.Bold(true).Render(fit.RunModeText()))
	lines = append(lines, "")
	lines = append(lines, styleCyan.Render("  -- Memory --"))
	lines = append(lines, "")
	if fit.Model.MinVRAMGB != nil {
		vramLabel := "  (no GPU)"
		if app.Specs.HasGPU {
			if app.Specs.UnifiedMemory {
				if app.Specs.GpuVRAMGB != nil {
					vramLabel = fmt.Sprintf("  (shared: %.1f GB)", *app.Specs.GpuVRAMGB)
				} else {
					vramLabel = "  (shared memory)"
				}
			} else if app.Specs.GpuVRAMGB != nil {
				vramLabel = fmt.Sprintf("  (system: %.1f GB)", *app.Specs.GpuVRAMGB)
			} else {
				vramLabel = "  (system: unknown)"
			}
		}
		lines = append(lines, styleDim.Render("  Min VRAM:    ")+styleNormal.Render(fmt.Sprintf("%.1f GB", *fit.Model.MinVRAMGB))+styleDim.Render(vramLabel))
	}
	lines = append(lines, styleDim.Render("  Min RAM:     ")+styleNormal.Render(fmt.Sprintf("%.1f GB", fit.Model.MinRAMGB))+styleDim.Render(fmt.Sprintf("  (system: %.1f GB avail)", app.Specs.AvailableRAMGB)))
	lines = append(lines, styleDim.Render("  Rec RAM:     ")+styleNormal.Render(fmt.Sprintf("%.1f GB", fit.Model.RecommendedRAMGB)))
	lines = append(lines, styleDim.Render("  Mem Usage:   ")+cellStyle.Render(fmt.Sprintf("%.1f%%", fit.UtilizationPct))+styleDim.Render(fmt.Sprintf("  (%.1f / %.1f GB)", fit.MemoryRequiredGB, fit.MemoryAvailableGB)))
	lines = append(lines, "")
	if len(fit.Notes) > 0 {
		lines = append(lines, styleCyan.Render("  ── Notes ──"))
		lines = append(lines, "")
		for _, n := range fit.Notes {
			lines = append(lines, styleNormal.Render("  "+n))
		}
	}

	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	return block.Render(styleNormal.Bold(true).Render(" "+fit.Model.Name+" ") + "\n" + strings.Join(lines, "\n"))
}

func renderProviderPopup(app *App, width, height int) string {
	maxNameLen := 10
	for _, p := range app.Providers {
		if len(p) > maxNameLen {
			maxNameLen = len(p)
		}
	}
	popupW := maxNameLen + 10
	if popupW > width-4 {
		popupW = width - 4
	}
	popupH := len(app.Providers) + 2
	if popupH > height-4 {
		popupH = height - 4
	}
	innerH := popupH - 2
	scrollOffset := 0
	if app.ProviderCursor >= innerH {
		scrollOffset = app.ProviderCursor - innerH + 1
	}
	activeCount := 0
	for _, s := range app.SelectedProviders {
		if s {
			activeCount++
		}
	}
	title := fmt.Sprintf(" Providers (%d/%d) ", activeCount, len(app.Providers))
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("11")).
		Padding(0, 1).
		Width(popupW)
	var lines []string
	for i := scrollOffset; i < len(app.Providers) && len(lines) < innerH; i++ {
		cb := "[ ]"
		if app.SelectedProviders[i] {
			cb = "[x]"
		}
		line := cb + " " + app.Providers[i]
		if i == app.ProviderCursor {
			line = styleYellow.Bold(true).Render(line)
		} else if app.SelectedProviders[i] {
			line = styleGreen.Render(line)
		} else {
			line = styleDim.Render(line)
		}
		lines = append(lines, line)
	}
	return block.Render(styleYellow.Bold(true).Render(title)+"\n"+strings.Join(lines, "\n"))
}
