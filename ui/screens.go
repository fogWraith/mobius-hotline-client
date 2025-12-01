package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/jhalter/mobius/hotline"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/reflow/wordwrap"
)

var subScreenStyle = lipgloss.NewStyle().
	Border(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("62")). // Cyan border
	Background(lipgloss.Color("235")). // Dark gray background
	Foreground(lipgloss.Color("255")). // White text
	Padding(1, 1)

// Task widget styles
var taskWidgetStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("63")).
	Padding(0, 1).
	Width(26)

var taskActiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("170")).
	Bold(true)

var taskCompleteStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("2")) // Green

var taskFailedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("196")) // Red

var categoryStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("170"))

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("170"))

// Access bit definitions organized by category
var accessBitsByCategory = []struct {
	category string
	bits     []accessBitInfo
}{
	{
		category: "File System Maintenance",
		bits: []accessBitInfo{
			{hotline.AccessDeleteFile, "Delete Files", "Can Delete Files"},
			{hotline.AccessUploadFile, "Upload Files", "Can Upload Files"},
			{hotline.AccessDownloadFile, "Download Files", "Can Download Files"},
			{hotline.AccessRenameFile, "Rename Files", "Can Rename Files"},
			{hotline.AccessMoveFile, "Move Files", "Can Move Files"},
			{hotline.AccessCreateFolder, "Create Folders", "Can Create Folders"},
			{hotline.AccessDeleteFolder, "Delete Folders", "Can Delete Folders"},
			{hotline.AccessRenameFolder, "Rename Folders", "Can Rename Folders"},
			{hotline.AccessMoveFolder, "Move Folders", "Can Move Folders"},
			{hotline.AccessUploadAnywhere, "Upload Anywhere", "Can Upload Anywhere"},
			{hotline.AccessSetFileComment, "Comment Files", "Can Comment Files"},
			{hotline.AccessSetFolderComment, "Comment Folders", "Can Comment Folders"},
			{hotline.AccessViewDropBoxes, "View Drop Boxes", "Can View Drop Boxes"},
			{hotline.AccessMakeAlias, "Make Aliases", "Can Make Aliases"},
			{hotline.AccessUploadFolder, "Upload Folders", "Can Upload Folders"},
			{hotline.AccessDownloadFolder, "Download Folders", "Can Download Folders"},
		},
	},
	{
		category: "Chat",
		bits: []accessBitInfo{
			{hotline.AccessReadChat, "Read Chat", "Can Read Chat"},
			{hotline.AccessSendChat, "Send Chat", "Can Send Chat"},
			{hotline.AccessOpenChat, "Initiate Private Chat", "Can Initiate Private Chat"},
		},
	},
	{
		category: "User Maintenance",
		bits: []accessBitInfo{
			{hotline.AccessCreateUser, "Create Accounts", "Can Create Accounts"},
			{hotline.AccessDeleteUser, "Delete Accounts", "Can Delete Accounts"},
			{hotline.AccessOpenUser, "Read Accounts", "Can Read Accounts"},
			{hotline.AccessModifyUser, "Modify Accounts", "Can Modify Accounts"},
			{hotline.AccessDisconUser, "Disconnect Users", "Can Disconnect Users"},
			{hotline.AccessCannotBeDiscon, "Cannot Be Disconnected", "Cannot be Disconnected"},
			{hotline.AccessGetClientInfo, "Get User Info", "Can Get User Info"},
		},
	},
	{
		category: "News",
		bits: []accessBitInfo{
			{hotline.AccessNewsReadArt, "Read Articles", "Can Read Articles"},
			{hotline.AccessNewsPostArt, "Post Articles", "Can Post Articles"},
			{hotline.AccessNewsDeleteArt, "Delete Articles", "Can Delete Articles"},
			{hotline.AccessNewsCreateCat, "Create Categories", "Can Create Categories"},
			{hotline.AccessNewsDeleteCat, "Delete Categories", "Can Delete Categories"},
			{hotline.AccessNewsCreateFldr, "Create Bundles", "Can Create News Bundles"},
			{hotline.AccessNewsDeleteFldr, "Delete Bundles", "Can Delete News Bundles"},
		},
	},
	{
		category: "Messaging",
		bits: []accessBitInfo{
			{hotline.AccessBroadcast, "Broadcast", "Can Broadcast"},
			{hotline.AccessSendPrivMsg, "Send Messages", "Can Send Messages"},
		},
	},
	{
		category: "Miscellaneous",
		bits: []accessBitInfo{
			{hotline.AccessAnyName, "Use Any Name", "Can Use Any Name"},
			{hotline.AccessNoAgreement, "No Agreement", "Don't Show Agreement"},
		},
	},
}

// serverUIKeyMap defines key bindings for the server UI help display
type serverUIKeyMap struct {
	News         key.Binding
	MessageBoard key.Binding
	Files        key.Binding
	Logs         key.Binding
	Accounts     key.Binding
	Disconnect   key.Binding
	Send         key.Binding
}

func (k serverUIKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.MessageBoard, k.News, k.Files, k.Logs, k.Accounts, k.Disconnect}
}

func (k serverUIKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.MessageBoard, k.News, k.Files, k.Logs, k.Accounts, k.Disconnect},
	}
}

// viewportKeyMap defines key bindings for the logs viewport help display
type viewportKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Back     key.Binding
}

func (k viewportKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Back}
}

func (k viewportKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Back},
	}
}

// Home screen
func (m *Model) renderHome() string {
	// Banner styles
	hotkeyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	menu := strings.Join(
		[]string{
			fmt.Sprintf("%s Join Server", hotkeyStyle.Render("(j)")),
			fmt.Sprintf("%s Bookmarks", hotkeyStyle.Render("(b)")),
			fmt.Sprintf("%s Browse Tracker", hotkeyStyle.Render("(t)")),
			fmt.Sprintf("%s Settings", hotkeyStyle.Render("(s)")),
			fmt.Sprintf("%s Quit", hotkeyStyle.Render("(q)")),
		},
		"\n",
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render(m.welcomeBanner),
		menu,
	)

	spew.Fdump(os.Stderr, m.width, m.height, m.currentScreen)
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().Render(content),
		lipgloss.WithWhitespaceChars("猫咪"),
		lipgloss.WithWhitespaceForeground(subtle),
	)
}

func (m *Model) handleHomeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j":
		m.currentScreen = ScreenJoinServer
		m.backPage = ScreenHome
		m.editingBookmark = false
		m.creatingBookmark = false
		m.focusIndex = 0
		m.serverInput.Focus()
		return m, nil
	case "b":
		m.initializeBookmarkList()
		m.currentScreen = ScreenBookmarks
		return m, nil
	case "t":
		m.logger.Info("Browse tracker key pressed")
		return m, m.fetchTrackerList()
	case "s":
		m.previousScreen = m.currentScreen
		m.currentScreen = ScreenSettings
		m.focusIndex = 0
		m.usernameInput.Focus()
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// Join Server screen
func (m *Model) renderJoinServer() string {
	var b strings.Builder

	// Show different title based on mode
	if m.editingBookmark {
		b.WriteString(serverTitleStyle.Render("Edit Bookmark"))
	} else if m.creatingBookmark {
		b.WriteString(serverTitleStyle.Render("New Bookmark"))
	} else {
		b.WriteString(serverTitleStyle.Render("Connect to Server"))
	}
	b.WriteString("\n\n")

	var fields []string

	// Show name field when editing or creating bookmark
	if m.editingBookmark || m.creatingBookmark {
		fields = append(fields, fmt.Sprintf("Name: %s", m.nameInput.View()))
	}

	fields = append(fields, []string{
		fmt.Sprintf("Server: %s", m.serverInput.View()),
		fmt.Sprintf("Login: %s", m.loginInput.View()),
		fmt.Sprintf("Password: %s", m.passwordInput.View()),
	}...)

	// TLS checkbox
	tlsBox := "[ ]"
	if m.useTLS {
		tlsBox = "[x]"
	}
	fields = append(fields, fmt.Sprintf("TLS: %s", tlsBox))

	// Save bookmark checkbox (only show when not editing/creating)
	if !m.editingBookmark && !m.creatingBookmark {
		saveBox := "[ ]"
		if m.saveBookmark {
			saveBox = "[x]"
		}
		fields = append(fields, fmt.Sprintf("Save: %s", saveBox))
	}

	for i, field := range fields {
		if i == m.focusIndex {
			b.WriteString(selectedItemStyle.Render("> " + field))
		} else {
			b.WriteString("  " + field)
		}
		b.WriteString("\n")
	}

	// Show appropriate instructions based on mode
	if m.editingBookmark {
		b.WriteString("\n[Enter] Save Changes  [Tab] Next Field  [Esc] Cancel")
	} else if m.creatingBookmark {
		b.WriteString("\n[Enter] Create Bookmark  [Tab] Next Field  [Esc] Cancel")
	} else {
		b.WriteString("\n[Enter] Connect  [Tab] Next Field  [Esc] Cancel")
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(b.String()),
	)
}

