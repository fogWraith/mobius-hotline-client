package ui

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius/hotline"
	"github.com/muesli/gamut"
	"github.com/muesli/reflow/wordwrap"
	"gopkg.in/yaml.v3"
)

type Bookmark struct {
	Name     string `yaml:"Name"`
	Addr     string `yaml:"Addr"`
	Login    string `yaml:"Login"`
	Password string `yaml:"Password"`
	TLS      bool   `yaml:"TLS"`
}

// Screen types
type Screen int

const (
	ScreenHome Screen = iota
	ScreenJoinServer
	ScreenBookmarks
	ScreenTracker
	ScreenSettings
	ScreenServerUI
	ScreenNews
	ScreenNewsPost
	ScreenMessageBoard
	ScreenFiles
	ScreenLogs
	ScreenModal
	ScreenTasks
	ScreenAccounts
)

// Messages
type screenMsg struct {
	screen Screen
}

type chatMsg struct {
	text string
}

type userListMsg struct {
	users []hotline.User
}

type newsMsg struct {
	text string
}

type messageBoardMsg struct {
	text string
}

type errorMsg struct {
	text string
}

type serverMsgMsg struct {
	from   string
	userID [2]byte
	text   string
	time   string
}

type agreementMsg struct {
	text string
}

type trackerListMsg struct {
	servers []hotline.ServerRecord
}

type serverConnectedMsg struct {
	name string
}

type filesMsg struct {
	files []hotline.FileNameWithInfo
}

type taskProgressMsg struct {
	taskID string
	bytes  int64
}

type taskStatusMsg struct {
	taskID string
	status TaskStatus
	err    error
}

type downloadReplyMsg struct {
	txID         [4]byte
	refNum       [4]byte
	transferSize uint32
	fileSize     uint32
}

type uploadReplyMsg struct {
	txID   [4]byte
	refNum [4]byte
}

type newsCategoriesMsg struct {
	categories []newsItem
}

type newsArticlesMsg struct {
	articles []newsArticleItem
}

type newsArticleDataMsg struct {
	article hotline.NewsArtData
}

// newsItem represents a category or bundle in the news hierarchy
type newsItem struct {
	name     string
	isBundle bool // true = bundle (container), false = category (contains articles)
}

func (i newsItem) FilterValue() string { return i.name }
func (i newsItem) Title() string {
	if i.name == "<- Back" {
		return i.name
	}
	if i.isBundle {
		return "📦 " + i.name
	}
	return "📰 " + i.name
}
func (i newsItem) Description() string {
	if i.name == "<- Back" {
		return ""
	}
	if i.isBundle {
		return "Bundle"
	}
	return "Category"
}

// newsArticleItem represents an article in a category
type newsArticleItem struct {
	id          uint32
	title       string
	poster      string
	date        [8]byte
	parentID    uint32 // Parent article ID (0 = root)
	depth       int    // Nesting level
	isExpanded  bool   // Are children shown?
	hasChildren bool   // Has replies?
}

func (i newsArticleItem) FilterValue() string { return i.title }
func (i newsArticleItem) Title() string {
	indent := strings.Repeat("  ", i.depth)

	var indicator string
	if i.hasChildren {
		if i.isExpanded {
			indicator = "∨ "
		} else {
			indicator = "› "
		}
	} else {
		indicator = "  "
	}

	return indent + indicator + i.title
}
func (i newsArticleItem) Description() string {
	// Convert and format timestamp
	timestamp := hotline.Time(i.date).Format("(Jan 2, 2006 at 3:04 PM)")

	// Use lipgloss for faint styling
	timestampStyle := lipgloss.NewStyle().Faint(true)

	return i.poster + "   " + timestampStyle.Render(timestamp)
}

// buildThreadTree calculates depth and hasChildren for each article
func buildThreadTree(articles []newsArticleItem) []newsArticleItem {
	// Build article ID set
	validIDs := make(map[uint32]bool)
	for _, art := range articles {
		validIDs[art.id] = true
	}

	// Build parent -> children map
	childrenMap := make(map[uint32][]int)

	for i, art := range articles {
		if art.parentID != 0 {
			// Orphaned articles become roots
			if !validIDs[art.parentID] {
				articles[i].parentID = 0
			} else {
				childrenMap[art.parentID] = append(childrenMap[art.parentID], i)
			}
		}
	}

	// Set hasChildren flag
	for i := range articles {
		articles[i].hasChildren = len(childrenMap[articles[i].id]) > 0
	}

	// Calculate depths recursively
	var calculateDepth func(articleID uint32, currentDepth int)
	calculateDepth = func(articleID uint32, currentDepth int) {
		for _, childIdx := range childrenMap[articleID] {
			articles[childIdx].depth = currentDepth
			calculateDepth(articles[childIdx].id, currentDepth+1)
		}
	}

	// Start from roots
	for i := range articles {
		if articles[i].parentID == 0 {
			articles[i].depth = 0
			calculateDepth(articles[i].id, 1)
		}
	}

	// Reverse the slice of articles so that newest are ordered first.
	slices.Reverse(articles)

	return articles
}

// filterVisibleArticles returns articles that should be shown based on expansion state
// Articles are ordered so children appear immediately after their parent
func (m *Model) filterVisibleArticles(articles []newsArticleItem) []newsArticleItem {
	var visible []newsArticleItem

	// Build article lookup and children map
	articleMap := make(map[uint32]newsArticleItem)
	childrenMap := make(map[uint32][]newsArticleItem)

	for _, art := range articles {
		articleMap[art.id] = art
		if art.parentID != 0 {
			childrenMap[art.parentID] = append(childrenMap[art.parentID], art)
		}
	}

	// Recursively add article and its visible children
	var addArticleAndChildren func(art newsArticleItem)
	addArticleAndChildren = func(art newsArticleItem) {
		visible = append(visible, art)

		// If this article is expanded, add its children
		if m.expandedArticles[art.id] {
			for _, child := range childrenMap[art.id] {
				addArticleAndChildren(child)
			}
		}
	}

	// Start with root articles (in original order)
	for _, art := range articles {
		if art.parentID == 0 {
			addArticleAndChildren(art)
		}
	}

	return visible
}

// refreshNewsArticleList rebuilds the news list with current visibility state
func (m *Model) refreshNewsArticleList() {
	visibleArticles := m.filterVisibleArticles(m.allArticles)

	var items []list.Item

	for _, art := range visibleArticles {
		art.isExpanded = m.expandedArticles[art.id]
		items = append(items, art)
	}

	currentIndex := m.newsList.Index()
	m.newsList.SetItems(items)

	if currentIndex >= len(items) {
		currentIndex = len(items) - 1
	}
	if currentIndex >= 0 {
		m.newsList.Select(currentIndex)
	}
}

