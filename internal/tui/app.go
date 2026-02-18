package tui

import (
	"sort"
	"strings"

	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/pole"
)

// InputMode is the current TUI input mode (normal, search, or provider popup).
type InputMode int

const (
	InputModeNormal InputMode = iota
	InputModeSearch
	InputModeProviderPopup
)

// FitFilter filters the model list by fit level (All, Runnable, Perfect, Good, Marginal; cycle with same key).
type FitFilter int

const (
	FitFilterAll FitFilter = iota
	FitFilterRunnable
	FitFilterPerfect
	FitFilterGood
	FitFilterMarginal
)

func (f FitFilter) Label() string {
	switch f {
	case FitFilterAll:
		return "All"
	case FitFilterRunnable:
		return "Runnable"
	case FitFilterPerfect:
		return "Perfect"
	case FitFilterGood:
		return "Good"
	case FitFilterMarginal:
		return "Marginal"
	default:
		return "All"
	}
}

func (f FitFilter) Next() FitFilter {
	switch f {
	case FitFilterAll:
		return FitFilterRunnable
	case FitFilterRunnable:
		return FitFilterPerfect
	case FitFilterPerfect:
		return FitFilterGood
	case FitFilterGood:
		return FitFilterMarginal
	case FitFilterMarginal:
		return FitFilterAll
	default:
		return FitFilterAll
	}
}

// App holds the TUI state (specs, fits, filters, selection, providers).
type App struct {
	ShouldQuit   bool
	InputMode    InputMode
	SearchQuery  string
	CursorPosition int

	Specs             *hardware.SystemSpecs
	AllFits           []*pole.ModelFit
	FilteredFits      []int // indices into AllFits
	Providers         []string
	SelectedProviders []bool

	FitFilter   FitFilter
	SelectedRow int
	ShowDetail  bool
	ProviderCursor int

	Width  int
	Height int
}

