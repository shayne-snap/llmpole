package tui

import (
	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/pole"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI. specs and allFits must already be loaded (e.g. from main).
func Run(specs *hardware.SystemSpecs, allFits []*pole.ModelFit) error {
	app := NewApp(specs, allFits)
	m := &model{app: app}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type model struct {
	app *App
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.app.Width = msg.Width
		m.app.Height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch m.app.InputMode {
		case InputModeNormal:
			m.handleNormal(msg)
		case InputModeSearch:
			m.handleSearch(msg)
		case InputModeProviderPopup:
			m.handleProviderPopup(msg)
		}
		if m.app.ShouldQuit {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m *model) handleNormal(msg tea.KeyMsg) {
	s := msg.String()
	switch s {
	case "q", "esc":
		if m.app.ShowDetail {
			m.app.ShowDetail = false
		} else {
			m.app.ShouldQuit = true
		}
	case "up", "k":
		m.app.MoveUp()
	case "down", "j":
		m.app.MoveDown()
	case "pgup":
		m.app.PageUp()
	case "pgdown":
		m.app.PageDown()
	case "home", "g":
		m.app.Home()
	case "end", "G":
		m.app.End()
	case "/":
		m.app.EnterSearch()
	case "f":
		m.app.CycleFitFilter()
	case "p":
		m.app.OpenProviderPopup()
	case "enter":
		m.app.ToggleDetail()
	}
}

func (m *model) handleSearch(msg tea.KeyMsg) {
	s := msg.String()
	switch s {
	case "esc", "enter":
		m.app.ExitSearch()
	case "backspace":
		m.app.SearchBackspace()
	case "delete":
		m.app.SearchDelete()
	case "ctrl+u":
		m.app.ClearSearch()
	case "up", "k":
		m.app.MoveUp()
	case "down", "j":
		m.app.MoveDown()
	default:
		if len(msg.Runes) == 1 {
			m.app.SearchInput(msg.Runes[0])
		}
	}
}

func (m *model) handleProviderPopup(msg tea.KeyMsg) {
	s := msg.String()
	switch s {
	case "esc", "p", "q":
		m.app.CloseProviderPopup()
	case "up", "k":
		m.app.ProviderPopupUp()
	case "down", "j":
		m.app.ProviderPopupDown()
	case " ", "enter":
		m.app.ProviderPopupToggle()
	case "a":
		m.app.ProviderPopupSelectAll()
	}
}

func (m *model) View() string {
	return Render(m.app)
}
