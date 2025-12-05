package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages sent from ComposeMessageScreen to parent

// ComposeMessageSentMsg signals user sent a private message
type ComposeMessageSentMsg struct {
	TargetID  [2]byte
	Text      string
	QuoteText string
}

// ComposeMessageCancelledMsg signals user cancelled composing
type ComposeMessageCancelledMsg struct{}

// ComposeMessageScreen is a self-contained BubbleTea model for composing private messages
type ComposeMessageScreen struct {
	textInput  textinput.Model
	width, height int
	model      *Model

	targetID   [2]byte
	targetName string
	quoteText  string
}

// NewComposeMessageScreen creates a new compose message screen
func NewComposeMessageScreen(targetID [2]byte, targetName string, quoteText string, m *Model) *ComposeMessageScreen {
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.Width = 50
	ti.Focus()

	return &ComposeMessageScreen{
		textInput:  ti,
		width:      m.width,
		height:     m.height,
		model:      m,
		targetID:   targetID,
		targetName: targetName,
		quoteText:  quoteText,
	}
}

// Init implements tea.Model
func (s *ComposeMessageScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements ScreenModel
func (s *ComposeMessageScreen) Update(msg tea.Msg) (ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil

	// Handle screen messages by delegating to parent methods
	case ComposeMessageSentMsg:
		return s, s.model.handleComposeMessageSentMsg(msg)

	case ComposeMessageCancelledMsg:
		s.model.PopScreen()
		return s, nil

	case tea.KeyMsg:
		return s.handleKeys(msg)
	}

	// Delegate to internal component
	var cmd tea.Cmd
	s.textInput, cmd = s.textInput.Update(msg)
	return s, cmd
}

// View implements tea.Model
func (s *ComposeMessageScreen) View() string {
	var b strings.Builder

	b.WriteString(serverTitleStyle.Render("Send Private Message to " + s.targetName))
	b.WriteString("\n\n")

	// Show quoted message if this is a reply
	if s.quoteText != "" {
		quotedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
		b.WriteString(quotedStyle.Render("> " + s.quoteText))
		b.WriteString("\n\n")
	}

	b.WriteString(s.textInput.View())
	b.WriteString("\n\n")
	b.WriteString("[Enter] Send  [Esc] Cancel")

	return lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Width(60).Render(b.String()),
	)
}

// SetSize updates dimensions
func (s *ComposeMessageScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// handleKeys handles keyboard input
func (s *ComposeMessageScreen) handleKeys(msg tea.KeyMsg) (ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return ComposeMessageCancelledMsg{} }

	case "enter":
		text := s.textInput.Value()
		if text != "" {
			targetID := s.targetID
			quoteText := s.quoteText
			return s, func() tea.Msg {
				return ComposeMessageSentMsg{
					TargetID:  targetID,
					Text:      text,
					QuoteText: quoteText,
				}
			}
		}
		return s, nil
	}

	// Pass other keys to the input
	var cmd tea.Cmd
	s.textInput, cmd = s.textInput.Update(msg)
	return s, cmd
}