func (m *Model) handleJoinServerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Calculate max focus index based on mode
	maxFocusIndex := 5 // Normal mode: server, login, password, TLS, Save
	if m.editingBookmark || m.creatingBookmark {
		maxFocusIndex = 5 // Edit/Create mode: name, server, login, password, TLS
	}

	switch msg.String() {
	case "tab":
		m.focusIndex = (m.focusIndex + 1) % maxFocusIndex
		m.updateJoinServerFocus()
		return m, nil

	case "shift+tab":
		m.focusIndex--
		if m.focusIndex < 0 {
			m.focusIndex = maxFocusIndex - 1
		}
		m.updateJoinServerFocus()
		return m, nil

	case " ":
		// Space toggles checkboxes, but should be passed to text inputs
		if m.editingBookmark || m.creatingBookmark {
			// In edit/create mode, TLS is at index 4
			if m.focusIndex == 4 {
				m.useTLS = !m.useTLS
				return m, nil
			}
		} else {
			// In normal mode, TLS is at index 3, Save is at index 4
			switch m.focusIndex {
			case 3:
				m.useTLS = !m.useTLS
				return m, nil
			case 4:
				m.saveBookmark = !m.saveBookmark
				return m, nil
			}
		}

	case "enter":
		name := m.nameInput.Value()
		addr := m.serverInput.Value()
		login := m.loginInput.Value()
		if login == "" {
			login = hotline.GuestAccount
		}
		password := m.passwordInput.Value()

		// Handle bookmark editing
		if m.editingBookmark {
			// Update existing bookmark
			if m.editingBookmarkIndex >= 0 && m.editingBookmarkIndex < len(m.allBookmarks) {
				m.allBookmarks[m.editingBookmarkIndex].Name = name
				m.allBookmarks[m.editingBookmarkIndex].Addr = addr
				m.allBookmarks[m.editingBookmarkIndex].Login = login
				m.allBookmarks[m.editingBookmarkIndex].Password = password
				m.allBookmarks[m.editingBookmarkIndex].TLS = m.useTLS
				m.prefs.Bookmarks = m.allBookmarks
				_ = m.savePreferences()
			}
			m.editingBookmark = false
			m.initializeBookmarkList()
			m.currentScreen = ScreenBookmarks
			return m, nil
		}

		// Handle bookmark creation
		if m.creatingBookmark {
			m.prefs.AddBookmark(name, addr, login, password, m.useTLS)
			_ = m.savePreferences()
			m.creatingBookmark = false
			m.initializeBookmarkList()
			m.currentScreen = ScreenBookmarks
			return m, nil
		}

		// Normal save bookmark behavior
		if m.saveBookmark {
			m.prefs.AddBookmark(name, addr, login, password, m.useTLS)
			_ = m.savePreferences()
		}

		// Store the address for display (used as fallback if no name was set)
		m.pendingServerAddr = addr
		// If no server name was set (e.g., connecting directly), use the address as the name
		if m.pendingServerName == "" {
			m.pendingServerName = addr
		}

		// Connect to server
		err := m.joinServer(addr, login, password, m.useTLS)
		if err != nil {
			m.modalTitle = "Connection Error"
			m.modalContent = err.Error()
			m.modalButtons = []string{"OK"}
			m.previousScreen = ScreenJoinServer
			m.currentScreen = ScreenModal
		}
		return m, nil
	}

	// Pass all other keys to the active text input
	return m, m.updateJoinServerForm(msg)
}

func (m *Model) updateJoinServerForm(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if m.editingBookmark || m.creatingBookmark {
		// In edit/create mode: name(0), server(1), login(2), password(3), TLS(4)
		switch m.focusIndex {
		case 0:
			m.nameInput, cmd = m.nameInput.Update(msg)
		case 1:
			m.serverInput, cmd = m.serverInput.Update(msg)
		case 2:
			m.loginInput, cmd = m.loginInput.Update(msg)
		case 3:
			m.passwordInput, cmd = m.passwordInput.Update(msg)
		}
	} else {
		// In normal mode: server(0), login(1), password(2), TLS(3), Save(4)
		switch m.focusIndex {
		case 0:
			m.serverInput, cmd = m.serverInput.Update(msg)
		case 1:
			m.loginInput, cmd = m.loginInput.Update(msg)
		case 2:
			m.passwordInput, cmd = m.passwordInput.Update(msg)
		}
	}
	return cmd
}

func (m *Model) updateJoinServerFocus() {
	m.nameInput.Blur()
	m.serverInput.Blur()
	m.loginInput.Blur()
	m.passwordInput.Blur()

	if m.editingBookmark || m.creatingBookmark {
		// In edit/create mode: name(0), server(1), login(2), password(3), TLS(4)
		switch m.focusIndex {
		case 0:
			m.nameInput.Focus()
		case 1:
			m.serverInput.Focus()
		case 2:
			m.loginInput.Focus()
		case 3:
			m.passwordInput.Focus()
		}
	} else {
		// In normal mode: server(0), login(1), password(2), TLS(3), Save(4)
		switch m.focusIndex {
		case 0:
			m.serverInput.Focus()
		case 1:
			m.loginInput.Focus()
		case 2:
			m.passwordInput.Focus()
		}
	}
}

// Bookmarks screen
type bookmarkItem struct {
	bookmark Bookmark
	index    int
}

func (i bookmarkItem) FilterValue() string {
	// Include both name and address for filtering
	return i.bookmark.Name + " " + i.bookmark.Addr
}
func (i bookmarkItem) Title() string       { return i.bookmark.Name }
func (i bookmarkItem) Description() string { return i.bookmark.Addr }

// newBookmarkDelegate creates a custom delegate for bookmark list items
func newBookmarkDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	// Customize the delegate's update function to handle delete
	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.String() == "x" {
				index := m.Index()
				if index >= 0 && index < len(m.Items()) {
					m.RemoveItem(index)
					if len(m.Items()) == 0 {
						return m.NewStatusMessage("All bookmarks deleted")
					}
					return m.NewStatusMessage("Deleted bookmark")
				}
			}
		}
		return nil
	}

	// Add custom help text for bookmark-specific keys
	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "choose"),
			),
			key.NewBinding(
				key.WithKeys("e"),
				key.WithHelp("e", "edit"),
			),
			key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "new"),
			),
			key.NewBinding(
				key.WithKeys("x"),
				key.WithHelp("x", "delete"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "choose"),
				),
				key.NewBinding(
					key.WithKeys("e"),
					key.WithHelp("e", "edit"),
				),
				key.NewBinding(
					key.WithKeys("n"),
					key.WithHelp("n", "new"),
				),
				key.NewBinding(
					key.WithKeys("/"),
					key.WithHelp("/", "filter"),
				),
			},
		}
	}

	return d
}

func (m *Model) initializeBookmarkList() {
	m.allBookmarks = m.prefs.Bookmarks
	items := make([]list.Item, len(m.allBookmarks))
	for i, bm := range m.allBookmarks {
		items[i] = bookmarkItem{bookmark: bm, index: i}
	}

	delegate := newBookmarkDelegate()

	// Calculate dimensions accounting for app style padding (1, 2)
	appStyle := lipgloss.NewStyle().Padding(1, 2)
	h, v := appStyle.GetFrameSize()

	m.bookmarkList = list.New(items, delegate, m.width-h, m.height-v)
	m.bookmarkList.Title = "Bookmarks"
	m.bookmarkList.SetFilteringEnabled(true) // Enable built-in filtering
	m.bookmarkList.SetShowStatusBar(true)    // Show status bar for delete messages
	m.bookmarkList.SetShowHelp(true)
	m.bookmarkList.DisableQuitKeybindings()
}

func (m *Model) renderBookmarks() string {
	// Use the same app style as the fancy list example: Padding(1, 2)
	appStyle := lipgloss.NewStyle().Padding(1, 2)
	return appStyle.Render(m.bookmarkList.View())
}

// Tracker screen
type trackerItem struct {
	server hotline.ServerRecord
	index  int
}

func (i trackerItem) FilterValue() string { return string(i.server.Name) }
func (i trackerItem) Title() string {
	name := string(i.server.Name)
	if i.server.TLSPort != [2]byte{0, 0} {
		name = "🔒 " + name
	}

	// Append user count if > 0
	numUsers := binary.BigEndian.Uint16(i.server.NumUsers[:])
	if numUsers > 0 {
		name += fmt.Sprintf(" (%d 🟢)", numUsers)
	}

	return name
}
func (i trackerItem) Description() string { return string(i.server.Description) }

// newTrackerDelegate creates a custom delegate for tracker list items
func newTrackerDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	// Add custom help text for tracker-specific keys
	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "choose"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "choose"),
				),
				key.NewBinding(
					key.WithKeys("/"),
					key.WithHelp("/", "filter"),
				),
			},
		}
	}

	return d
}

// Files screen
type fileItem struct {
	name     string
	isFolder bool
	size     uint32 // size in KB
	fileType [4]byte
	creator  [4]byte
	fileSize [4]byte
}

// fileTypeEmoji returns the appropriate emoji for a file type code
func fileTypeEmoji(typeCode [4]byte) string {
	tc := string(typeCode[:])

	switch tc {
	case "JPEG", "GIFf", "PNGf", "TIFF", "BMPf":
		return "🖼️" // Image files
	case "PDF ":
		return "📄" // PDF documents
	case "MooV", "MPEG":
		return "🎬" // Video files
	case "SIT!", "ZIP ", "Gzip":
		return "📦" // Archive files
	case "TEXT":
		return "📝" // Text files
	case "HTft":
		return "⏳" // Incomplete file uploads
	case "rohd":
		return "💾" // Disk images
	default:
		return "📄" // Default document emoji
	}
}