// selectedArticleData holds the full article data for display
type selectedArticleData struct {
	id      uint32
	title   string
	poster  string
	date    [8]byte
	content string
}

// Account management types
type accountListMsg struct {
	accounts []accountItem
}

type accountItem struct {
	login   string
	name    string
	hasPass bool
	access  hotline.AccessBitmap
	index   int
}

func (a accountItem) FilterValue() string { return a.login }
func (a accountItem) Title() string       { return a.login }
func (a accountItem) Description() string {
	desc := a.name
	if a.hasPass {
		desc += " [password set]"
	}
	return desc
}

type selectedAccountData struct {
	login          string
	name           string
	originalAccess hotline.AccessBitmap
	hasPassword    bool
}

type accessBitInfo struct {
	bit         int
	name        string
	description string
}

// Focus indices for account editor (beyond checkboxes)
const (
	focusLogin = 41 // Login field
	focusName  = 42 // Display name field
	focusPass  = 43 // Password field
)

// Model
type Model struct {
	// Configuration
	cfgPath     string
	prefs       *Settings
	logger      *slog.Logger
	debugBuffer *DebugBuffer
	soundPlayer *SoundPlayer

	// Screen state
	currentScreen  Screen
	previousScreen Screen
	width          int
	height         int
	welcomeBanner  string // Randomly selected banner, loaded once at startup

	// Bubble Tea program reference for sending messages
	program *tea.Program

	// Hotline client
	hlClient          *hotline.Client
	serverName        string
	pendingServerName string // Name to display when connection succeeds (from bookmark/tracker/address)
	pendingServerAddr string // Address being connected to
	userAccess        hotline.AccessBitmap
	userList          []hotline.User

	// Join server form
	nameInput            textinput.Model
	serverInput          textinput.Model
	loginInput           textinput.Model
	passwordInput        textinput.Model
	useTLS               bool
	saveBookmark         bool
	focusIndex           int
	backPage             Screen
	editingBookmark      bool
	editingBookmarkIndex int
	creatingBookmark     bool

	// Server UI
	chatViewport     viewport.Model
	chatInput        textinput.Model
	userViewport     viewport.Model
	chatContent      string
	chatMessages     []string // Store original formatted messages for re-wrapping
	chatWasAtBottom  bool     // Track scroll position for smart auto-scroll
	serverUIKeys     serverUIKeyMap
	serverUIHelp     help.Model
	focusOnUserList  bool // true = user list focused, false = chat input focused
	selectedUserIdx  int  // index of selected user in userList
	composeMsgInput  textinput.Model
	composeMsgModal  bool
	composeMsgTarget [2]byte // user ID of target user
	composeMsgQuote  string  // quoted message for reply

	// Settings form
	usernameInput    textinput.Model
	iconIDInput      textinput.Model
	trackerInput     textinput.Model
	downloadDirInput textinput.Model
	enableBell       bool
	enableSounds     bool

	// Bookmarks/Tracker
	bookmarkList list.Model
	allBookmarks []Bookmark // Store all bookmarks for filtering
	trackerList  list.Model
	allTrackers  []hotline.ServerRecord // Store all trackers for filtering

	// News (old message board style)
	newsViewport         viewport.Model
	newsContent          string
	newsPostText         string
	newsPostConfirm      bool
	newsPostForm         *huh.Form
	newsArticlePostForm  *huh.Form // For threaded news article posting
	newsBundleForm       *huh.Form // For creating news bundles
	newsCategoryForm     *huh.Form // For creating news categories
	replyParentArticleID uint32    // Parent article ID when replying (0 for new posts)

	// Threaded News
	newsList          list.Model
	newsPath          []string             // Track current location in news hierarchy
	isViewingCategory bool                 // true = viewing category (articles), false = viewing bundle/root
	showNewsModal     bool                 // Display news as modal over server UI
	selectedArticle   *selectedArticleData // Currently selected article
	pendingArticleID  uint32               // Article ID of pending request for tracking
	articleViewport   viewport.Model       // For scrolling article content
	allArticles       []newsArticleItem    // Complete article set
	expandedArticles  map[uint32]bool      // Track expanded state

	// Account management
	accountList          list.Model           // Bubble Tea list component
	allAccounts          []accountItem        // Complete account dataset
	selectedAccount      *selectedAccountData // Currently selected account
	showAccountsModal    bool                 // Display accounts screen
	accountDetailFocused bool                 // true = detail pane focused, false = list pane focused
	editedAccessBits     hotline.AccessBitmap // Working copy of permissions
	editedName           string               // Working copy of display name
	editedLogin          string               // Working copy of login name
	editedPassword       string               // Working copy of password
	focusedAccessBit     int                  // Currently focused checkbox (0-40, or 41+ for other fields)
	accountsViewport     viewport.Model       // For scrolling account details
	isNewAccount         bool                 // Creating new vs editing existing
	passwordChanged      bool                 // Track if password was modified

	// Files
	fileList       list.Model
	filePath       []string // Track current folder path for navigation
	showFilesModal bool     // Track if files should be shown as modal over server UI

	// File picker for uploads
	filePicker         filepicker.Model
	showFilePicker     bool
	lastPickerLocation string // Remember last location

	// MessageBoard
	messageBoardViewport viewport.Model
	messageBoardContent  string

	// Logs
	logsViewport viewport.Model

	// Modal state
	modalTitle   string
	modalContent string
	modalButtons []string

	// Task management for file downloads and uploads
	taskManager      *TaskManager
	downloadDir      string
	pendingDownloads map[[4]byte]string // transaction ID -> task ID
	pendingUploads   map[[4]byte]string // transaction ID -> task ID

	// Task widget
	taskProgress map[string]progress.Model // task ID -> progress model
}

// Styles
var (
	// Styling for the main server screen title.
	serverTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("170"))

	subTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 0, 1).
			Foreground(lipgloss.Color("170"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("170")).
				Bold(true)

	adminUserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	awayUserStyle = lipgloss.NewStyle().
			Faint(true)

	awayAdminUserStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true).
				Faint(true)
)

// Initialize logs viewport help and key bindings
var viewportKeys = viewportKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("PgUp", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("PgDn", "page down"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("Esc", "back"),
	),
}

