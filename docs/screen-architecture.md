# Screen Architecture

This document describes the architectural pattern used for screen components in the Mobius Hotline Client.

## Overview

The client uses the [Bubble Tea](https://github.com/charmbracelet/bubbletea) library which follows the Elm architecture (Model-Update-View). Each major screen is implemented as a self-contained model that encapsulates its own state and behavior while communicating with the parent model through typed messages.

## Pattern Structure

### Screen Model

Each screen is defined as a struct that implements a subset of the `tea.Model` interface:

```go
type ExampleScreen struct {
    // Bubble Tea components
    list          list.Model
    viewport      viewport.Model

    // Screen dimensions
    width, height int

    // Reference to parent model for callbacks
    model         *Model

    // Screen-specific state
    items         []exampleItem
    selectedItem  *exampleData
}
```

### Constructor

A constructor function creates and initializes the screen:

```go
func NewExampleScreen(data []Item, m *Model) *ExampleScreen {
    // Initialize Bubble Tea components
    l := list.New(items, newExampleDelegate(), m.width, m.height)
    l.SetShowTitle(false)
    l.DisableQuitKeybindings()

    return &ExampleScreen{
        list:   l,
        width:  m.width,
        height: m.height,
        model:  m,
    }
}
```

### Message Types

Screens communicate with the parent model through typed messages defined at the package level:

```go
// Messages sent from ExampleScreen to parent
type ExampleSelectedMsg struct {
    Item Item
}

type ExampleCancelledMsg struct{}

type ExampleActionMsg struct {
    Data string
}
```

### Tea.Model Methods

Each screen implements these methods:

```go
// Init returns initial commands (usually nil)
func (s *ExampleScreen) Init() tea.Cmd {
    return nil
}

// Update handles messages and returns updated screen + commands
func (s *ExampleScreen) Update(msg tea.Msg) (*ExampleScreen, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        s.SetSize(msg.Width, msg.Height)
        return s, nil

    // Handle screen messages by delegating to parent methods
    case ExampleSelectedMsg:
        s.model.handleExampleSelectedMsg(msg)
        return s, nil
    case ExampleCancelledMsg:
        s.model.handleExampleCancelledMsg()
        return s, nil
    case ExampleActionMsg:
        cmd := s.model.handleExampleActionMsg(msg)
        return s, cmd  // Return command if handler produces one

    case tea.KeyMsg:
        return s.handleKeys(msg)
    }

    // Delegate to internal components
    var cmd tea.Cmd
    s.list, cmd = s.list.Update(msg)
    return s, cmd
}

// View renders the screen
func (s *ExampleScreen) View() string {
    return s.list.View()
}

// SetSize updates dimensions
func (s *ExampleScreen) SetSize(width, height int) {
    s.width = width
    s.height = height
    s.list.SetSize(width, height)
}
```

### Key Handling

Key handling is encapsulated within the screen, emitting messages for parent actions:

```go
func (s *ExampleScreen) handleKeys(msg tea.KeyMsg) (*ExampleScreen, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return s, func() tea.Msg { return ExampleCancelledMsg{} }

    case "enter":
        if item, ok := s.list.SelectedItem().(exampleItem); ok {
            return s, func() tea.Msg {
                return ExampleSelectedMsg{Item: item.data}
            }
        }
    }

    // Pass to list component
    var cmd tea.Cmd
    s.list, cmd = s.list.Update(msg)
    return s, cmd
}
```

## Parent Model Integration

### Screen Storage

The parent model holds a pointer to each screen:

```go
type Model struct {
    // Screen instances
    exampleScreen *ExampleScreen

    // ... other fields
}
```

### Message Routing

The parent Update function routes messages to the active screen:

```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Route to screen-specific Update
    var cmd tea.Cmd
    switch m.CurrentScreen() {
    case ScreenExample:
        m.exampleScreen, cmd = m.exampleScreen.Update(msg)
        return m, cmd
    }

    // ... handle other messages
}
```

### View Delegation

The parent View function delegates to the screen's View:

```go
func (m *Model) View() string {
    switch m.CurrentScreen() {
    case ScreenExample:
        return m.exampleScreen.View()
    }
    return ""
}
```

### Handler Methods

Handler methods are called directly by the screen's Update method. They use simplified signatures since they don't need to return the model:

```go
// Simple handler - no return value needed
func (m *Model) handleExampleCancelledMsg() {
    m.PopScreen()
}

// Handler with typed message parameter
func (m *Model) handleExampleSelectedMsg(msg ExampleSelectedMsg) {
    m.doSomethingWith(msg.Item)
    m.PopScreen()
}

// Handler that returns a command (e.g., for form initialization)
func (m *Model) handleExampleActionMsg(msg ExampleActionMsg) tea.Cmd {
    cmd := m.initSomeForm(msg.Data)
    m.PushScreen(ScreenForm)
    return cmd
}
```

Note: Handlers are called inline from the screen's Update method, not via registered message handlers. This keeps the message flow explicit and the screen in control of its own messages.

## Screen Implementations

The following screens follow this pattern:

| Screen | File | Description |
|--------|------|-------------|
| HomeScreen | `ui/home_screen.go` | Main menu with options to join server, bookmarks, tracker, settings |
| TrackerScreen | `ui/tracker.go` | Browse and select servers from tracker |
| BookmarkScreen | `ui/bookmark.go` | Manage saved server bookmarks |
| SettingsScreen | `ui/settings.go` | Configure application settings |
| NewsScreen | `ui/news_screen.go` | Browse threaded news articles |
| FilesScreen | `ui/files_screen.go` | Browse and download server files |
| TasksScreen | `ui/tasks_screen.go` | View download/upload task progress |
| LogsScreen | `ui/logs_screen.go` | View debug logs |

## Benefits

1. **Encapsulation**: Each screen manages its own state and behavior
2. **Testability**: Screens can be tested in isolation
3. **Maintainability**: Changes to one screen don't affect others
4. **Type Safety**: Typed messages ensure compile-time checking of screen communication
5. **Consistency**: Common pattern makes codebase easier to understand

## Guidelines for New Screens

When creating a new screen:

1. Create a new file `ui/<screen_name>.go`
2. Define message types for parent communication
3. Create the screen struct with necessary state
4. Implement `Init()`, `Update()`, `View()`, `SetSize()` methods
5. Handle screen messages inline in `Update()` by calling parent handler methods
6. Add constructor function `New<ScreenName>Screen()`
7. Add screen pointer to parent Model struct
8. Add routing in parent `Update()` and `View()`
9. Create handler methods on parent Model for each message type