func (i fileItem) FilterValue() string { return i.name }
func (i fileItem) Title() string {
	if i.isFolder {
		return "📁 " + i.name
	}
	emoji := fileTypeEmoji(i.fileType)
	return emoji + " " + i.name
}
func (i fileItem) Description() string {
	if i.isFolder {
		return "Folder"
	}
	return fmt.Sprintf("%d KB", i.size)
}

// newFileDelegate creates a custom delegate for file list items
func newFileDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	// Add custom help text for file-specific keys
	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
			key.NewBinding(
				key.WithKeys("ctrl+u"),
				key.WithHelp("^u", "upload file"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "select"),
				),
				key.NewBinding(
					key.WithKeys("ctrl+u"),
					key.WithHelp("^u", "upload file"),
				),
			},
		}
	}

	return d
}

func (m *Model) initializeFileList(files []hotline.FileNameWithInfo) {
	var items []list.Item

	// Add "<- Back" option if we're in a subfolder
	if len(m.filePath) > 0 {
		items = append(items, fileItem{
			name:     "<- Back",
			isFolder: true,
			size:     0,
		})
	}

	// Add the files
	for _, f := range files {
		items = append(items, fileItem{
			name:     string(f.Name),
			isFolder: bytes.Equal(f.Type[:], []byte("fldr")),
			size:     binary.BigEndian.Uint32(f.FileSize[:]) / 1024,
			fileType: f.Type,
			creator:  f.Creator,
		})
	}

	delegate := newFileDelegate()

	// Calculate dimensions accounting for app style padding (1, 2)
	appStyle := lipgloss.NewStyle().Padding(1, 2)
	h, v := appStyle.GetFrameSize()

	m.fileList = list.New(items, delegate, m.width-h, m.height-v)
	m.fileList.Title = "Files"
	m.fileList.SetFilteringEnabled(true)
	m.fileList.SetShowStatusBar(true)
	m.fileList.SetShowHelp(true)
	m.fileList.DisableQuitKeybindings()
}

// newNewsDelegate creates a custom delegate for news list items
func newNewsDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.ShortHelpFunc = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
			key.NewBinding(
				key.WithKeys("space"),
				key.WithHelp("space", "expand/collapse"),
			),
			key.NewBinding(
				key.WithKeys("^P"),
				key.WithHelp("^P", "new article"),
			),
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "back"),
			),
		}
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{
			{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "select"),
				),
				key.NewBinding(
					key.WithKeys("space"),
					key.WithHelp("space", "expand/collapse"),
				),
				key.NewBinding(
					key.WithKeys("^P"),
					key.WithHelp("^P", "new article"),
				),
				key.NewBinding(
					key.WithKeys("esc"),
					key.WithHelp("esc", "back"),
				),
			},
		}
	}

	return d
}

func (m *Model) initializeNewsCategoryList(categories []newsItem) {
	var items []list.Item

	// Add the categories/bundles
	for _, cat := range categories {
		items = append(items, cat)
	}

	m.newsList = list.New(items, newNewsDelegate(), m.width, m.height)
	m.newsList.SetShowTitle(false)
	m.newsList.SetFilteringEnabled(true)
	m.newsList.SetShowStatusBar(true)
	m.newsList.SetShowHelp(true)
	m.newsList.DisableQuitKeybindings()
}

func (m *Model) initializeNewsArticleList(articles []newsArticleItem) {
	// Build thread tree and store articles
	articles = buildThreadTree(articles)
	m.allArticles = articles

	// Filter for visible articles
	visibleArticles := m.filterVisibleArticles(articles)

	var items []list.Item

	// Add the visible articles
	for _, art := range visibleArticles {
		art.isExpanded = m.expandedArticles[art.id]
		items = append(items, art)
	}

	// Calculate dimensions for split view (half height for list, half for article)
	m.newsList = list.New(items, newNewsDelegate(), m.width, m.height)

	// Build title with category name
	title := "Articles"
	if len(m.newsPath) > 0 {
		title += " - " + m.newsPath[len(m.newsPath)-1]
	}
	m.newsList.Title = title

	m.newsList.SetFilteringEnabled(false)
	m.newsList.SetShowStatusBar(false) // Disable status bar in split view for space
	m.newsList.SetShowHelp(true)       // Disable help in split view for space
	m.newsList.DisableQuitKeybindings()
}

func (m *Model) initializeTrackerList(servers []hotline.ServerRecord) {
	m.allTrackers = servers
	items := make([]list.Item, len(servers))
	for i, srv := range servers {
		items[i] = trackerItem{server: srv, index: i}
	}

	m.trackerList = list.New(items, newTrackerDelegate(), m.width, m.height)
	m.trackerList.Title = "Servers"
	m.trackerList.SetFilteringEnabled(true)
	m.trackerList.SetShowStatusBar(true)
	m.trackerList.SetShowHelp(true)
	m.trackerList.DisableQuitKeybindings()
}

func newAccountDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	return d
}

func (m *Model) initializeAccountList(accounts []accountItem) {
	m.allAccounts = accounts

	items := make([]list.Item, len(accounts))
	for i, acct := range accounts {
		items[i] = acct
	}

	m.accountList = list.New(items, newAccountDelegate(), m.width/2, m.height-10)
	m.accountList.Title = "User Accounts"
	m.accountList.SetFilteringEnabled(true)
	m.accountList.SetShowStatusBar(false)
	m.accountList.SetShowHelp(false)
	m.accountList.DisableQuitKeybindings()
}

func (m *Model) renderAccounts() string {
	if m.selectedAccount != nil || m.isNewAccount {
		return m.renderAccountsSplitView()
	}
	return m.renderAccountsListOnly()
}

func (m *Model) renderAccountsListOnly() string {
	content := m.accountList.View()

	// Add help text
	help := "\n\n"
	if m.userAccess.IsSet(hotline.AccessCreateUser) {
		help += "n: new account  "
	}
	help += "enter: view/edit  esc: close"

	return subScreenStyle.Render(content + help)
}

func (m *Model) renderAccountsSplitView() string {
	canEdit := m.userAccess.IsSet(hotline.AccessModifyUser) || m.isNewAccount

	// Determine border styles based on focus
	leftBorderStyle := lipgloss.NormalBorder()
	rightBorderStyle := lipgloss.NormalBorder()
	if m.accountDetailFocused {
		rightBorderStyle = lipgloss.DoubleBorder()
	} else {
		leftBorderStyle = lipgloss.DoubleBorder()
	}

	// Left pane: account list
	leftPane := lipgloss.NewStyle().
		Width(m.width / 2).
		Height(m.height - 10).
		BorderStyle(leftBorderStyle).
		BorderRight(true).
		Render(m.accountList.View())

	// Right pane: account details
	var rightContent strings.Builder

	if m.isNewAccount {
		rightContent.WriteString(titleStyle.Render("New Account"))
	} else {
		rightContent.WriteString(titleStyle.Render("Account: " + m.selectedAccount.login))
	}
	rightContent.WriteString("\n\n")

	// Account fields
	loginLabel := "Login: "
	if m.focusedAccessBit == focusLogin {
		loginLabel = "> " + loginLabel
	} else {
		loginLabel = "  " + loginLabel
	}
	rightContent.WriteString(loginLabel + m.editedLogin + "\n")

	nameLabel := "Name: "
	if m.focusedAccessBit == focusName {
		nameLabel = "> " + nameLabel
	} else {
		nameLabel = "  " + nameLabel
	}
	rightContent.WriteString(nameLabel + m.editedName + "\n")

	passLabel := "Password: "
	if m.focusedAccessBit == focusPass {
		passLabel = "> " + passLabel
	} else {
		passLabel = "  " + passLabel
	}
	passDisplay := m.editedPassword
	if len(passDisplay) == 0 {
		passDisplay = "(not set)"
	} else {
		passDisplay = strings.Repeat("*", len(passDisplay))
	}
	rightContent.WriteString(passLabel + passDisplay + "\n\n")

	// Access permissions by category
	focusIndex := 0
	for _, category := range accessBitsByCategory {
		rightContent.WriteString(categoryStyle.Render(category.category))
		rightContent.WriteString("\n")

		for _, bit := range category.bits {
			checkbox := "[ ]"
			if m.editedAccessBits.IsSet(bit.bit) {
				checkbox = "[x]"
			}

			prefix := "  "
			if focusIndex == m.focusedAccessBit {
				prefix = "> "
			}

			style := lipgloss.NewStyle()
			if !canEdit {
				style = style.Foreground(lipgloss.Color("240"))
			} else if focusIndex == m.focusedAccessBit {
				style = style.Bold(true)
			}

			rightContent.WriteString(style.Render(prefix + checkbox + " " + bit.name))
			rightContent.WriteString("\n")

			focusIndex++
		}
		rightContent.WriteString("\n")
	}

	// Set viewport content
	m.accountsViewport.SetContent(rightContent.String())

	// Help text
	helpText := "\n"
	if canEdit {
		helpText += "tab: toggle focus  ↑↓: navigate  space: toggle  enter: save  pgup/pgdn: scroll"
		if !m.isNewAccount && m.userAccess.IsSet(hotline.AccessDeleteUser) {
			helpText += "  ctrl+d: delete"
		}
		helpText += "  esc: cancel"
	} else {
		helpText += "tab: toggle focus  esc: close (read-only)"
	}

	// Render right pane with viewport and border
	rightPane := lipgloss.NewStyle().
		Width(m.width/2 - 2).
		Height(m.height - 10).
		BorderStyle(rightBorderStyle).
		BorderLeft(true).
		Padding(1).
		Render(m.accountsViewport.View())

	splitView := lipgloss.JoinHorizontal(
		lipgloss.Left,
		leftPane,
		rightPane,
	)

	return subScreenStyle.Render(splitView + helpText)
}