// NewApp builds app state from specs and pre-analyzed fits (caller must have run RankModelsByFit).
func NewApp(specs *hardware.SystemSpecs, allFits []*pole.ModelFit) *App {
	providerSet := make(map[string]struct{})
	for _, f := range allFits {
		providerSet[f.Model.Provider] = struct{}{}
	}
	providers := make([]string, 0, len(providerSet))
	for p := range providerSet {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	selectedProviders := make([]bool, len(providers))
	for i := range selectedProviders {
		selectedProviders[i] = true
	}
	filteredFits := make([]int, len(allFits))
	for i := range filteredFits {
		filteredFits[i] = i
	}
	app := &App{
		Specs:             specs,
		AllFits:           allFits,
		FilteredFits:      filteredFits,
		Providers:         providers,
		SelectedProviders: selectedProviders,
		FitFilter:         FitFilterAll,
	}
	app.ApplyFilters()
	return app
}

// ApplyFilters updates FilteredFits from search, provider, and fit filters; clamps SelectedRow.
func (a *App) ApplyFilters() {
	query := strings.ToLower(a.SearchQuery)
	var out []int
	for i, fit := range a.AllFits {
		m := fit.Model
		matchesSearch := query == "" ||
			strings.Contains(strings.ToLower(m.Name), query) ||
			strings.Contains(strings.ToLower(m.Provider), query) ||
			strings.Contains(strings.ToLower(m.ParameterCount), query) ||
			strings.Contains(strings.ToLower(m.UseCase), query)
		providerIdx := -1
		for j, p := range a.Providers {
			if p == m.Provider {
				providerIdx = j
				break
			}
		}
		matchesProvider := providerIdx < 0 || (providerIdx < len(a.SelectedProviders) && a.SelectedProviders[providerIdx])
		matchesFit := true
		switch a.FitFilter {
		case FitFilterAll:
			// noop
		case FitFilterRunnable:
			matchesFit = fit.FitLevel != pole.FitTooTight
		case FitFilterPerfect:
			matchesFit = fit.FitLevel == pole.FitPerfect
		case FitFilterGood:
			matchesFit = fit.FitLevel == pole.FitGood
		case FitFilterMarginal:
			matchesFit = fit.FitLevel == pole.FitMarginal
		}
		if matchesSearch && matchesProvider && matchesFit {
			out = append(out, i)
		}
	}
	a.FilteredFits = out
	if len(a.FilteredFits) == 0 {
		a.SelectedRow = 0
	} else if a.SelectedRow >= len(a.FilteredFits) {
		a.SelectedRow = len(a.FilteredFits) - 1
	}
}

// SelectedFit returns the currently selected fit or nil.
func (a *App) SelectedFit() *pole.ModelFit {
	if len(a.FilteredFits) == 0 || a.SelectedRow < 0 || a.SelectedRow >= len(a.FilteredFits) {
		return nil
	}
	idx := a.FilteredFits[a.SelectedRow]
	if idx < 0 || idx >= len(a.AllFits) {
		return nil
	}
	return a.AllFits[idx]
}

func (a *App) MoveUp() {
	if a.SelectedRow > 0 {
		a.SelectedRow--
	}
}

func (a *App) MoveDown() {
	if len(a.FilteredFits) > 0 && a.SelectedRow < len(a.FilteredFits)-1 {
		a.SelectedRow++
	}
}

func (a *App) PageUp() {
	a.SelectedRow -= 10
	if a.SelectedRow < 0 {
		a.SelectedRow = 0
	}
}

func (a *App) PageDown() {
	if len(a.FilteredFits) == 0 {
		return
	}
	a.SelectedRow += 10
	if a.SelectedRow >= len(a.FilteredFits) {
		a.SelectedRow = len(a.FilteredFits) - 1
	}
}

func (a *App) Home() {
	a.SelectedRow = 0
}

func (a *App) End() {
	if len(a.FilteredFits) > 0 {
		a.SelectedRow = len(a.FilteredFits) - 1
	}
}

func (a *App) CycleFitFilter() {
	a.FitFilter = a.FitFilter.Next()
	a.ApplyFilters()
}

func (a *App) EnterSearch() {
	a.InputMode = InputModeSearch
}

func (a *App) ExitSearch() {
	a.InputMode = InputModeNormal
}

func (a *App) SearchInput(r rune) {
	runes := []rune(a.SearchQuery)
	if a.CursorPosition > len(runes) {
		a.CursorPosition = len(runes)
	}
	runes = append(runes[:a.CursorPosition], append([]rune{r}, runes[a.CursorPosition:]...)...)
	a.SearchQuery = string(runes)
	a.CursorPosition++
	a.ApplyFilters()
}

func (a *App) SearchBackspace() {
	runes := []rune(a.SearchQuery)
	if a.CursorPosition <= 0 || a.CursorPosition > len(runes) {
		return
	}
	runes = append(runes[:a.CursorPosition-1], runes[a.CursorPosition:]...)
	a.SearchQuery = string(runes)
	a.CursorPosition--
	a.ApplyFilters()
}

func (a *App) SearchDelete() {
	runes := []rune(a.SearchQuery)
	if a.CursorPosition < 0 || a.CursorPosition >= len(runes) {
		return
	}
	runes = append(runes[:a.CursorPosition], runes[a.CursorPosition+1:]...)
	a.SearchQuery = string(runes)
	a.ApplyFilters()
}

func (a *App) ClearSearch() {
	a.SearchQuery = ""
	a.CursorPosition = 0
	a.ApplyFilters()
}

func (a *App) ToggleDetail() {
	a.ShowDetail = !a.ShowDetail
}

func (a *App) OpenProviderPopup() {
	a.InputMode = InputModeProviderPopup
}

func (a *App) CloseProviderPopup() {
	a.InputMode = InputModeNormal
}

func (a *App) ProviderPopupUp() {
	if a.ProviderCursor > 0 {
		a.ProviderCursor--
	}
}

func (a *App) ProviderPopupDown() {
	if a.ProviderCursor+1 < len(a.Providers) {
		a.ProviderCursor++
	}
}

func (a *App) ProviderPopupToggle() {
	if a.ProviderCursor < len(a.SelectedProviders) {
		a.SelectedProviders[a.ProviderCursor] = !a.SelectedProviders[a.ProviderCursor]
		a.ApplyFilters()
	}
}

func (a *App) ProviderPopupSelectAll() {
	allSelected := true
	for _, s := range a.SelectedProviders {
		if !s {
			allSelected = false
			break
		}
	}
	val := !allSelected
	for i := range a.SelectedProviders {
		a.SelectedProviders[i] = val
	}
	a.ApplyFilters()
}
