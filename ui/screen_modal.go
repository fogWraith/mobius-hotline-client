package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Messages sent from ModalScreen to parent
type ModalCancelledMsg struct{}

type ModalButtonClickedMsg struct {
	Title         string // Modal title for context
	ButtonClicked string // Which button was clicked
}

// ModalScreen encapsulates the modal dialog component
type ModalScreen struct {
	// Bubble Tea components
	form *huh.Form

	// Screen dimensions
	width, height int

	// Reference to parent model
	model *Model

	// Modal state
	title   string
	content string
	buttons []string
}

// NewModalScreen creates a new modal screen instance
func NewModalScreen(title, content string, buttons []string, m *Model) *ModalScreen {
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}

	s := &ModalScreen{
		title:   title,
		content: content,
		buttons: buttons,
		width:   m.width,
		height:  m.height,
		model:   m,
	}

	s.initForm()
	return s
}

// initForm creates the huh form for the modal buttons
func (s *ModalScreen) initForm() {
	var confirmField *huh.Confirm
	if len(s.buttons) == 1 {
		// Single button modal - just shows one button (always returns true)
		confirmField = huh.NewConfirm().
			Key("confirm").
			Affirmative(s.buttons[0]).
			Negative("")
	} else if len(s.buttons) >= 2 {
		// Two button modal - second button is affirmative (default), first is negative
		confirmField = huh.NewConfirm().
			Key("confirm").
			Value(func() *bool { t := true; return &t }()).
			Affirmative(s.buttons[1]).
			Negative(s.buttons[0])
	}

	s.form = huh.NewForm(
		huh.NewGroup(confirmField),
	).
		WithWidth(60).
		WithShowHelp(false).
		WithShowErrors(false)
}

// Init returns initial commands
func (s *ModalScreen) Init() tea.Cmd {
	if s.form != nil {
		return s.form.Init()
	}
	return nil
}

// Update handles messages and returns updated screen + commands
func (s *ModalScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	case ModalCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case ModalButtonClickedMsg:
		cmd := s.model.handleModalButtonClickedMsg(msg)
		return s, cmd

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate to form component
	if s.form != nil {
		form, cmd := s.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.form = f
		}

		// Check if form is complete
		if s.form.State == huh.StateCompleted {
			return s, s.handleFormComplete()
		}

		return s, cmd
	}

	return s, nil
}

// handleKeys handles keyboard input
func (s *ModalScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return ModalCancelledMsg{} }
	}

	// Pass to form component
	if s.form != nil {
		form, cmd := s.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			s.form = f
		}

		// Check if form is complete after key handling
		if s.form.State == huh.StateCompleted {
			return s, s.handleFormComplete()
		}

		return s, cmd
	}

	return s, nil
}

// handleFormComplete processes form completion and sends appropriate message
func (s *ModalScreen) handleFormComplete() tea.Cmd {
	// Get boolean from Confirm field and map to button label
	confirmed := s.form.GetBool("confirm")

	var buttonClicked string
	if confirmed {
		buttonClicked = s.buttons[len(s.buttons)-1] // Affirmative is last button
	} else if len(s.buttons) > 1 {
		buttonClicked = s.buttons[0] // Negative is first button
	}

	return func() tea.Msg {
		return ModalButtonClickedMsg{
			Title:         s.title,
			ButtonClicked: buttonClicked,
		}
	}
}

// View renders the modal screen
func (s *ModalScreen) View() string {
	title := lipgloss.NewStyle().Align(lipgloss.Center).Render(rainbow(lipgloss.NewStyle(), s.title, blends))

	var body string
	if s.content != "" {
		body = lipgloss.NewStyle().
			Padding(1).
			Width(50).Render(s.content)
	}

	// Render the huh form buttons
	var buttons string
	if s.form != nil {
		buttons = lipgloss.NewStyle().
			Width(50).
			Align(lipgloss.Center).
			Render(s.form.View())
	}

	return lipgloss.Place(s.width, s.height,
		lipgloss.Center, lipgloss.Center,
		dialogBoxStyle.Render(lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			body,
			buttons,
		)),
		lipgloss.WithWhitespaceChars("☃︎"),
		lipgloss.WithWhitespaceForeground(subtle),
	)
}

// SetSize updates dimensions
func (s *ModalScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// GetTitle returns the modal title (for parent context)
func (s *ModalScreen) GetTitle() string {
	return s.title
}