func (m *Model) renderTracker() string {

	return lipgloss.NewStyle().Padding(1, 2).Render(m.trackerList.View())
}

func (m *Model) fetchTrackerList() tea.Cmd {
	return func() tea.Msg {
		m.logger.Info("Fetching tracker list", "tracker", m.prefs.Tracker)
		conn, err := net.DialTimeout("tcp", m.prefs.Tracker, 5*time.Second)
		if err != nil {
			m.logger.Error("Error connecting to tracker", "err", err)
			return errorMsg{text: fmt.Sprintf("Error connecting to tracker:\n%v", err)}
		}
		defer func() {
			if err := conn.Close(); err != nil {
				m.logger.Error("Error closing tracker connection", "err", err)
			}
		}()

		// Set read deadline to prevent hanging
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			m.logger.Error("Error setting connection deadline", "err", err)
			return errorMsg{text: fmt.Sprintf("Error setting connection deadline:\n%v", err)}
		}

		m.logger.Info("Connected to tracker, fetching listing...")

		listing, err := hotline.GetListing(conn)
		if err != nil {
			m.logger.Error("Error fetching tracker results", "err", err)
			return errorMsg{text: fmt.Sprintf("Error fetching tracker results:\n%v", err)}
		}

		m.logger.Info("Tracker list fetched successfully", "count", len(listing))
		return trackerListMsg{servers: listing}
	}
}

// Settings screen

// renderSettingsOverlay renders the settings screen as a modal overlay
func (m *Model) renderSettingsOverlay() string {
	// Render settings modal content
	var b strings.Builder

	b.WriteString(serverTitleStyle.Render("Settings"))
	b.WriteString("\n\n")

	fields := []string{
		fmt.Sprintf("Your Name: %s", m.usernameInput.View()),
		fmt.Sprintf("Icon ID: %s", m.iconIDInput.View()),
		fmt.Sprintf("Tracker: %s", m.trackerInput.View()),
		fmt.Sprintf("Download Directory: %s", m.downloadDirInput.View()),
	}

	// Bell checkbox
	bellBox := "[ ]"
	if m.enableBell {
		bellBox = "[x]"
	}
	fields = append(fields, fmt.Sprintf("Enable Terminal Bell: %s", bellBox))

	// Sounds checkbox
	soundsBox := "[ ]"
	if m.enableSounds {
		soundsBox = "[x]"
	}
	fields = append(fields, fmt.Sprintf("Enable Sounds: %s", soundsBox))

	for i, field := range fields {
		if i == m.focusIndex {
			b.WriteString(selectedItemStyle.Render("> " + field))
		} else {
			b.WriteString("  " + field)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n[Enter] Save  [Tab] Next Field  [Esc] Cancel")

	// Create modal box with enhanced styling for overlay
	// Use a darker background and distinct border to create modal effect
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("62")).
		Background(lipgloss.Color("235")).
		Padding(1, 2)

	modalBox := modalStyle.Render(b.String())

	// Place modal centered with a dark background to create modal overlay effect
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalBox,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("0")), // Black background
	)
}

func (m *Model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.focusIndex = (m.focusIndex + 1) % 6
		m.updateSettingsFocus()
		return m, nil

	case "shift+tab":
		m.focusIndex--
		if m.focusIndex < 0 {
			m.focusIndex = 5
		}
		m.updateSettingsFocus()
		return m, nil

	case " ":
		if m.focusIndex == 4 {
			m.enableBell = !m.enableBell
			return m, nil
		}
		if m.focusIndex == 5 {
			m.enableSounds = !m.enableSounds
			return m, nil
		}

	case "enter":
		// Save settings
		m.prefs.Username = m.usernameInput.Value()
		if iconID, err := strconv.Atoi(m.iconIDInput.Value()); err == nil {
			m.prefs.IconID = iconID
		}
		m.prefs.Tracker = m.trackerInput.Value()
		m.prefs.DownloadDir = m.downloadDirInput.Value()
		m.prefs.EnableBell = m.enableBell
		m.prefs.EnableSounds = m.enableSounds

		// Update the active download directory
		m.downloadDir = m.prefs.DownloadDir

		// Update sound player enabled state
		if m.soundPlayer != nil {
			m.soundPlayer.SetEnabled(m.prefs.EnableSounds)
		}

		err := m.savePreferences()
		if err != nil {
			m.logger.Error("Failed to save preferences", "err", err)
		}

		m.currentScreen = ScreenHome
		return m, nil
	}

	// Pass all other keys to the active text input
	return m, m.updateSettingsForm(msg)
}

func (m *Model) updateSettingsForm(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.usernameInput, cmd = m.usernameInput.Update(msg)
	case 1:
		m.iconIDInput, cmd = m.iconIDInput.Update(msg)
	case 2:
		m.trackerInput, cmd = m.trackerInput.Update(msg)
	case 3:
		m.downloadDirInput, cmd = m.downloadDirInput.Update(msg)
	}
	return cmd
}

func (m *Model) updateSettingsFocus() {
	m.usernameInput.Blur()
	m.iconIDInput.Blur()
	m.trackerInput.Blur()
	m.downloadDirInput.Blur()

	switch m.focusIndex {
	case 0:
		m.usernameInput.Focus()
	case 1:
		m.iconIDInput.Focus()
	case 2:
		m.trackerInput.Focus()
	case 3:
		m.downloadDirInput.Focus()
	}
}

// Server UI screen
func (m *Model) renderServerUI() string {
	// Shortcuts
	shortcuts := m.serverUIHelp.View(m.serverUIKeys)

	// User list
	var userListContent strings.Builder
	for i, u := range m.userList {
		flags := binary.BigEndian.Uint16(u.Flags)
		userName := u.Name

		// Highlight selected user when user list is focused
		if m.focusOnUserList && i == m.selectedUserIdx {
			userName = "> " + userName
		} else {
			userName = "  " + userName
		}

		// Check both admin and away flags
		isAdmin := (flags & (1 << hotline.UserFlagAdmin)) != 0
		isAway := (flags & (1 << hotline.UserFlagAway)) != 0

		if isAdmin && isAway {
			userListContent.WriteString(awayAdminUserStyle.Render(userName))
		} else if isAdmin {
			userListContent.WriteString(adminUserStyle.Render(userName))
		} else if isAway {
			userListContent.WriteString(awayUserStyle.Render(userName))
		} else {
			userListContent.WriteString(userName)
		}
		userListContent.WriteString("\n")
	}
	m.userViewport.SetContent(userListContent.String())

	// Chat area - use double border when focused, grey border when scrolled up
	chatBorder := lipgloss.RoundedBorder()
	chatBorderColor := lipgloss.Color("63") // Default cyan

	if !m.focusOnUserList {
		chatBorder = lipgloss.DoubleBorder()

		// Change to grey when in scrollback mode (not at bottom)
		if !m.chatViewport.AtBottom() {
			chatBorderColor = lipgloss.Color("245") // Light grey
		}
	}

	chatView := lipgloss.NewStyle().
		PaddingLeft(1).
		Border(chatBorder).
		BorderForeground(chatBorderColor).
		Render(m.chatViewport.View())

	// User list area - use double border when focused
	userBorder := lipgloss.RoundedBorder()
	if m.focusOnUserList {
		userBorder = lipgloss.DoubleBorder()
	}
	userView := lipgloss.NewStyle().
		Border(userBorder).
		BorderForeground(lipgloss.Color("63")).
		Render(m.userViewport.View())

	// Main content area
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, chatView, boxStyle.Render(m.chatInput.View())),
		lipgloss.JoinVertical(lipgloss.Left, userView, m.renderTaskWidget()),
	)

	// Title
	title := serverTitleStyle.Render(fmt.Sprintf("Mobius - Connected to %s", m.serverName))

	// Compose full view
	serverView := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		shortcuts,
		mainContent,
	)

	// If news modal is active, show it as overlay
	if m.showNewsModal {
		return m.renderNews()
	}

	return serverView
}

func (m *Model) renderTaskWidget() string {
	// Get active tasks first, then completed to fill remaining slots
	activeTasks := m.taskManager.GetActive()
	completedTasks := m.taskManager.GetCompleted(3)

	// Combine: active tasks first, then completed
	var displayTasks []*Task
	displayTasks = append(displayTasks, activeTasks...)

	// Fill remaining slots with completed tasks
	remaining := 3 - len(activeTasks)
	if remaining > 0 && len(completedTasks) > 0 {
		for i := 0; i < remaining && i < len(completedTasks); i++ {
			displayTasks = append(displayTasks, completedTasks[i])
		}
	}

	// Limit to 3 tasks maximum
	if len(displayTasks) > 3 {
		displayTasks = displayTasks[:3]
	}

	// Build widget content
	var content strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Render("Recent Transfers")
	content.WriteString(title + "\n")

	if len(displayTasks) == 0 {
		// Empty state
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No transfers")
		content.WriteString(emptyMsg)
	} else {
		// Render each task
		for i, task := range displayTasks {
			if i > 0 {
				content.WriteString("\n") // Spacing between tasks
			}
			content.WriteString(m.renderCompactTask(task))
		}
	}

	return taskWidgetStyle.Render(content.String())
}