func NewModel(cfgPath string, logger *slog.Logger, db *DebugBuffer) *Model {
	prefs, err := readConfig(cfgPath)
	if err != nil {
		logger.Error(fmt.Sprintf("unable to read config file %s\n", cfgPath))
		os.Exit(1)
	}

	// Initialize text inputs
	nameInput := textinput.New()
	nameInput.Placeholder = "Bookmark Name"

	serverInput := textinput.New()
	serverInput.Placeholder = "server:port"

	loginInput := textinput.New()
	loginInput.Placeholder = "guest"

	passwordInput := textinput.New()
	passwordInput.Placeholder = "password"
	passwordInput.EchoMode = textinput.EchoPassword

	chatInput := textinput.New()
	chatInput.Placeholder = "Type a message..."
	chatInput.Width = 80

	composeMsgInput := textinput.New()
	composeMsgInput.Placeholder = "Type your message..."
	composeMsgInput.Width = 50
	composeMsgInput.Focus()

	usernameInput := textinput.New()
	usernameInput.Placeholder = "Your Name"
	usernameInput.SetValue(prefs.Username)

	iconIDInput := textinput.New()
	iconIDInput.Placeholder = "Icon ID"
	iconIDInput.SetValue(strconv.Itoa(prefs.IconID))

	trackerInput := textinput.New()
	trackerInput.Placeholder = "Tracker URL"
	trackerInput.SetValue(prefs.Tracker)

	downloadDirInput := textinput.New()
	downloadDirInput.Placeholder = "Download Directory"
	if prefs.DownloadDir != "" {
		downloadDirInput.SetValue(prefs.DownloadDir)
	} else {
		home, _ := os.UserHomeDir()
		downloadDirInput.SetValue(home + "/Downloads/Hotline")
	}

	hlClient := hotline.NewClient(prefs.Username, logger)

	// Initialize server UI help and key bindings
	serverUIKeys := serverUIKeyMap{
		MessageBoard: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("^B", "messageboard"),
		),
		News: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("^N", "news"),
		),
		Files: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("^F", "files"),
		),
		Logs: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("^L", "logs"),
		),
		Accounts: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("^A", "accounts"),
		),
		Disconnect: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "disconnect"),
		),
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
	}

	serverUIHelp := help.New()

	// Initialize download directory
	downloadDir := prefs.DownloadDir
	if downloadDir == "" {
		home, _ := os.UserHomeDir()
		downloadDir = home + "/Downloads/Hotline"
	}

	// Initialize file picker for uploads
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // Allow all file types
	fp.SetHeight(20)             // Set reasonable height for modal
	startDir, _ := os.UserHomeDir()
	fp.CurrentDirectory = startDir

	// Initialize sound player
	soundPlayer, err := NewSoundPlayer(prefs, logger)
	if err != nil {
		logger.Error("Failed to initialize sound player", "err", err)
		// Non-fatal - continue without sounds
		soundPlayer = nil
	}

	return &Model{
		cfgPath:            cfgPath,
		prefs:              prefs,
		logger:             logger,
		debugBuffer:        db,
		soundPlayer:        soundPlayer,
		currentScreen:      ScreenHome,
		welcomeBanner:      randomBanner(), // Load banner once at startup
		nameInput:          nameInput,
		serverInput:        serverInput,
		loginInput:         loginInput,
		passwordInput:      passwordInput,
		chatInput:          chatInput,
		composeMsgInput:    composeMsgInput,
		usernameInput:      usernameInput,
		iconIDInput:        iconIDInput,
		trackerInput:       trackerInput,
		downloadDirInput:   downloadDirInput,
		allBookmarks:       prefs.Bookmarks, // Store all bookmarks for filtering
		hlClient:           hlClient,
		enableBell:         prefs.EnableBell,
		enableSounds:       prefs.EnableSounds,
		serverUIKeys:       serverUIKeys,
		serverUIHelp:       serverUIHelp,
		taskManager:        NewTaskManager(),
		downloadDir:        downloadDir,
		pendingDownloads:   make(map[[4]byte]string),
		pendingUploads:     make(map[[4]byte]string),
		filePicker:         fp,
		lastPickerLocation: startDir,
		taskProgress:       make(map[string]progress.Model),
		expandedArticles:   make(map[uint32]bool),
		allArticles:        []newsArticleItem{},
	}
}

