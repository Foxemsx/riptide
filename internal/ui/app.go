package ui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Foxemsx/riptide/internal/db"
	apptheme "github.com/Foxemsx/riptide/internal/theme"
)

// App is the top-level Bubble Tea model and the screen router.
type App struct {
	theme   apptheme.Theme
	compact bool
	store   *db.Store
	version string

	width  int
	height int

	screen   screenID
	menu     *menuModel
	test     *model
	monitor  *monitorModel
	settings *settingsModel

	// cached history for the speed-test screen
	history []db.TestRun
}

// NewApp builds the root model for the riptide TUI.
func NewApp(t apptheme.Theme, compact bool, store *db.Store, version string) *App {
	if version == "" {
		version = "dev"
	}
	a := &App{
		theme:   t,
		compact: compact,
		store:   store,
		version: version,
		screen:  screenMenu,
		menu:    newMenuModel(t, compact, version),
	}
	a.reloadHistory()
	return a
}

func (a *App) Init() tea.Cmd {
	return a.menu.Init()
}

func (a *App) reloadHistory() {
	if a.store == nil {
		a.history = nil
		return
	}
	runs, err := a.store.LatestRuns(historyLimit)
	if err != nil {
		a.history = nil
		return
	}
	a.history = runs
}

// enter builds (lazily) and starts the chosen sub-screen.
func (a *App) enter(s screenID) tea.Cmd {
	if a.test != nil && a.test.cancel != nil {
		a.test.cancel()
	}
	if a.monitor != nil && a.monitor.cancel != nil {
		a.monitor.cancel()
	}

	switch s {
	case screenTest:
		cs := newCardState(a.theme, a.compact)
		a.test = newTestModel(cs, a.store)
		a.test.width = a.width
		a.test.height = a.height
		a.test.history = a.history
		a.test.syncLayout()
		a.screen = screenTest
		return a.test.Start()
	case screenMonitor:
		cs := newCardState(a.theme, a.compact)
		a.monitor = newMonitorModel(cs)
		a.monitor.width = a.width
		a.monitor.height = a.height
		a.monitor.syncLayout()
		a.screen = screenMonitor
		return a.monitor.Start()
	case screenSettings:
		a.settings = newSettingsModel(a.theme, a.compact, a.store, a.version)
		a.settings.width = a.width
		a.settings.height = a.height
		a.screen = screenSettings
		return a.settings.Init()
	default:
		a.screen = screenMenu
		return nil
	}
}

func (a *App) applyTheme(name string) {
	t := apptheme.Get(name)
	a.theme = t
	fmt.Fprint(os.Stdout, "\x1b]11;"+t.HexBG()+"\a")
	if a.menu != nil {
		a.menu.applyTheme(t)
	}
	if a.settings != nil {
		a.settings.applyTheme(t)
	}
	if a.test != nil {
		a.test.theme = t
		if a.test.dlGraph != nil {
			a.test.dlGraph.bottom, a.test.dlGraph.top = t.GraphDownBottom, t.GraphDownTop
		}
		if a.test.ulGraph != nil {
			a.test.ulGraph.bottom, a.test.ulGraph.top = t.GraphUpBottom, t.GraphUpTop
		}
	}
	if a.monitor != nil {
		a.monitor.theme = t
		if a.monitor.dlGraph != nil {
			a.monitor.dlGraph.bottom, a.monitor.dlGraph.top = t.GraphDownBottom, t.GraphDownTop
		}
		if a.monitor.ulGraph != nil {
			a.monitor.ulGraph.bottom, a.monitor.ulGraph.top = t.GraphUpBottom, t.GraphUpTop
		}
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			a.test.syncLayout()
			a.test.savePrompt.width = msg.Width
			a.test.savePrompt.height = msg.Height
		}
		if a.monitor != nil {
			a.monitor.width = msg.Width
			a.monitor.height = msg.Height
			a.monitor.syncLayout()
		}
		if a.settings != nil {
			a.settings.width = msg.Width
			a.settings.height = msg.Height
		}
		return a, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			a.quitAll()
			return a, tea.Quit
		}
		if msg.String() == "t" {
			if a.screen == screenSettings && a.settings != nil && a.settings.focus == focusSearch {
				// fall through — typing in search
			} else if a.screen == screenTest && a.test != nil && a.test.savePrompt.active {
				// fall through — naming a run
			} else {
				a.compact = !a.compact
				if a.menu != nil {
					a.menu.compact = a.compact
				}
				if a.test != nil {
					a.test.compact = a.compact
				}
				if a.monitor != nil {
					a.monitor.compact = a.compact
				}
				if a.settings != nil {
					a.settings.compact = a.compact
				}
				return a, nil
			}
		}
		if a.screen != screenMenu && a.screen != screenSettings && (msg.String() == "esc" || msg.String() == "m") {
			if a.screen == screenTest && a.test != nil && a.test.savePrompt.active {
				// fall through
			} else {
				a.backToMenu()
				return a, nil
			}
		}
		if a.screen == screenMenu && msg.String() == "q" {
			return a, tea.Quit
		}

	case menuSelectMsg:
		return a, a.enter(msg.screen)

	case backToMenuMsg:
		a.backToMenu()
		return a, nil

	case themeChangedMsg:
		a.applyTheme(msg.name)
		return a, nil

	case dbResetMsg:
		a.reloadHistory()
		if a.test != nil {
			a.test.history = a.history
		}
		return a, nil

	case saveRunMsg:
		if a.store != nil {
			if _, err := a.store.SaveTestRun(msg.run); err == nil {
				a.reloadHistory()
				if a.test != nil {
					a.test.history = a.history
					a.test.savedFlash = "Saved · " + msg.run.Name
				}
			}
		}
		return a, nil
	}

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
	case screenSettings:
		if a.settings == nil {
			return a, nil
		}
		cmd := a.settings.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) backToMenu() {
	if a.test != nil && a.test.cancel != nil {
		a.test.cancel()
	}
	if a.monitor != nil && a.monitor.cancel != nil {
		a.monitor.cancel()
	}
	a.reloadHistory()
	a.screen = screenMenu
}

func (a *App) quitAll() {
	if a.test != nil && a.test.cancel != nil {
		a.test.cancel()
	}
	if a.monitor != nil && a.monitor.cancel != nil {
		a.monitor.cancel()
	}
}

func (a *App) View() string {
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
	case screenSettings:
		if a.settings == nil {
			return ""
		}
		return a.settings.View()
	default:
		return a.menu.View()
	}
}

// Close releases the database handle.
func (a *App) Close() {
	if a.store != nil {
		_ = a.store.Close()
	}
}