func (m *Model) renderCompactTask(task *Task) string {
	var lines []string

	// Line 1: Filename (truncated) + status/percentage
	fileName := task.FileName
	if len(fileName) > 18 {
		fileName = fileName[:15] + "..."
	}

	var statusStr string
	switch task.Status {
	case TaskActive:
		percent := 0
		if task.TotalBytes > 0 {
			percent = int((float64(task.TransferredBytes) / float64(task.TotalBytes)) * 100)
		}
		statusStr = taskActiveStyle.Render(fmt.Sprintf("%3d%%", percent))
	case TaskCompleted:
		statusStr = taskCompleteStyle.Render("Done")
	case TaskFailed:
		statusStr = taskFailedStyle.Render("Fail")
	case TaskPending:
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Wait")
	}

	line1 := fmt.Sprintf("%-18s %4s", fileName, statusStr)
	lines = append(lines, line1)

	// Line 2: Progress bar (only for active tasks)
	if task.Status == TaskActive {
		if prog, ok := m.taskProgress[task.ID]; ok {
			lines = append(lines, prog.View())
		}
	}

	// Line 3: Speed (only for active tasks)
	if task.Status == TaskActive && task.Speed > 0 {
		speedStr := formatSpeed(task.Speed)
		lines = append(lines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(speedStr))
	}

	return strings.Join(lines, "\n")
}

// Helper function to format speed
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	}
}

func (m *Model) handleServerUIKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Toggle focus between chat input and user list
		m.focusOnUserList = !m.focusOnUserList
		if m.focusOnUserList {
			// Blur chat input when switching to user list
			m.chatInput.Blur()
		} else {
			// Focus chat input when switching back
			m.chatInput.Focus()
		}
		return m, nil

	case "ctrl+n":
		// Request threaded news - reset path to root
		m.newsPath = []string{}
		m.selectedArticle = nil
		m.isViewingCategory = false // At root, viewing bundles/categories
		if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetNewsCatNameList, [2]byte{})); err != nil {
			m.logger.Error("Error requesting news categories", "err", err)
		}
		return m, nil

	case "ctrl+b":
		// Request messageboard
		if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetMsgs, [2]byte{})); err != nil {
			m.logger.Error("Error requesting messageboard", "err", err)
		}
		return m, nil

	case "ctrl+f":
		// List files - reset path to root
		m.filePath = []string{}
		if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetFileNameList, [2]byte{})); err != nil {
			m.logger.Error("Error requesting files", "err", err)
		}
		return m, nil

	case "ctrl+a":
		// Request user accounts list
		if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranListUsers, [2]byte{})); err != nil {
			m.logger.Error("Error requesting account list", "err", err)
		}

		return m, nil

	case "up":
		if m.focusOnUserList && m.selectedUserIdx > 0 {
			m.selectedUserIdx--
		} else if !m.focusOnUserList {
			// Scroll chat viewport up when chat input is focused
			m.chatViewport.ScrollUp(1)
		}
		return m, nil

	case "down":
		if m.focusOnUserList && m.selectedUserIdx < len(m.userList)-1 {
			m.selectedUserIdx++
		} else if !m.focusOnUserList {
			// Scroll chat viewport down when chat input is focused
			m.chatViewport.ScrollDown(1)
		}
		return m, nil

	case "pgup":
		if !m.focusOnUserList {
			// Page up in chat viewport
			m.chatViewport.PageUp()
		}
		return m, nil

	case "pgdown":
		if !m.focusOnUserList {
			// Page down in chat viewport
			m.chatViewport.PageDown()
		}
		return m, nil

	case "home":
		if !m.focusOnUserList {
			// Jump to top of chat
			m.chatViewport.GotoTop()
		}
		return m, nil

	case "end":
		if !m.focusOnUserList {
			// Jump to bottom of chat
			m.chatViewport.GotoBottom()
		}
		return m, nil

	case "enter":
		if m.focusOnUserList {
			// Open compose message modal for selected user
			if m.selectedUserIdx >= 0 && m.selectedUserIdx < len(m.userList) {
				m.composeMsgTarget = m.userList[m.selectedUserIdx].ID
				m.composeMsgQuote = "" // No quote when composing new message
				m.composeMsgInput.SetValue("")
				m.composeMsgModal = true
				return m, nil
			}
		} else {
			// Send chat message
			text := m.chatInput.Value()
			if text != "" {
				t := hotline.NewTransaction(hotline.TranChatSend, [2]byte{},
					hotline.NewField(hotline.FieldData, []byte(text)),
				)
				_ = m.hlClient.Send(t)
				m.chatInput.SetValue("")
			}
		}
		return m, nil
	}

	// Pass all other keys to the chat input if it's focused
	if !m.focusOnUserList {
		var cmd tea.Cmd
		m.chatInput, cmd = m.chatInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// News screen - renders as centered modal overlay (threaded news)
func (m *Model) renderNews() string {
	// Check if we have an article selected for split view
	if m.selectedArticle != nil {
		return m.renderNewsSplitView()
	}

	// Otherwise, render full modal with list only
	return m.renderNewsListOnly()
}

func (m *Model) renderNewsListOnly() string {
	// Set news list dimensions
	m.newsList.SetSize(m.width-10, m.height-10)

	newsHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170"))

	// Place modal centered with dim gray background
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		subScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				newsHeaderStyle.Render("News"),
				m.newsList.View(),
			),
		),

		lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
	)
}

func (m *Model) renderNewsSplitView() string {
	// Calculate available width for article content
	const borderWidth = 2
	const padding = 2 // Padding from Padding(0, 1) on both sides

	articleWidth := (m.width - 10) / 2 // Half of modal width
	wrapWidth := articleWidth - borderWidth - padding
	if wrapWidth < 20 {
		wrapWidth = 20 // Minimum for edge cases
	}

	// Wrap article content to fit width
	wrappedContent := wordwrap.String(m.selectedArticle.content, wrapWidth)

	m.articleViewport.SetContent(wrappedContent)

	// Update list dimensions for top half

	m.newsList.SetSize(m.width-10, m.height-10)

	// Build top half (article list)
	articleList := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("62")).
		Render(m.newsList.View())

	// Build bottom half (article content)
	var articleHeader strings.Builder
	articleHeader.WriteString(lipgloss.NewStyle().Bold(true).Render(m.selectedArticle.title))
	articleHeader.WriteString("\n")

	posterInfo := m.selectedArticle.poster

	timestamp := hotline.Time(m.selectedArticle.date).Format("Jan 2, 2006 at 3:04 PM")
	posterInfo += " - " + timestamp

	articleHeader.WriteString(lipgloss.NewStyle().Faint(true).Render(posterInfo))
	articleHeader.WriteString("\n\n")

	renderedArticle := lipgloss.NewStyle().
		Padding(0, 1).
		Render(articleHeader.String() + wrappedContent)

	// Combine top and bottom
	splitView := lipgloss.JoinHorizontal(
		lipgloss.Left,
		articleList,
		renderedArticle,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		subScreenStyle.Render(
			splitView,
		),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
	)

}

// scrollToFocusedCheckbox scrolls the accounts viewport to keep the focused item visible
func (m *Model) scrollToFocusedCheckbox() {
	// Calculate line position of focused item
	// Account for header (3 lines), Login/Name/Password fields (5 lines including blank)
	const headerLines = 3
	const accountFieldLines = 5

	// Count lines up to focused item
	linePos := headerLines + accountFieldLines

	// If focused on checkbox (0-40), calculate its position
	if m.focusedAccessBit < 41 {
		// Count through categories to find line position
		currentBit := 0
		for _, category := range accessBitsByCategory {
			linePos++ // Category header line
			for range category.bits {
				if currentBit == m.focusedAccessBit {
					// Center the focused item in viewport
					centerOffset := m.accountsViewport.Height / 2
					targetYOffset := linePos - centerOffset
					if targetYOffset < 0 {
						targetYOffset = 0
					}
					m.accountsViewport.SetYOffset(targetYOffset)
					return
				}
				linePos++ // Checkbox line
				currentBit++
			}
			linePos++ // Blank line after category
		}
	} else {
		// Focused on text fields at top - scroll to top
		m.accountsViewport.SetYOffset(0)
	}
}