func readConfig(cfgPath string) (*Settings, error) {
	fh, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = fh.Close()
	}()

	prefs := Settings{}
	decoder := yaml.NewDecoder(fh)
	if err := decoder.Decode(&prefs); err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle compose message modal when active
	if m.composeMsgModal {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			return m.handleComposeMsgKeys(msg)
		}
		return m, nil
	}

	// Handle file picker when active
	if m.showFilePicker {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.showFilePicker = false
				return m, nil
			}
		}

		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)

		// Check if file was selected
		if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
			m.showFilePicker = false
			m.currentScreen = ScreenServerUI // Return to server UI after selecting file for upload
			// Remember location for next time
			m.lastPickerLocation = m.filePicker.CurrentDirectory
			// Initiate upload
			return m, m.initiateFileUpload(path)
		}

		// Check for disabled file (means user hit 'q' or similar to quit)
		if didSelect, _ := m.filePicker.DidSelectDisabledFile(msg); didSelect {
			m.showFilePicker = false
			return m, nil
		}

		return m, cmd
	}
	if m.currentScreen == ScreenTracker {
		var cmds []tea.Cmd

		// Handle custom keys when NOT actively filtering
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if m.trackerList.FilterState() != list.Filtering {
				switch keyMsg.String() {
				case "esc":
					m.currentScreen = ScreenHome
					return m, nil
				case "enter":
					// Get selected server
					if item, ok := m.trackerList.SelectedItem().(trackerItem); ok {
						srv := item.server

						// Set up join server form with tracker server details
						m.serverInput.SetValue(srv.Addr())
						m.loginInput.SetValue(hotline.GuestAccount)
						m.passwordInput.SetValue("")
						m.useTLS = false
						m.saveBookmark = false
						m.focusIndex = 0
						m.backPage = ScreenTracker
						m.currentScreen = ScreenJoinServer
						m.serverInput.Focus()

						// Set the pending server name to tracker server name
						m.pendingServerName = string(srv.Name)
					}
					return m, nil
				}
			}
		}

		// Always update the list (for all message types and unhandled keys)
		var cmd tea.Cmd
		m.trackerList, cmd = m.trackerList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
	if m.currentScreen == ScreenBookmarks {
		var cmds []tea.Cmd

		// Handle custom keys when NOT actively filtering
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if m.bookmarkList.FilterState() != list.Filtering {
				switch keyMsg.String() {
				case "esc":
					m.currentScreen = ScreenHome
					return m, nil
				case "enter":
					// Get selected bookmark
					if item, ok := m.bookmarkList.SelectedItem().(bookmarkItem); ok {
						bm := item.bookmark

						// Set up join server form with bookmark details
						m.serverInput.SetValue(bm.Addr)
						m.loginInput.SetValue(bm.Login)
						m.passwordInput.SetValue(bm.Password)
						m.useTLS = bm.TLS
						m.saveBookmark = false
						m.editingBookmark = false
						m.creatingBookmark = false
						m.focusIndex = 0
						m.backPage = ScreenBookmarks
						m.currentScreen = ScreenJoinServer
						m.serverInput.Focus()

						// Set the pending server name to bookmark name
						m.pendingServerName = bm.Name
					}
					return m, nil

				case "e":
					// Edit selected bookmark
					if item, ok := m.bookmarkList.SelectedItem().(bookmarkItem); ok {
						bm := item.bookmark

						// Set up join server form with bookmark details
						m.nameInput.SetValue(bm.Name)
						m.serverInput.SetValue(bm.Addr)
						m.loginInput.SetValue(bm.Login)
						m.passwordInput.SetValue(bm.Password)
						m.useTLS = bm.TLS
						m.saveBookmark = false
						m.editingBookmark = true
						m.editingBookmarkIndex = item.index
						m.creatingBookmark = false
						m.focusIndex = 0
						m.backPage = ScreenBookmarks
						m.currentScreen = ScreenJoinServer
						m.nameInput.Focus()
					}
					return m, nil

				case "n":
					// Create new bookmark
					m.nameInput.SetValue("")
					m.serverInput.SetValue("")
					m.loginInput.SetValue("")
					m.passwordInput.SetValue("")
					m.useTLS = false
					m.saveBookmark = false
					m.editingBookmark = false
					m.creatingBookmark = true
					m.focusIndex = 0
					m.backPage = ScreenBookmarks
					m.currentScreen = ScreenJoinServer
					m.nameInput.Focus()
					return m, nil

				case "x":
					// Delete bookmark - delegate handles removal from list, we sync and save
					index := m.bookmarkList.Index()
					if index >= 0 && index < len(m.bookmarkList.Items()) {
						// Get the bookmark being deleted
						if item, ok := m.bookmarkList.Items()[index].(bookmarkItem); ok {
							// Remove from allBookmarks by finding matching bookmark
							for i, bm := range m.allBookmarks {
								if bm.Name == item.bookmark.Name && bm.Addr == item.bookmark.Addr {
									m.allBookmarks = append(m.allBookmarks[:i], m.allBookmarks[i+1:]...)
									break
								}
							}
							// Update prefs and save
							m.prefs.Bookmarks = m.allBookmarks
							_ = m.savePreferences()
						}
					}
					// Let the list handle the UI update
					var cmd tea.Cmd
					m.bookmarkList, cmd = m.bookmarkList.Update(msg)
					cmds = append(cmds, cmd)
					return m, tea.Batch(cmds...)
				}
			}
		}

		// Always update the list (for all message types and unhandled keys)
		var cmd tea.Cmd
		m.bookmarkList, cmd = m.bookmarkList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Handle news post form when active
	if m.currentScreen == ScreenNewsPost {
		// Handle ESC to cancel
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "esc" {
				// Determine which form is active
				if m.newsArticlePostForm != nil {
					m.newsArticlePostForm = nil
					m.showNewsModal = true // Return to news modal
				} else if m.newsBundleForm != nil {
					m.newsBundleForm = nil
					m.showNewsModal = true // Return to news modal
				} else if m.newsCategoryForm != nil {
					m.newsCategoryForm = nil
					m.showNewsModal = true // Return to news modal
				} else if m.newsPostForm != nil {
					m.newsPostForm = nil
				}
				m.currentScreen = ScreenServerUI
				return m, nil
			}
		}

		var cmds []tea.Cmd

		// Handle article post form (new threaded news)
		if m.newsArticlePostForm != nil {
			form, cmd := m.newsArticlePostForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.newsArticlePostForm = f
				cmds = append(cmds, cmd)
			}

			// Check if form is complete
			if m.newsArticlePostForm.State == huh.StateCompleted {
				subject := m.newsArticlePostForm.GetString("subject")
				body := m.newsArticlePostForm.GetString("body")
				confirmed := m.newsArticlePostForm.GetBool("confirm")

				// Only send if confirmed and has content
				if confirmed && subject != "" && body != "" {
					// Create transaction with all required fields
					pathBytes := encodeNewsPath(m.newsPath)

					// Parent article ID: 0 for new post, or stored ID for replies
					parentIDBytes := make([]byte, 4)
					binary.BigEndian.PutUint32(parentIDBytes, m.replyParentArticleID)

					t := hotline.NewTransaction(
						hotline.TranPostNewsArt,
						[2]byte{},
						hotline.NewField(hotline.FieldNewsPath, pathBytes),
						hotline.NewField(hotline.FieldNewsArtID, parentIDBytes),
						hotline.NewField(hotline.FieldNewsArtTitle, []byte(subject)),
						hotline.NewField(hotline.FieldNewsArtData, []byte(body)),
					)

					if err := m.hlClient.Send(t); err != nil {
						m.logger.Error("Error posting news article", "err", err)
					} else {
						// Success - clean up form
						m.newsArticlePostForm = nil
						m.replyParentArticleID = 0 // Reset parent ID
						m.currentScreen = ScreenServerUI
						m.showNewsModal = true

						// Refetch the article list to show the new post
						refetchCmd := func() tea.Msg {
							pathBytes := encodeNewsPath(m.newsPath)
							if err := m.hlClient.Send(hotline.NewTransaction(
								hotline.TranGetNewsArtNameList,
								[2]byte{},
								hotline.NewField(hotline.FieldNewsPath, pathBytes),
							)); err != nil {
								m.logger.Error("Error refetching articles", "err", err)
							}
							return nil
						}
						cmds = append(cmds, refetchCmd)
					}
				} else {
					// Cancelled - return to news
					m.newsArticlePostForm = nil
					m.replyParentArticleID = 0 // Reset parent ID
					m.currentScreen = ScreenServerUI
					m.showNewsModal = true
				}
			}

			return m, tea.Batch(cmds...)
		}

		// Handle news bundle creation form
		if m.newsBundleForm != nil {
			form, cmd := m.newsBundleForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.newsBundleForm = f
				cmds = append(cmds, cmd)
			}

			// Check if form is complete
			if m.newsBundleForm.State == huh.StateCompleted {
				bundleName := m.newsBundleForm.GetString("bundleName")
				confirmed := m.newsBundleForm.GetBool("confirm")

				// Only send if confirmed and has valid name
				if confirmed && strings.TrimSpace(bundleName) != "" {
					// Create bundle at current location
					pathBytes := encodeNewsPath(m.newsPath)

					t := hotline.NewTransaction(
						hotline.TranNewNewsFldr,
						[2]byte{},
						hotline.NewField(hotline.FieldNewsPath, pathBytes),
						hotline.NewField(hotline.FieldFileName, []byte(bundleName)),
					)

					if err := m.hlClient.Send(t); err != nil {
						m.logger.Error("Error creating news bundle", "err", err)
					} else {
						// Clean up and refresh list
						m.newsBundleForm = nil
						m.currentScreen = ScreenServerUI
						m.showNewsModal = true

						// Refetch current location
						refetchCmd := func() tea.Msg {
							pathBytes := encodeNewsPath(m.newsPath)
							if err := m.hlClient.Send(hotline.NewTransaction(
								hotline.TranGetNewsCatNameList,
								[2]byte{},
								hotline.NewField(hotline.FieldNewsPath, pathBytes),
							)); err != nil {
								m.logger.Error("Error refetching categories", "err", err)
							}
							return nil
						}
						cmds = append(cmds, refetchCmd)
					}
				} else {
					// Cancelled
					m.newsBundleForm = nil
					m.currentScreen = ScreenServerUI
					m.showNewsModal = true
				}
			}

			return m, tea.Batch(cmds...)
		}

		// Handle news category creation form
		if m.newsCategoryForm != nil {
			form, cmd := m.newsCategoryForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.newsCategoryForm = f
				cmds = append(cmds, cmd)
			}

			// Check if form is complete
			if m.newsCategoryForm.State == huh.StateCompleted {
				categoryName := m.newsCategoryForm.GetString("categoryName")
				confirmed := m.newsCategoryForm.GetBool("confirm")

				// Only send if confirmed and has valid name
				if confirmed && strings.TrimSpace(categoryName) != "" {
					// Create category at current location
					pathBytes := encodeNewsPath(m.newsPath)

					t := hotline.NewTransaction(
						hotline.TranNewNewsCat,
						[2]byte{},
						hotline.NewField(hotline.FieldNewsPath, pathBytes),
						hotline.NewField(hotline.FieldNewsCatName, []byte(categoryName)),
					)

					if err := m.hlClient.Send(t); err != nil {
						m.logger.Error("Error creating news category", "err", err)
					} else {
						// Clean up and refresh list
						m.newsCategoryForm = nil
						m.currentScreen = ScreenServerUI
						m.showNewsModal = true

						// Refetch current location
						refetchCmd := func() tea.Msg {
							pathBytes := encodeNewsPath(m.newsPath)
							if err := m.hlClient.Send(hotline.NewTransaction(
								hotline.TranGetNewsCatNameList,
								[2]byte{},
								hotline.NewField(hotline.FieldNewsPath, pathBytes),
							)); err != nil {
								m.logger.Error("Error refetching categories", "err", err)
							}
							return nil
						}
						cmds = append(cmds, refetchCmd)
					}
				} else {
					// Cancelled
					m.newsCategoryForm = nil
					m.currentScreen = ScreenServerUI
					m.showNewsModal = true
				}
			}

			return m, tea.Batch(cmds...)
		}

		// Handle old messageboard post form (backward compatibility)
		if m.newsPostForm != nil {
			// Pass ALL messages to the form
			form, cmd := m.newsPostForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.newsPostForm = f
				cmds = append(cmds, cmd)
			}

			// Check if form is complete
			if m.newsPostForm.State == huh.StateCompleted {
				// Extract the news text from the form
				newsText := m.newsPostForm.GetString("newsPost")

				// Only send if there's actual content
				if newsText != "" && m.newsPostForm.GetBool("done") {
					// Create and send the transaction
					t := hotline.NewTransaction(
						hotline.TranOldPostNews,
						[2]byte{},
						hotline.NewField(hotline.FieldData, []byte(newsText)),
					)

					if err := m.hlClient.Send(t); err != nil {
						m.logger.Error("Error posting news", "err", err)
					}
				}

				m.newsPostForm = nil
				m.currentScreen = ScreenServerUI
			}

			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.initializeViewports()

		// Re-wrap chat content for new width
		if len(m.chatMessages) > 0 {
			m.rebuildChatContent()
		}

		// Update list sizes to match new window dimensions
		appStyle := lipgloss.NewStyle().Padding(1, 2)
		h, v := appStyle.GetFrameSize()

		if m.bookmarkList.Items() != nil {
			m.bookmarkList.SetSize(msg.Width-h, msg.Height-v)
		}
		if m.trackerList.Items() != nil {
			m.trackerList.SetSize(msg.Width-h, msg.Height-v)
		}
		if m.fileList.Items() != nil {
			m.fileList.SetSize(msg.Width-h, msg.Height-v)
		}

		// Update help component width
		m.serverUIHelp.Width = msg.Width

		return m, nil

	case screenMsg:
		m.previousScreen = m.currentScreen
		m.currentScreen = msg.screen
		// Focus chat input when entering server UI
		if msg.screen == ScreenServerUI {
			m.chatInput.Focus()
		}
		return m, nil

	case chatMsg:
		// Check if at bottom before updating (for smart auto-scroll)
		m.chatWasAtBottom = m.chatViewport.AtBottom()

		// Use regex to extract username (everything up to and including first colon)
		re := regexp.MustCompile(`^[^:]*:`)
		match := re.FindString(msg.text)

		var formattedMsg string

		// Apply bold styling to username
		usernameStyle := lipgloss.NewStyle().Bold(true)
		message := strings.TrimPrefix(msg.text, match)
		formattedMsg = usernameStyle.Render(match) + message

		// Store original formatted message
		m.chatMessages = append(m.chatMessages, formattedMsg)

		// Rebuild chat content with wrapping
		m.rebuildChatContent()

		// Only auto-scroll if user was at bottom
		if m.chatWasAtBottom {
			m.chatViewport.GotoBottom()
		}

		return m, nil

	case userListMsg:
		m.userList = msg.users
		m.updateUserListDisplay()
		return m, nil

	case newsMsg:
		m.newsContent = msg.text
		m.newsViewport.SetContent(m.newsContent)
		m.currentScreen = ScreenNews
		return m, nil

	case messageBoardMsg:
		m.messageBoardContent = msg.text
		m.messageBoardViewport.SetContent(m.messageBoardContent)
		m.currentScreen = ScreenMessageBoard
		return m, nil

	case errorMsg:
		m.logger.Error("Received error message", "text", msg.text)
		m.modalTitle = "Error"
		m.modalContent = msg.text
		m.modalButtons = []string{"OK"}
		m.previousScreen = m.currentScreen
		m.currentScreen = ScreenModal
		return m, nil

	case serverMsgMsg:
		m.modalTitle = "Private Message from " + msg.from
		m.modalContent = msg.text + "\n\nAt " + msg.time
		m.modalButtons = []string{"OK", "Reply"}
		m.previousScreen = m.currentScreen
		m.composeMsgTarget = msg.userID
		m.composeMsgQuote = msg.text
		m.currentScreen = ScreenModal
		return m, nil

	case agreementMsg:
		m.modalTitle = "Server Agreement"
		m.modalContent = msg.text
		m.modalButtons = []string{"Agree", "Disagree"}
		m.previousScreen = m.currentScreen
		m.currentScreen = ScreenModal
		return m, nil

	case serverConnectedMsg:
		m.serverName = msg.name
		m.currentScreen = ScreenServerUI
		m.chatInput.Focus()
		return m, nil

	case trackerListMsg:
		m.logger.Info("Received tracker list message", "count", len(msg.servers))
		m.initializeTrackerList(msg.servers)
		m.currentScreen = ScreenTracker
		return m, nil

	case filesMsg:
		m.logger.Info("Received files message", "count", len(msg.files))
		m.initializeFileList(msg.files)
		// Only update previousScreen if not already in Files screen
		// This preserves the correct screen to return to when closing Files
		if m.currentScreen != ScreenFiles {
			m.previousScreen = m.currentScreen
		}
		m.currentScreen = ScreenFiles
		return m, nil

	case newsCategoriesMsg:
		m.logger.Info("Received news categories message", "count", len(msg.categories))
		m.initializeNewsCategoryList(msg.categories)
		m.isViewingCategory = false // Viewing bundle/root
		m.showNewsModal = true

		return m, nil

	case newsArticlesMsg:
		m.logger.Info("Received news articles message", "count", len(msg.articles))
		m.initializeNewsArticleList(msg.articles)
		m.isViewingCategory = true // Viewing category (even if 0 articles)
		m.showNewsModal = true
		// Keep currentScreen as ScreenServerUI
		return m, nil

	case newsArticleDataMsg:
		m.logger.Info("Received news article data", "title", msg.article.Title)
		// Store the article for display in split view
		m.selectedArticle = &selectedArticleData{
			id:      m.pendingArticleID, // Use tracked ID from request
			title:   msg.article.Title,
			poster:  msg.article.Poster,
			date:    msg.article.Date,
			content: msg.article.Data,
		}
		m.pendingArticleID = 0 // Clear pending ID
		return m, nil

	case accountListMsg:
		m.logger.Info("Received account list message", "count", len(msg.accounts))
		m.initializeAccountList(msg.accounts)
		m.currentScreen = ScreenAccounts
		m.showAccountsModal = true
		return m, nil

	case taskProgressMsg:
		task := m.taskManager.Get(msg.taskID)
		if task != nil {
			now := time.Now()

			// Calculate speed if we have previous data
			if !task.LastUpdate.IsZero() {
				duration := now.Sub(task.LastUpdate).Seconds()
				if duration > 0 {
					bytesSinceLast := msg.bytes - task.LastBytes
					task.Speed = float64(bytesSinceLast) / duration
				}
			}

			task.TransferredBytes = msg.bytes
			task.LastBytes = msg.bytes
			task.LastUpdate = now

			// Update or create progress model for active tasks
			if task.Status == TaskActive {
				prog, exists := m.taskProgress[msg.taskID]
				if !exists {
					prog = progress.New(progress.WithDefaultGradient())
					prog.Width = 20 // Compact width for widget
					m.taskProgress[msg.taskID] = prog
				}

				percent := 0.0
				if task.TotalBytes > 0 {
					percent = float64(task.TransferredBytes) / float64(task.TotalBytes)
				}
				cmd := prog.SetPercent(percent)
				m.taskProgress[msg.taskID] = prog
				return m, cmd // Return progress animation command
			}
		}
		return m, nil

	case taskStatusMsg:
		task := m.taskManager.Get(msg.taskID)
		if task != nil {
			task.Status = msg.status
			task.Error = msg.err
			if msg.status == TaskCompleted || msg.status == TaskFailed {
				task.EndTime = time.Now()
				// Remove progress model when task completes
				delete(m.taskProgress, msg.taskID)
			}
		}
		return m, nil

	case downloadReplyMsg:
		taskID := m.pendingDownloads[msg.txID]
		delete(m.pendingDownloads, msg.txID)

		task := m.taskManager.Get(taskID)
		if task != nil {
			task.TotalBytes = int64(msg.transferSize)
			task.Status = TaskActive

			// Launch file transfer in background
			go m.performFileTransfer(task, msg.refNum, msg.transferSize)
		}
		return m, nil

	case uploadReplyMsg:
		taskID, ok := m.pendingUploads[msg.txID]
		if !ok {
			return m, nil
		}
		delete(m.pendingUploads, msg.txID)

		task := m.taskManager.Get(taskID)
		if task == nil {
			return m, nil
		}

		task.Status = TaskActive

		// Start file transfer in goroutine
		go m.performFileUpload(task, msg.refNum)

		return m, nil
	}

	return m, nil
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q":
		return m, tea.Quit

	case "esc":
		// Special handling: if news modal is showing, let handleNewsKeys deal with it
		if m.currentScreen == ScreenServerUI && m.showNewsModal {
			// Don't intercept - fall through to screen-specific handler
		} else {
			// Normal Esc handling for all other cases
			switch m.currentScreen {
			case ScreenJoinServer:
				// Reset bookmark editing/creating flags
				m.editingBookmark = false
				m.creatingBookmark = false
				// Return to the page we came from
				if m.backPage != ScreenHome {
					m.currentScreen = m.backPage
				} else {
					m.currentScreen = ScreenHome
				}
			case ScreenBookmarks, ScreenTracker, ScreenSettings:
				m.currentScreen = ScreenHome
			case ScreenNews, ScreenNewsPost, ScreenMessageBoard, ScreenFiles, ScreenModal, ScreenTasks, ScreenAccounts:
				m.currentScreen = ScreenServerUI
			case ScreenLogs:
				m.currentScreen = m.previousScreen
			case ScreenServerUI:
				blends := gamut.Blends(lipgloss.Color("#F25D94"), lipgloss.Color("#EDFF82"), 50)

				question := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(rainbow(lipgloss.NewStyle(), "Disconnect from the server?", blends))
				m.modalContent = question
				m.modalButtons = []string{"Cancel", "Exit"}
				m.previousScreen = ScreenServerUI
				m.currentScreen = ScreenModal
			}
			return m, nil
		}

	case "ctrl+l":
		m.previousScreen = m.currentScreen
		m.currentScreen = ScreenLogs
		m.logsViewport.SetContent(m.debugBuffer.String())
		m.logsViewport.GotoBottom()
		return m, nil

	case "ctrl+t":
		// Only allow opening tasks screen from ServerUI
		if m.currentScreen == ScreenServerUI {
			m.previousScreen = m.currentScreen
			m.currentScreen = ScreenTasks
			return m, nil
		}
	}

	// Screen-specific key handling
	switch m.currentScreen {
	case ScreenHome:
		return m.handleHomeKeys(msg)
	case ScreenJoinServer:
		return m.handleJoinServerKeys(msg)
	case ScreenServerUI:
		// Route to news handler if modal is active
		if m.showNewsModal {
			return m.handleNewsKeys(msg)
		}
		return m.handleServerUIKeys(msg)
	case ScreenSettings:
		return m.handleSettingsKeys(msg)
	case ScreenNews:
		return m.handleNewsKeys(msg)
	case ScreenMessageBoard:
		return m.handleMessageBoardKeys(msg)
	case ScreenFiles:
		return m.handleFilesKeys(msg)
	case ScreenLogs:
		return m.handleLogsKeys(msg)
	case ScreenModal:
		return m.handleModalKeys(msg)
	case ScreenTasks:
		return m.handleTasksKeys(msg)
	case ScreenAccounts:
		return m.handleAccountsKeys(msg)
	}

	return m, nil
}

