package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

// appModel is the top-level Bubble Tea model and the screen router. It owns the
// three sub-screens (menu, speed test, monitor) and routes messages between
// them. The sub-models no longer implement tea.Model themselves — they expose
// Start/Update/View and this router drives them, keeping a single point of
// control for quit + back-to-menu navigation.
type appModel struct {
	theme Theme

	width  int
	height int

	screen screenID
	menu   *menuModel
	test   *model
	monitor *monitorModel
}

func newAppModel(theme Theme) appModel {
	return appModel{
		theme:  theme,
		screen: screenMenu,
		menu:   newMenuModel(theme),
	}
}

func (a appModel) Init() tea.Cmd {
	// Animate the menu's spinner; sub-screens start their own commands when
	// entered.
	return a.menu.Init()
}

// enter builds (lazily) and starts the chosen sub-screen, replacing whatever
// was active and cancelling the previous one.
func (a *appModel) enter(s screenID) tea.Cmd {
	// Tear down any previous sub-screen.
	if a.test != nil && a.test.cancel != nil {
		a.test.cancel()
	}
	if a.monitor != nil && a.monitor.cancel != nil {
		a.monitor.cancel()
	}

	switch s {
	case screenTest:
		cs := newCardState(a.theme)
		a.test = newTestModel(cs)
		a.test.width = a.width
		a.test.height = a.height
		a.screen = screenTest
		return a.test.Start()
	case screenMonitor:
		cs := newCardState(a.theme)
		a.monitor = newMonitorModel(cs)
		a.monitor.width = a.width
		a.monitor.height = a.height
		a.screen = screenMonitor
		return a.monitor.Start()
	default:
		a.screen = screenMenu
		return nil
	}
}

func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.menu != nil {
			a.menu.width = msg.Width
			a.menu.height = msg.Height
		}
		if a.test != nil {
			a.test.width = msg.Width
			a.test.height = msg.Height
		}
		if a.monitor != nil {
			a.monitor.width = msg.Width
			a.monitor.height = msg.Height
		}
		return a, nil

	case tea.KeyMsg:
		// Global quit from anywhere.
		if msg.String() == "ctrl+c" {
			a.quitAll()
			return a, tea.Quit
		}
		// Global back-to-menu (except on the menu itself).
		if a.screen != screenMenu && (msg.String() == "esc" || msg.String() == "m") {
			a.backToMenu()
			return a, nil
		}
		if a.screen == screenMenu && (msg.String() == "q") {
			return a, tea.Quit
		}

	case menuSelectMsg:
		return a, a.enter(msg.screen)

	case backToMenuMsg:
		a.backToMenu()
		return a, nil
	}

	// Route to the active sub-model.
	switch a.screen {
	case screenMenu:
		cmd, quit := a.menu.Update(msg)
		if quit {
			a.quitAll()
			return a, tea.Quit
		}
		return a, cmd
	case screenTest:
		if a.test == nil {
			return a, nil
		}
		cmd, _ := a.test.Update(msg)
		return a, cmd
	case screenMonitor:
		if a.monitor == nil {
			return a, nil
		}
		cmd, _ := a.monitor.Update(msg)
		return a, cmd
	}
	return a, nil
}

// backToMenu cancels the active sub-screen and returns to the menu.
func (a *appModel) backToMenu() {
	if a.test != nil && a.test.cancel != nil {
		a.test.cancel()
	}
	if a.monitor != nil && a.monitor.cancel != nil {
		a.monitor.cancel()
	}
	a.screen = screenMenu
}

// quitAll cancels every in-flight background test/monitor.
func (a *appModel) quitAll() {
	if a.test != nil && a.test.cancel != nil {
		a.test.cancel()
	}
	if a.monitor != nil && a.monitor.cancel != nil {
		a.monitor.cancel()
	}
}

func (a appModel) View() string {
	switch a.screen {
	case screenTest:
		if a.test == nil {
			return ""
		}
		return a.test.View()
	case screenMonitor:
		if a.monitor == nil {
			return ""
		}
		return a.monitor.View()
	default:
		return a.menu.View()
	}
}