func (m *Model) handleAccountsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// List-only view
	if m.selectedAccount == nil && !m.isNewAccount {
		switch msg.String() {
		case "n":
			if m.userAccess.IsSet(hotline.AccessCreateUser) {
				m.isNewAccount = true
				m.editedLogin = ""
				m.editedName = ""
				m.editedPassword = ""
				m.editedAccessBits = hotline.AccessBitmap{}
				m.focusedAccessBit = focusLogin
				return m, nil
			}
		case "enter":
			if item, ok := m.accountList.SelectedItem().(accountItem); ok {
				m.selectedAccount = &selectedAccountData{
					login:          item.login,
					name:           item.name,
					originalAccess: item.access,
					hasPassword:    item.hasPass,
				}
				m.editedLogin = item.login
				m.editedName = item.name
				m.editedPassword = ""
				m.editedAccessBits = item.access
				m.passwordChanged = false
				m.focusedAccessBit = 0
				return m, nil
			}
		case "esc":
			m.currentScreen = ScreenServerUI
			return m, nil
		default:
			var cmd tea.Cmd
			m.accountList, cmd = m.accountList.Update(msg)
			return m, cmd
		}
	}

	// Split view (editing account)
	canEdit := m.userAccess.IsSet(hotline.AccessModifyUser) || m.isNewAccount

	switch msg.String() {
	case "tab":
		// Toggle focus between list and detail panes
		m.accountDetailFocused = !m.accountDetailFocused
		return m, nil

	case "esc":
		m.selectedAccount = nil
		m.isNewAccount = false
		m.accountDetailFocused = false // Reset focus
		return m, nil

	case "up":
		// Route based on focus
		if !m.accountDetailFocused {
			// List focused - pass to list
			var cmd tea.Cmd
			m.accountList, cmd = m.accountList.Update(msg)
			return m, cmd
		}
		// Detail focused - navigate checkboxes/fields
		if canEdit && m.focusedAccessBit > 0 {
			m.focusedAccessBit--
			m.scrollToFocusedCheckbox()
		}
		return m, nil

	case "down":
		// Route based on focus
		if !m.accountDetailFocused {
			// List focused - pass to list
			var cmd tea.Cmd
			m.accountList, cmd = m.accountList.Update(msg)
			return m, cmd
		}
		// Detail focused - navigate checkboxes/fields
		if canEdit && m.focusedAccessBit < focusPass {
			m.focusedAccessBit++
			m.scrollToFocusedCheckbox()
		}
		return m, nil

	case "pgup":
		// Manual viewport scrolling when detail pane focused
		if m.accountDetailFocused {
			m.accountsViewport.ViewUp()
		}
		return m, nil

	case "pgdown":
		// Manual viewport scrolling when detail pane focused
		if m.accountDetailFocused {
			m.accountsViewport.ViewDown()
		}
		return m, nil

	case " ", "space":
		if canEdit && m.accountDetailFocused {
			// Map focus index to actual access bit
			focusIndex := 0
			for _, category := range accessBitsByCategory {
				for _, bit := range category.bits {
					if focusIndex == m.focusedAccessBit {
						// Toggle the checkbox
						if m.editedAccessBits.IsSet(bit.bit) {
							m.editedAccessBits[bit.bit/8] &^= 1 << uint(7-bit.bit%8)
						} else {
							m.editedAccessBits.Set(bit.bit)
						}
						return m, nil
					}
					focusIndex++
				}
			}
		}
		return m, nil

	case "enter":
		// If list focused, select account
		if !m.accountDetailFocused {
			if item, ok := m.accountList.SelectedItem().(accountItem); ok {
				m.selectedAccount = &selectedAccountData{
					login:          item.login,
					name:           item.name,
					originalAccess: item.access,
					hasPassword:    item.hasPass,
				}
				m.editedLogin = item.login
				m.editedName = item.name
				m.editedPassword = ""
				m.editedAccessBits = item.access
				m.passwordChanged = false
				m.focusedAccessBit = 0
				m.accountDetailFocused = true // Switch to detail pane
				return m, nil
			}
		}
		// If detail focused and can edit, submit changes
		if canEdit && m.accountDetailFocused {
			return m, m.submitAccountChanges()
		}
		return m, nil

	case "ctrl+d":
		if !m.isNewAccount && m.userAccess.IsSet(hotline.AccessDeleteUser) && m.accountDetailFocused {
			return m, m.deleteAccount()
		}
		return m, nil

	default:
		// Handle text input for focused fields (only when detail pane focused)
		if canEdit && m.accountDetailFocused {
			switch m.focusedAccessBit {
			case focusLogin:
				if msg.Type == tea.KeyRunes {
					m.editedLogin += string(msg.Runes)
				} else if msg.Type == tea.KeyBackspace && len(m.editedLogin) > 0 {
					m.editedLogin = m.editedLogin[:len(m.editedLogin)-1]
				}
			case focusName:
				if msg.Type == tea.KeyRunes {
					m.editedName += string(msg.Runes)
				} else if msg.Type == tea.KeyBackspace && len(m.editedName) > 0 {
					m.editedName = m.editedName[:len(m.editedName)-1]
				}
			case focusPass:
				if msg.Type == tea.KeyRunes {
					m.editedPassword += string(msg.Runes)
					m.passwordChanged = true
				} else if msg.Type == tea.KeyBackspace && len(m.editedPassword) > 0 {
					m.editedPassword = m.editedPassword[:len(m.editedPassword)-1]
					m.passwordChanged = true
				}
			}
		}
	}

	return m, nil
}

func (m *Model) handleNewsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// If an article is selected, deselect it first
		if m.selectedArticle != nil {
			m.selectedArticle = nil
			return m, nil
		}

		// Check if we're viewing articles (not categories)
		items := m.newsList.Items()
		isViewingArticles := false
		if len(items) > 0 {
			// Check first item - if it's a newsArticleItem, we're viewing articles
			// (The "<- Back" is at index 0 as a newsItem, articles start at index 1)
			for _, item := range items {
				if _, ok := item.(newsArticleItem); ok {
					isViewingArticles = true
					break
				}
			}
		}

		// If viewing articles and not at root, go back one level
		if isViewingArticles && len(m.newsPath) > 0 {
			// Go up one level
			m.newsPath = m.newsPath[:len(m.newsPath)-1]

			// Request categories for parent level
			var fields []hotline.Field
			if len(m.newsPath) > 0 {
				pathBytes := encodeNewsPath(m.newsPath)
				fields = append(fields, hotline.NewField(hotline.FieldNewsPath, pathBytes))
			}

			if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetNewsCatNameList, [2]byte{}, fields...)); err != nil {
				m.logger.Error("Error requesting news categories", "err", err)
			}
			return m, nil
		}

		// Otherwise close the news modal
		m.showNewsModal = false
		m.newsPath = []string{}
		m.isViewingCategory = false // Reset state
		m.selectedArticle = nil
		return m, nil

	case "ctrl+p":
		// Only allow posting when viewing a category (not bundles/root)
		if m.isViewingCategory {
			cmd := m.initNewsArticlePostForm("", 0) // New post with empty subject and parent ID 0
			m.currentScreen = ScreenNewsPost
			return m, cmd
		}
		return m, nil

	case "ctrl+r":
		// Only allow replying when viewing an article
		if m.selectedArticle == nil {
			return m, nil
		}

		// Create subject with "Re: " prefix
		subject := m.selectedArticle.title
		if !strings.HasPrefix(subject, "Re: ") {
			subject = "Re: " + subject
		}

		cmd := m.initNewsArticlePostForm(subject, m.selectedArticle.id)
		m.currentScreen = ScreenNewsPost
		return m, cmd

	case "ctrl+b":
		// Only allow creating bundles when viewing bundle/root (not categories)
		if !m.isViewingCategory {
			cmd := m.initNewsBundleForm()
			m.currentScreen = ScreenNewsPost
			return m, cmd
		}
		return m, nil

	case "ctrl+c":
		// Only allow creating categories when viewing bundle/root (not articles)
		if !m.isViewingCategory {
			cmd := m.initNewsCategoryForm()
			m.currentScreen = ScreenNewsPost
			return m, cmd
		}
		return m, nil

	case " ":
		// Toggle expand/collapse for articles with children
		selectedItem := m.newsList.SelectedItem()

		if article, ok := selectedItem.(newsArticleItem); ok {
			if article.hasChildren {
				if m.expandedArticles[article.id] {
					delete(m.expandedArticles, article.id)
				} else {
					m.expandedArticles[article.id] = true
				}

				m.refreshNewsArticleList()
			}
		}
		return m, nil

	case "pgup", "pgdown", "up", "down":
		// If we have an article selected, handle viewport scrolling
		if m.selectedArticle != nil {
			switch msg.String() {
			case "pgup":
				m.articleViewport.PageUp()
				return m, nil
			case "pgdown":
				m.articleViewport.PageDown()
				return m, nil
			}
		}
		// Otherwise pass to list
		var cmd tea.Cmd
		m.newsList, cmd = m.newsList.Update(msg)
		return m, cmd

	case "enter":
		// Get selected item
		selectedItem := m.newsList.SelectedItem()

		// Handle newsItem (category/bundle)
		if item, ok := selectedItem.(newsItem); ok {
			// Handle "<- Back" option
			if item.name == "<- Back" {
				// Go up one level
				if len(m.newsPath) > 0 {
					m.newsPath = m.newsPath[:len(m.newsPath)-1]
				}
				m.selectedArticle = nil

				// If we're back at root or in a bundle, request categories
				var fields []hotline.Field
				if len(m.newsPath) > 0 {
					pathBytes := encodeNewsPath(m.newsPath)
					fields = append(fields, hotline.NewField(hotline.FieldNewsPath, pathBytes))
				}

				// Request categories when going back
				if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetNewsCatNameList, [2]byte{}, fields...)); err != nil {
					m.logger.Error("Error requesting news categories", "err", err)
				}
				return m, nil
			}

			// Navigate into bundle or category
			m.newsPath = append(m.newsPath, item.name)
			m.selectedArticle = nil

			pathBytes := encodeNewsPath(m.newsPath)

			if item.isBundle {
				// Request sub-categories for bundle
				if err := m.hlClient.Send(hotline.NewTransaction(
					hotline.TranGetNewsCatNameList,
					[2]byte{},
					hotline.NewField(hotline.FieldNewsPath, pathBytes),
				)); err != nil {
					m.logger.Error("Error requesting news categories", "err", err)
				}
			} else {
				// Request articles for category
				if err := m.hlClient.Send(hotline.NewTransaction(
					hotline.TranGetNewsArtNameList,
					[2]byte{},
					hotline.NewField(hotline.FieldNewsPath, pathBytes),
				)); err != nil {
					m.logger.Error("Error requesting news articles", "err", err)
				}
			}
			return m, nil
		}

		// Handle newsArticleItem (article)
		if item, ok := selectedItem.(newsArticleItem); ok {
			// Auto-expand parent chain if this is a child article
			if item.parentID != 0 {
				// Walk up parent chain and expand all parents
				parentMap := make(map[uint32]uint32)
				for _, art := range m.allArticles {
					parentMap[art.id] = art.parentID
				}

				currentID := item.parentID
				for currentID != 0 {
					m.expandedArticles[currentID] = true
					currentID = parentMap[currentID]
				}

				// Refresh the list to show the expanded chain
				m.refreshNewsArticleList()
			}

			// Request full article data
			pathBytes := encodeNewsPath(m.newsPath)

			// Store article ID for when response arrives
			m.pendingArticleID = item.id

			articleIDBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(articleIDBytes, item.id)

			if err := m.hlClient.Send(hotline.NewTransaction(
				hotline.TranGetNewsArtData,
				[2]byte{},
				hotline.NewField(hotline.FieldNewsPath, pathBytes),
				hotline.NewField(hotline.FieldNewsArtID, articleIDBytes),
			)); err != nil {
				m.logger.Error("Error requesting article data", "err", err)
			}
			return m, nil
		}

		return m, nil
	}

	// Pass other keys to the list
	var cmd tea.Cmd
	m.newsList, cmd = m.newsList.Update(msg)
	return m, cmd
}