func (m *Model) View() string {
	// Render compose message modal if active
	if m.composeMsgModal {
		return m.renderComposeMsg()
	}

	// Render file picker modal if active
	if m.showFilePicker {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			subScreenStyle.Width(m.width-20).Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					subTitleStyle.Render("Select file to upload"),
					m.filePicker.View(),
				),
			),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("236")),
		)
	}

	switch m.currentScreen {
	case ScreenHome:
		return m.renderHome()
	case ScreenJoinServer:
		return m.renderJoinServer()
	case ScreenBookmarks:
		return m.renderBookmarks()
	case ScreenTracker:
		return m.renderTracker()
	case ScreenSettings:
		return m.renderSettingsOverlay()
	case ScreenServerUI:
		return m.renderServerUI()
	case ScreenNews:
		return m.renderNews()
	case ScreenNewsPost:
		return m.renderNewMsgBoardPost()
	case ScreenMessageBoard:
		return m.renderMessageBoard()
	case ScreenFiles:
		return m.renderFiles()
	case ScreenLogs:
		return m.renderLogs()
	case ScreenModal:
		return m.renderModal()
	case ScreenTasks:
		return m.renderTasks()
	case ScreenAccounts:
		return m.renderAccounts()
	}

	return ""
}

func (m *Model) initializeViewports() {
	// Account for borders in viewport sizes
	// Rounded border = 1 char each side = 2 total width, 2 total height
	const borderWidth = 2
	const borderHeight = 2

	// Chat viewport - subtract space for user list (30) and borders
	chatWidth := m.width - 30 - borderWidth
	chatHeight := m.height - 10 - borderHeight
	if chatWidth < 10 {
		chatWidth = 10
	}
	if chatHeight < 5 {
		chatHeight = 5
	}
	m.chatViewport = viewport.New(chatWidth, chatHeight)
	m.chatViewport.SetContent(m.chatContent)

	// Update chat input width to match chat viewport
	// Subtract additional padding for the input box border
	m.chatInput.Width = chatWidth - 4

	// User list viewport - subtract borders
	userWidth := 28 - borderWidth
	userHeight := m.height - 10 - borderHeight

	if userHeight < 5 {
		userHeight = 5
	}
	m.userViewport = viewport.New(userWidth, userHeight)

	m.newsViewport = viewport.New(m.width-10, m.height-10)
	m.newsViewport.KeyMap = viewport.DefaultKeyMap() // Enable default viewport key bindings

	m.messageBoardViewport = viewport.New(m.width-10, m.height-10)
	m.messageBoardViewport.KeyMap = viewport.DefaultKeyMap() // Enable default viewport key bindings

	m.logsViewport = viewport.New(m.width-10, m.height-10)
	m.logsViewport.KeyMap = viewport.DefaultKeyMap() // Enable default viewport key bindings

	// Account details viewport
	accountDetailWidth := (m.width / 2) - 4
	accountDetailHeight := m.height - 12
	m.accountsViewport = viewport.New(accountDetailWidth, accountDetailHeight)
}