// encodeNewsPath encodes a news path into bytes for the FieldNewsPath field
func encodeNewsPath(path []string) []byte {
	if len(path) == 0 {
		return []byte{}
	}

	// Format: [2 bytes count][1 byte len][name]...
	var buf []byte
	countBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(countBytes, uint16(len(path)))
	buf = append(buf, countBytes...)

	for _, name := range path {
		buf = append(buf, 0, 0)
		// Add length byte
		buf = append(buf, byte(len(name)))
		// Add name
		buf = append(buf, []byte(name)...)
	}

	return buf
}

// News Post screen

// initNewsPostForm creates a new huh form for posting news
func (m *Model) initNewsPostForm() tea.Cmd {
	m.newsPostText = ""
	m.newsPostConfirm = false

	// Create form with explicit configuration
	m.newsPostForm = huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Key("newsPost").
				//Title("New Messageboard Post").
				CharLimit(1000),

			huh.NewConfirm().
				Key("done").
				Value(func() *bool { t := true; return &t }()).
				Affirmative("Post").
				Negative("Cancel"),
		),
	).
		WithWidth(45).
		WithShowHelp(false).
		WithShowErrors(false)

	// Initialize form and return command
	return m.newsPostForm.Init()
}

// initNewsArticlePostForm creates a Huh form for posting threaded news articles
func (m *Model) initNewsArticlePostForm(prefillSubject string, parentArticleID uint32) tea.Cmd {
	// Store parent article ID for submission
	m.replyParentArticleID = parentArticleID

	subject := prefillSubject

	m.newsArticlePostForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("subject").
				Title("Subject").
				Placeholder("Enter article subject").
				Value(&subject). // Pre-fill subject if provided
				CharLimit(255).
				Validate(func(s string) error {
					if len(strings.TrimSpace(s)) == 0 {
						return fmt.Errorf("subject cannot be empty")
					}
					return nil
				}),

			huh.NewText().
				Key("body").
				Title("Body").
				CharLimit(4000).
				Validate(func(s string) error {
					if len(strings.TrimSpace(s)) == 0 {
						return fmt.Errorf("body cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Post this article?").
				Affirmative("Post").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true)

	return m.newsArticlePostForm.Init()
}

// initNewsBundleForm creates a Huh form for creating a new News Bundle
func (m *Model) initNewsBundleForm() tea.Cmd {
	bundleName := ""

	m.newsBundleForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("bundleName").
				Title("Bundle Name").
				Placeholder("Enter bundle name").
				Value(&bundleName).
				CharLimit(255).
				Validate(func(s string) error {
					if len(strings.TrimSpace(s)) == 0 {
						return fmt.Errorf("bundle name cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Create this bundle?").
				Affirmative("Create").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true)

	return m.newsBundleForm.Init()
}

// initNewsCategoryForm creates a Huh form for creating a new News Category
func (m *Model) initNewsCategoryForm() tea.Cmd {
	categoryName := ""

	m.newsCategoryForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("categoryName").
				Title("Category Name").
				Placeholder("Enter category name").
				Value(&categoryName).
				CharLimit(255).
				Validate(func(s string) error {
					if len(strings.TrimSpace(s)) == 0 {
						return fmt.Errorf("category name cannot be empty")
					}
					return nil
				}),

			huh.NewConfirm().
				Key("confirm").
				Title("Create this category?").
				Affirmative("Create").
				Negative("Cancel"),
		),
	).
		WithWidth(60).
		WithShowHelp(true).
		WithShowErrors(true)

	return m.newsCategoryForm.Init()
}

func (m *Model) renderNewMsgBoardPost() string {
	var formView string
	var title string

	if m.newsArticlePostForm != nil {
		formView = m.newsArticlePostForm.View()
		title = "New Article"
	} else if m.newsBundleForm != nil {
		formView = m.newsBundleForm.View()
		title = "New News Bundle"
	} else if m.newsCategoryForm != nil {
		formView = m.newsCategoryForm.View()
		title = "New News Category"
	} else if m.newsPostForm != nil {
		formView = m.newsPostForm.View()
		title = "New Messageboard Post"
	} else {
		formView = "No form loaded"
		title = "Error"
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		subScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				subTitleStyle.Render(title),
				formView,
			),
		),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
	)
}

// Files screen - renders as full screen view
func (m *Model) renderFiles() string {
	m.fileList.SetSize(m.width-10, m.height-10)
	// Wrap with app style padding, matching the bookmark/tracker screens

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		subScreenStyle.Render(m.fileList.View()),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
	)
	//return subScreenStyle.Render(m.fileList.View())
}

func (m *Model) handleFilesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentScreen = m.previousScreen
		m.filePath = []string{} // Reset file path when closing
		return m, nil

	case "ctrl+u":
		// Show file picker for upload (restore last location)
		m.showFilePicker = true
		m.filePicker.CurrentDirectory = m.lastPickerLocation
		return m, m.filePicker.Init()

	case "enter":
		// Get selected file
		if item, ok := m.fileList.SelectedItem().(fileItem); ok {
			// Handle "<- Back" option
			if item.name == "<- Back" {
				// Go up one level
				if len(m.filePath) > 0 {
					m.filePath = m.filePath[:len(m.filePath)-1]
				}
				// Request new file list for parent directory
				f := hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(strings.Join(m.filePath, "/")))
				if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetFileNameList, [2]byte{}, f)); err != nil {
					m.logger.Error("Error requesting file list", "err", err)
				}
				return m, nil
			}

			// Handle folder navigation
			if item.isFolder {
				m.logger.Info("Navigating to folder", "name", item.name)
				m.filePath = append(m.filePath, item.name)
				// Request new file list for this folder
				f := hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(strings.Join(m.filePath, "/")))
				if err := m.hlClient.Send(hotline.NewTransaction(hotline.TranGetFileNameList, [2]byte{}, f)); err != nil {
					m.logger.Error("Error requesting file list", "err", err)
				}
				return m, nil
			}

			// Handle file selection - initiate download
			m.logger.Info("File selected for download", "name", item.name)

			// Return to previous screen
			m.currentScreen = m.previousScreen

			// Create task
			taskID := uuid.New().String()
			task := &Task{
				ID:        taskID,
				FileName:  item.name,
				FilePath:  m.filePath,
				Status:    TaskPending,
				StartTime: time.Now(),
			}
			m.taskManager.Add(task)

			// Send download transaction
			t := hotline.NewTransaction(
				hotline.TranDownloadFile,
				[2]byte{},
				hotline.NewField(hotline.FieldFileName, []byte(item.name)),
			)

			// Add file path if in subdirectory
			if len(m.filePath) > 0 {
				pathStr := strings.Join(m.filePath, "/")
				t.Fields = append(t.Fields, hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(pathStr)))
			}

			// Map transaction ID to task ID
			m.pendingDownloads[t.ID] = taskID

			if err := m.hlClient.Send(t); err != nil {
				m.logger.Error("Error sending download transaction", "err", err)
			}
		}
		return m, nil
	}

	// Pass other keys to the list for navigation
	var cmd tea.Cmd
	m.fileList, cmd = m.fileList.Update(msg)
	return m, cmd
}

// Logs screen
func (m *Model) renderLogs() string {

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		subScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				subTitleStyle.Render("Logs"),
				m.logsViewport.View(),
				" ",
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					help.New().View(viewportKeys),
					"  ", // Spacer
					fmt.Sprintf("%3.f%%", m.logsViewport.ScrollPercent()*100),
				),
			),
		),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
	)
}