func (m *Model) initiateFileUpload(localPath string) tea.Cmd {
	return func() tea.Msg {
		// Get file info
		fileInfo, err := os.Stat(localPath)
		if err != nil {
			return errorMsg{text: fmt.Sprintf("Failed to access file: %v", err)}
		}

		if fileInfo.IsDir() {
			return errorMsg{text: "Folder uploads not yet supported"}
		}

		fileName := filepath.Base(localPath)

		// Create task
		task := &Task{
			ID:         uuid.New().String(),
			FileName:   fileName,
			FilePath:   m.filePath, // Upload to current directory in Files screen
			Status:     TaskPending,
			TotalBytes: fileInfo.Size(),
			StartTime:  time.Now(),
			LocalPath:  localPath,
		}
		m.taskManager.Add(task)

		// Create upload transaction
		sizeBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBytes, uint32(fileInfo.Size()))

		fields := []hotline.Field{
			hotline.NewField(hotline.FieldFileName, []byte(fileName)),
			hotline.NewField(hotline.FieldTransferSize, sizeBytes),
		}

		// Add file path if uploading to subfolder
		if len(m.filePath) > 0 {
			pathStr := strings.Join(m.filePath, "/")
			pathBytes := hotline.EncodeFilePath(pathStr)
			fields = append(fields, hotline.NewField(hotline.FieldFilePath, pathBytes))
		}

		t := hotline.NewTransaction(hotline.TranUploadFile, [2]byte{}, fields...)

		// Map transaction ID to task ID
		m.pendingUploads[t.ID] = task.ID

		// Send transaction
		if err := m.hlClient.Send(t); err != nil {
			m.logger.Error("Failed to send upload transaction", "err", err)
			return errorMsg{text: fmt.Sprintf("Failed to initiate upload: %v", err)}
		}

		m.logger.Info("Upload initiated", "file", fileName, "size", fileInfo.Size())

		return taskStatusMsg{
			taskID: task.ID,
			status: TaskPending,
		}
	}
}

func (m *Model) Start() error {
	// Store program reference for sending messages from transaction handlers
	m.program = tea.NewProgram(m, tea.WithAltScreen())

	// Register transaction handlers
	m.hlClient.HandleFunc(hotline.TranChatMsg, m.HandleClientChatMsg)
	m.hlClient.HandleFunc(hotline.TranLogin, m.HandleClientTranLogin)
	m.hlClient.HandleFunc(hotline.TranShowAgreement, m.HandleClientTranShowAgreement)
	m.hlClient.HandleFunc(hotline.TranUserAccess, m.HandleClientTranUserAccess)
	m.hlClient.HandleFunc(hotline.TranGetUserNameList, m.HandleClientGetUserNameList)
	m.hlClient.HandleFunc(hotline.TranNotifyChangeUser, m.HandleNotifyChangeUser)
	m.hlClient.HandleFunc(hotline.TranNotifyChatDeleteUser, m.HandleNotifyDeleteUser)
	m.hlClient.HandleFunc(hotline.TranNotifyDeleteUser, m.HandleNotifyDeleteUser)
	m.hlClient.HandleFunc(hotline.TranGetMsgs, m.TranGetMsgs)
	m.hlClient.HandleFunc(hotline.TranGetFileNameList, m.HandleGetFileNameList)
	m.hlClient.HandleFunc(hotline.TranServerMsg, m.HandleTranServerMsg)
	m.hlClient.HandleFunc(hotline.TranKeepAlive, m.HandleKeepAlive)
	m.hlClient.HandleFunc(hotline.TranDownloadFile, m.HandleDownloadFile)
	m.hlClient.HandleFunc(hotline.TranUploadFile, m.HandleUploadFile)
	m.hlClient.HandleFunc(hotline.TranListUsers, m.HandleListUsers)
	m.hlClient.HandleFunc(hotline.TranGetNewsCatNameList, m.HandleGetNewsCatNameList)
	m.hlClient.HandleFunc(hotline.TranGetNewsArtNameList, m.HandleGetNewsArtNameList)
	m.hlClient.HandleFunc(hotline.TranGetNewsArtData, m.HandleGetNewsArtData)
	m.hlClient.HandleFunc(hotline.TranPostNewsArt, m.HandlePostNewsArt)
	m.hlClient.HandleFunc(hotline.TranNewNewsFldr, m.HandleNewNewsFldr)
	m.hlClient.HandleFunc(hotline.TranNewNewsCat, m.HandleNewNewsCat)

	_, err := m.program.Run()
	return err
}