func (m *Model) renderMessageBoard() string {

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		subScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				subTitleStyle.Render("Message Board"),
				m.messageBoardViewport.View(),
				" ",
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					help.New().View(viewportKeys),
					"  ", // Spacer
					fmt.Sprintf("%3.f%%", m.messageBoardViewport.ScrollPercent()*100),
				),
			),
		),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
	)
}

func (m *Model) handleMessageBoardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.messageBoardViewport, cmd = m.messageBoardViewport.Update(msg)
	return m, cmd
}

func (m *Model) handleLogsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.logsViewport, cmd = m.logsViewport.Update(msg)
	return m, cmd
}

var subtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}

var dialogBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#874BFD")).
	Padding(1, 0).
	BorderTop(true).
	BorderLeft(true).
	BorderRight(true).
	BorderBottom(true)

var buttonStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFF7DB")).
	Background(lipgloss.Color("#888B7E")).
	Padding(0, 3).
	MarginTop(1)

var activeButtonStyle = buttonStyle.
	Foreground(lipgloss.Color("#FFF7DB")).
	Background(lipgloss.Color("#F25D94")).
	MarginRight(2).
	Underline(true)

// Modal screen
func (m *Model) renderModal() string {
	var b strings.Builder

	//b.WriteString(serverTitleStyle.Render(m.modalTitle))
	b.WriteString(dialogBoxStyle.Render(m.modalContent))

	okButton := activeButtonStyle.Render("Yes")
	cancelButton := buttonStyle.Render("Cancel")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, okButton, cancelButton)
	ui := lipgloss.JoinVertical(lipgloss.Center, m.modalContent, buttons)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialogBoxStyle.Render(ui),
		lipgloss.WithWhitespaceChars("☃︎"),
		lipgloss.WithWhitespaceForeground(subtle),
	)
}

func (m *Model) renderComposeMsg() string {
	var b strings.Builder

	// Get target username
	targetName := ""
	for _, u := range m.userList {
		if u.ID == m.composeMsgTarget {
			targetName = u.Name
			break
		}
	}

	b.WriteString(serverTitleStyle.Render("Send Private Message to " + targetName))
	b.WriteString("\n\n")

	// Show quoted message if this is a reply
	if m.composeMsgQuote != "" {
		quotedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
		b.WriteString(quotedStyle.Render("> " + m.composeMsgQuote))
		b.WriteString("\n\n")
	}

	b.WriteString(m.composeMsgInput.View())
	b.WriteString("\n\n")
	b.WriteString("[Enter] Send  [Esc] Cancel")

	content := boxStyle.Width(60).Render(b.String())

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func (m *Model) handleComposeMsgKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel - close modal
		m.composeMsgModal = false
		m.composeMsgInput.SetValue("")
		m.composeMsgQuote = ""
		return m, nil

	case "enter":
		// Send message
		text := m.composeMsgInput.Value()
		if text != "" {
			// Create transaction with message and target user ID
			fields := []hotline.Field{
				hotline.NewField(hotline.FieldData, []byte(text)),
				hotline.NewField(hotline.FieldUserID, m.composeMsgTarget[:]),
			}

			// Add quoted message if replying
			if m.composeMsgQuote != "" {
				fields = append(fields, hotline.NewField(hotline.FieldQuotingMsg, []byte(m.composeMsgQuote)))
			}

			t := hotline.NewTransaction(hotline.TranSendInstantMsg, [2]byte{}, fields...)
			if err := m.hlClient.Send(t); err != nil {
				m.logger.Error("Error sending private message", "err", err)
			}

			// Close modal and reset
			m.composeMsgModal = false
			m.composeMsgInput.SetValue("")
			m.composeMsgQuote = ""
		}
		return m, nil
	}

	// Pass other keys to the input
	var cmd tea.Cmd
	m.composeMsgInput, cmd = m.composeMsgInput.Update(msg)
	return m, cmd
}

func (m *Model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1", "enter":
		// First button or Enter
		switch m.modalTitle {
		case "Server Agreement":
			// Send TranAgreed to accept the agreement
			_ = m.hlClient.Send(hotline.NewTransaction(
				hotline.TranAgreed,
				[2]byte{},
				hotline.NewField(hotline.FieldUserName, []byte(m.prefs.Username)),
				hotline.NewField(hotline.FieldUserIconID, m.prefs.IconBytes()),
				hotline.NewField(hotline.FieldUserFlags, []byte{0x00, 0x00}),
				hotline.NewField(hotline.FieldOptions, []byte{0x00, 0x00}),
			))

			// Dismiss modal and return to previous screen
			// The login handler will automatically switch to server UI when complete
			m.currentScreen = m.previousScreen
			return m, nil

		case "Disconnect":
			if len(m.modalButtons) > 1 {
				// Cancel disconnect
				m.currentScreen = m.previousScreen
			} else {
				// Exit confirmation
				_ = m.hlClient.Disconnect()
				m.currentScreen = ScreenHome
			}

		default:
			// Generic modal (errors, etc.) - return to previous screen
			m.currentScreen = m.previousScreen
		}
		return m, nil

	case "2":
		// Second button
		if strings.HasPrefix(m.modalTitle, "Private Message from") {
			// Reply button - open compose modal with quote
			m.composeMsgInput.SetValue("")
			m.composeMsgModal = true
			m.currentScreen = m.previousScreen
			return m, nil
		}

		switch m.modalTitle {
		case "Server Agreement":
			// User disagreed - disconnect and return to home
			_ = m.hlClient.Disconnect()
			m.currentScreen = ScreenHome

		case "Disconnect from the server?":
			_ = m.hlClient.Disconnect()
			m.currentScreen = ScreenHome
		}
		return m, nil
	}

	return m, nil
}

// Tasks screen rendering
func (m *Model) renderTasks() string {
	activeTasks := m.taskManager.GetActive()
	completedTasks := m.taskManager.GetCompleted(10) // Last 10

	var b strings.Builder

	// Header
	b.WriteString(serverTitleStyle.Render("Download Tasks"))
	b.WriteString("\n\n")

	// Active section
	if len(activeTasks) > 0 {
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
		b.WriteString(sectionStyle.Render("Active Downloads"))
		b.WriteString("\n\n")

		for _, task := range activeTasks {
			b.WriteString(m.renderTask(task))
			b.WriteString("\n\n")
		}
	} else {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(mutedStyle.Render("No active downloads"))
		b.WriteString("\n\n")
	}

	// Completed section
	if len(completedTasks) > 0 {
		b.WriteString("\n")
		sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
		b.WriteString(sectionStyle.Render("Recent Completed"))
		b.WriteString("\n\n")

		for _, task := range completedTasks {
			b.WriteString(m.renderCompletedTask(task))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(helpStyle.Render("[Esc] Close"))

	return b.String()
}

func (m *Model) renderTask(task *Task) string {
	var b strings.Builder

	// File name
	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	b.WriteString(highlightStyle.Render(task.FileName))
	b.WriteString("\n")

	// Progress bar (40 chars)
	progress := float64(task.TransferredBytes) / float64(task.TotalBytes)
	if task.TotalBytes == 0 {
		progress = 0
	}
	filled := int(progress * 40)
	if filled > 40 {
		filled = 40
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 40-filled)
	b.WriteString(bar)
	b.WriteString("\n")

	// Stats
	pct := int(progress * 100)
	if pct > 100 {
		pct = 100
	}
	transferred := formatBytes(task.TransferredBytes)
	total := formatBytes(task.TotalBytes)

	// Calculate speed and ETA
	now := time.Now()
	elapsed := now.Sub(task.LastUpdate).Seconds()
	if elapsed > 0 {
		deltaBytes := task.TransferredBytes - task.LastBytes
		task.Speed = float64(deltaBytes) / elapsed
		task.LastBytes = task.TransferredBytes
		task.LastUpdate = now
	}

	speed := formatBytes(int64(task.Speed)) + "/s"

	var eta string
	if task.Speed > 0 {
		remaining := task.TotalBytes - task.TransferredBytes
		etaSeconds := float64(remaining) / task.Speed
		eta = formatDuration(time.Duration(etaSeconds) * time.Second)
	} else {
		eta = "--:--"
	}

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	stats := fmt.Sprintf("%d%% • %s / %s • %s • ETA: %s", pct, transferred, total, speed, eta)
	b.WriteString(mutedStyle.Render(stats))

	return b.String()
}

func (m *Model) renderCompletedTask(task *Task) string {
	var icon, status string

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	if task.Status == TaskCompleted {
		icon = successStyle.Render("✓")
		duration := task.EndTime.Sub(task.StartTime)
		status = fmt.Sprintf("%s • %s", formatBytes(task.TotalBytes), formatDuration(duration))
	} else {
		icon = errorStyle.Render("✗")
		if task.Error != nil {
			status = task.Error.Error()
		} else {
			status = "Failed"
		}
	}

	return fmt.Sprintf("%s %s  %s", icon, task.FileName, mutedStyle.Render(status))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m *Model) handleTasksKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentScreen = m.previousScreen
		return m, nil
	}
	return m, nil
}

func rainbow(base lipgloss.Style, s string, colors []color.Color) string {
	var str string
	for i, ss := range s {
		color, _ := colorful.MakeColor(colors[i%len(colors)])
		str = str + base.Foreground(lipgloss.Color(color.Hex())).Render(string(ss))
	}
	return str
}