func (m *Model) joinServer(addr, login, password string, useTLS bool) error {
	// Append default port to address if no port supplied
	if len(strings.Split(addr, ":")) == 1 {
		if useTLS {
			addr += ":5600"
		} else {
			addr += ":5500"
		}
	}

	var err error
	if useTLS {
		// Create TLS connection
		m.hlClient.Connection, err = tls.Dial("tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return fmt.Errorf("TLS connection error: %v", err)
		}

		// Perform handshake
		if err := m.hlClient.Handshake(); err != nil {
			return fmt.Errorf("handshake error: %v", err)
		}

		// Send login transaction
		err = m.hlClient.Send(
			hotline.NewTransaction(
				hotline.TranLogin, [2]byte{0, 0},
				hotline.NewField(hotline.FieldUserName, []byte(m.prefs.Username)),
				hotline.NewField(hotline.FieldUserIconID, m.prefs.IconBytes()),
				hotline.NewField(hotline.FieldUserLogin, hotline.EncodeString([]byte(login))),
				hotline.NewField(hotline.FieldUserPassword, hotline.EncodeString([]byte(password))),
			),
		)
		if err != nil {
			return fmt.Errorf("login error: %v", err)
		}
	} else {
		if err := m.hlClient.Connect(addr, login, password); err != nil {
			return fmt.Errorf("error joining server: %v", err)
		}
	}

	go func() {
		if err := m.hlClient.HandleTransactions(context.TODO()); err != nil {
			m.logger.Error("Transaction handler error", "err", err)
		}
	}()

	return nil
}

func (m *Model) savePreferences() error {
	out, err := yaml.Marshal(m.prefs)
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfgPath, out, 0666)
}

// wrapChatMessage wraps a chat message to fit within the chat viewport width.
// Handles ANSI-styled text (bold usernames) correctly using wordwrap library.
func (m *Model) wrapChatMessage(formattedMsg string) string {
	const borderWidth = 2
	const paddingLeft = 1 // From chatView rendering (PaddingLeft)

	chatWidth := m.width - 30 - borderWidth
	if chatWidth < 10 {
		chatWidth = 10
	}

	// Subtract padding to get usable content width
	wrapWidth := chatWidth - paddingLeft
	if wrapWidth < 5 {
		wrapWidth = 5 // Minimum for edge cases
	}

	// wordwrap.String handles ANSI codes correctly
	return wordwrap.String(formattedMsg, wrapWidth)
}

// rebuildChatContent rebuilds entire chat content from stored messages,
// applying word-wrapping based on current viewport width.
// Called when messages arrive or window resizes.
func (m *Model) rebuildChatContent() {
	var content strings.Builder

	for _, msg := range m.chatMessages {
		wrapped := m.wrapChatMessage(msg)
		content.WriteString(wrapped)
		content.WriteString("\n")
	}

	m.chatContent = content.String()
	m.chatViewport.SetContent(m.chatContent)
}

// submitAccountChanges submits account updates to the server
func (m *Model) submitAccountChanges() tea.Cmd {
	return func() tea.Msg {
		// Build sub-fields
		subFields := []hotline.Field{
			hotline.NewField(hotline.FieldUserLogin,
				hotline.EncodeString([]byte(m.editedLogin))),
			hotline.NewField(hotline.FieldUserName, []byte(m.editedName)),
			hotline.NewField(hotline.FieldUserAccess, m.editedAccessBits[:]),
		}

		// Handle password
		if m.passwordChanged {
			if len(m.editedPassword) > 0 {
				subFields = append(subFields,
					hotline.NewField(hotline.FieldUserPassword, []byte(m.editedPassword)))
			}
			// If password is empty and changed, don't include field (removes password)
		} else {
			// Keep existing password
			subFields = append(subFields,
				hotline.NewField(hotline.FieldUserPassword, []byte{0}))
		}

		// Serialize sub-fields
		var fieldData []byte
		subFieldCount := make([]byte, 2)
		binary.BigEndian.PutUint16(subFieldCount, uint16(len(subFields)))
		fieldData = append(fieldData, subFieldCount...)

		for _, field := range subFields {
			b, _ := io.ReadAll(&field)
			fieldData = append(fieldData, b...)
		}

		// Send transaction
		if err := m.hlClient.Send(hotline.NewTransaction(
			hotline.TranUpdateUser,
			[2]byte{},
			hotline.NewField(hotline.FieldData, fieldData),
		)); err != nil {
			m.logger.Error("Error updating account", "err", err)
			return errorMsg{text: fmt.Sprintf("Error updating account: %v", err)}
		}

		m.logger.Info("Account updated successfully")

		// Reset state and return to server UI
		m.selectedAccount = nil
		m.isNewAccount = false
		m.currentScreen = ScreenServerUI

		return nil
	}
}

// deleteAccount deletes the selected account from the server
func (m *Model) deleteAccount() tea.Cmd {
	return func() tea.Msg {
		// For delete, send only FieldData with the login
		loginData := hotline.EncodeString([]byte(m.selectedAccount.login))

		if err := m.hlClient.Send(hotline.NewTransaction(
			hotline.TranUpdateUser,
			[2]byte{},
			hotline.NewField(hotline.FieldData, loginData),
		)); err != nil {
			m.logger.Error("Error deleting account", "err", err)
			return errorMsg{text: fmt.Sprintf("Error deleting account: %v", err)}
		}

		m.logger.Info("Account deleted successfully")

		// Reset state and return to server UI
		m.selectedAccount = nil
		m.currentScreen = ScreenServerUI

		return nil
	}
}
