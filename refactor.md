# Mobius Hotline Client - Code Organization Improvement Plan

## Overview

Reorganize the codebase from a flat structure with large monolithic files (screens.go: 2813 lines, ui.go: 1890 lines, Model: 140+ fields) into a domain-driven architecture using Go's `internal/` package idiom. The refactoring will be incremental, keeping the application working at each phase.

## Goals

1. **Reduce cognitive load** by grouping related functionality together
2. **Follow Go idioms** using internal packages and clear separation of concerns
3. **Decompose Model struct** from 140+ fields into focused domain sub-structs
4. **Maintain working application** at each refactoring step
5. **Improve code discoverability** making it clear where functionality lives

## Current Pain Points

- **ui.go (1890 lines)**: Model with 140+ fields, Update() handling 40+ message types, mixed concerns
- **screens.go (2813 lines)**: 70+ functions mixing rendering, key handling, list initialization, utilities
- **No domain boundaries**: News, files, accounts, chat logic scattered across multiple files
- **Unclear dependencies**: Hard to trace which functions need which Model fields
- **Single package**: Everything in `ui` package with no logical sub-organization

## Target Architecture

### Package Structure

```
mobius-hotline-client/
├── main.go                           # Entry point (unchanged)
├── internal/
│   ├── app/                          # Core application orchestration
│   │   ├── model.go                  # Root Model struct with domain composition
│   │   ├── update.go                 # Main Update() dispatcher
│   │   ├── view.go                   # Main View() dispatcher
│   │   ├── init.go                   # Initialization logic
│   │   ├── messages.go               # Bubble Tea message types
│   │   ├── config.go                 # Settings and configuration
│   │   └── styles.go                 # Global styles
│   │
│   ├── client/                       # Hotline protocol connection
│   │   ├── client.go                 # Wrapper for hotline.Client
│   │   ├── connection.go             # Connection management
│   │   └── handlers.go               # Transaction handler registration
│   │
│   ├── screen/                       # Screen navigation and base screens
│   │   ├── types.go                  # Screen enum and types
│   │   ├── home.go                   # Home screen
│   │   ├── join.go                   # Join server form
│   │   ├── bookmarks.go              # Bookmarks screen
│   │   ├── tracker.go                # Tracker screen
│   │   ├── settings.go               # Settings screen
│   │   └── modal.go                  # Modal dialog utilities
│   │
│   ├── chat/                         # Chat domain
│   │   ├── state.go                  # ChatState struct
│   │   ├── view.go                   # Chat rendering (viewport, messages)
│   │   ├── input.go                  # Chat input handling
│   │   ├── keys.go                   # Chat key bindings
│   │   ├── users.go                  # User list rendering and selection
│   │   ├── compose.go                # Private message composition
│   │   ├── handlers.go               # Chat transaction handlers
│   │   └── formatting.go             # Message formatting utilities
│   │
│   ├── news/                         # Threaded news domain
│   │   ├── state.go                  # NewsState struct
│   │   ├── view.go                   # News list/article rendering
│   │   ├── keys.go                   # News navigation key handling
│   │   ├── navigation.go             # Path navigation, tree manipulation
│   │   ├── forms.go                  # Post/reply/bundle/category forms
│   │   ├── article.go                # Article viewing and selection
│   │   ├── handlers.go               # News transaction handlers
│   │   └── types.go                  # newsItem, newsArticleItem, etc.
│   │
│   ├── files/                        # File management domain
│   │   ├── state.go                  # FilesState struct
│   │   ├── view.go                   # File list rendering
│   │   ├── keys.go                   # File browser key handling
│   │   ├── browser.go                # Directory navigation
│   │   ├── picker.go                 # File picker for uploads
│   │   ├── transfer.go               # Download/upload protocol (from file_transfer.go)
│   │   ├── tasks.go                  # Task tracking (from tasks.go)
│   │   ├── handlers.go               # File transaction handlers
│   │   └── types.go                  # File list items and types
│   │
│   ├── accounts/                     # Account management domain
│   │   ├── state.go                  # AccountsState struct
│   │   ├── view.go                   # Account list and editor rendering
│   │   ├── keys.go                   # Account editor key handling
│   │   ├── editor.go                 # Account editing logic
│   │   ├── permissions.go            # Permission/access bit UI
│   │   ├── handlers.go               # Account transaction handlers
│   │   └── types.go                  # accountItem, selectedAccountData
│   │
│   └── util/                         # Shared utilities
│       ├── sound.go                  # Sound player (from sound.go)
│       ├── debug.go                  # Debug buffer (from debug_buffer.go)
│       ├── banner.go                 # Banner selection (from banner.go)
│       └── format.go                 # Common formatting utilities
│
└── ui/                               # Deprecated - to be removed after migration
    └── (existing files remain during transition)
```

### Model Decomposition

**Before (140+ fields in one struct):**
```go
type Model struct {
    // 140+ fields covering all concerns
}
```

**After (composition of focused sub-structs):**
```go
// internal/app/model.go
type Model struct {
    // Core infrastructure
    program *tea.Program
    client  *client.Client
    config  *Config
    logger  *slog.Logger
    debug   *util.DebugBuffer
    sound   *util.SoundPlayer

    // Screen management
    currentScreen  screen.Screen
    previousScreen screen.Screen
    width          int
    height         int

    // Domain state (10-25 fields each)
    Chat     *chat.State
    News     *news.State
    Files    *files.State
    Accounts *accounts.State

    // Screen-specific state
    Home      *screen.HomeState
    Join      *screen.JoinState
    Bookmarks *screen.BookmarksState
    Tracker   *screen.TrackerState
    Settings  *screen.SettingsState
}

// internal/chat/state.go
type State struct {
    Viewport        viewport.Model
    Input           textinput.Model
    UserViewport    viewport.Model
    Messages        []string
    UserList        []hotline.User
    WasAtBottom     bool
    FocusOnUserList bool
    SelectedUserIdx int
    ComposeMsgInput textinput.Model
    ComposeMsgModal bool
    ComposeMsgTarget [2]byte
    ComposeMsgQuote  string
    // ~13 fields total
}

// internal/news/state.go
type State struct {
    List              list.Model
    Path              []string
    IsViewingCategory bool
    ShowModal         bool
    SelectedArticle   *SelectedArticle
    PendingArticleID  uint32
    ArticleViewport   viewport.Model
    AllArticles       []ArticleItem
    ExpandedArticles  map[uint32]bool
    PostForm          *huh.Form
    ArticlePostForm   *huh.Form
    BundleForm        *huh.Form
    CategoryForm      *huh.Form
    ReplyParentID     uint32
    // ~14 fields total
}

// internal/files/state.go
type State struct {
    List           list.Model
    Path           []string
    ShowModal      bool
    Picker         filepicker.Model
    ShowPicker     bool
    LastPickerLoc  string
    TaskManager    *TaskManager
    DownloadDir    string
    PendingDownloads map[[4]byte]string
    PendingUploads   map[[4]byte]string
    TaskProgress     map[string]progress.Model
    // ~11 fields total
}

// internal/accounts/state.go
type State struct {
    List              list.Model
    AllAccounts       []AccountItem
    Selected          *SelectedAccount
    ShowModal         bool
    DetailFocused     bool
    EditedAccessBits  hotline.AccessBitmap
    EditedName        string
    EditedLogin       string
    EditedPassword    string
    FocusedAccessBit  int
    Viewport          viewport.Model
    IsNewAccount      bool
    PasswordChanged   bool
    // ~13 fields total
}
```

## Incremental Refactoring Phases

### Phase 1: Create Package Structure (No Breaking Changes)

**Goal**: Set up internal/ directory and create empty package files

**Steps**:
1. Create internal/ directory structure with all subdirectories
2. Create empty placeholder files in each package:
   - state.go files with empty State structs
   - view.go, keys.go, handlers.go with package declarations
3. Add package documentation comments
4. Run `go build` to ensure packages compile

**Files Created** (all empty stubs):
- internal/app/model.go, update.go, view.go, init.go, messages.go, config.go, styles.go
- internal/client/client.go, connection.go, handlers.go
- internal/screen/types.go, home.go, join.go, bookmarks.go, tracker.go, settings.go, modal.go
- internal/chat/state.go, view.go, input.go, keys.go, users.go, compose.go, handlers.go, formatting.go
- internal/news/state.go, view.go, keys.go, navigation.go, forms.go, article.go, handlers.go, types.go
- internal/files/state.go, view.go, keys.go, browser.go, picker.go, transfer.go, tasks.go, handlers.go, types.go
- internal/accounts/state.go, view.go, keys.go, editor.go, permissions.go, handlers.go, types.go
- internal/util/sound.go, debug.go, banner.go, format.go

**Test**: `go build` succeeds, existing ui/ package still works

---

### Phase 2: Move Utility Packages (Bottom-Up)

**Goal**: Move self-contained utilities with no dependencies on Model

**Steps**:
1. Move ui/sound.go → internal/util/sound.go
2. Move ui/debug_buffer.go → internal/util/debug.go
3. Move ui/banner.go → internal/util/banner.go
4. Extract formatting utilities from screens.go → internal/util/format.go:
   - formatSpeed()
   - encodeNewsPath()
   - fileTypeEmoji()
   - Other pure formatting functions
5. Update imports in ui/ package to reference internal/util

**Files Modified**:
- ui/ui.go: Import internal/util, change types
- ui/screens.go: Import internal/util for formatSpeed, etc.
- main.go: Import internal/util for DebugBuffer

**Test**: Run application, verify sound playback, debug logging work

---

### Phase 3: Extract Domain State Structs

**Goal**: Define State structs in each domain package with fields from Model

**Steps**:

1. **internal/chat/state.go**: Define ChatState with 13 fields from Model
   - chatViewport, chatInput, userViewport
   - chatContent, chatMessages, chatWasAtBottom
   - focusOnUserList, selectedUserIdx
   - composeMsgInput, composeMsgModal, composeMsgTarget, composeMsgQuote
   - serverUIKeys, serverUIHelp

2. **internal/news/state.go**: Define NewsState with 14 fields
   - newsList, newsPath, isViewingCategory, showNewsModal
   - selectedArticle, pendingArticleID, articleViewport
   - allArticles, expandedArticles
   - newsPostForm, newsArticlePostForm, newsBundleForm, newsCategoryForm
   - replyParentArticleID

3. **internal/files/state.go**: Define FilesState with 11 fields
   - fileList, filePath, showFilesModal
   - filePicker, showFilePicker, lastPickerLocation
   - taskManager, downloadDir
   - pendingDownloads, pendingUploads, taskProgress

4. **internal/accounts/state.go**: Define AccountsState with 13 fields
   - accountList, allAccounts, selectedAccount
   - showAccountsModal, accountDetailFocused
   - editedAccessBits, editedName, editedLogin, editedPassword
   - focusedAccessBit, accountsViewport
   - isNewAccount, passwordChanged

5. **internal/screen/types.go**: Define screen-specific states
   - JoinState (12 fields): nameInput, serverInput, loginInput, passwordInput, useTLS, saveBookmark, focusIndex, backPage, editingBookmark, editingBookmarkIndex, creatingBookmark, pendingServerName
   - BookmarksState: bookmarkList, allBookmarks
   - TrackerState: trackerList, allTrackers
   - SettingsState: usernameInput, iconIDInput, trackerInput, downloadDirInput, enableBell, enableSounds
   - HomeState: (minimal, mostly navigation)

6. Add constructor functions (NewChatState(), NewNewsState(), etc.) in each package

7. **Do NOT modify ui/ui.go Model yet** - these are just type definitions

**Files Created**:
- internal/chat/state.go with State struct and NewState()
- internal/news/state.go with State struct and NewState()
- internal/files/state.go with State struct and NewState()
- internal/accounts/state.go with State struct and NewState()
- internal/screen/types.go with screen state structs

**Test**: `go build` succeeds

---

### Phase 4: Move Type Definitions

**Goal**: Move custom types (list items, message types) to appropriate packages

**Steps**:

1. **internal/screen/types.go**: Move from ui/ui.go
   - Screen enum constants
   - Bookmark struct
   - bookmarkItem, trackerItem types

2. **internal/news/types.go**: Move from ui/screens.go
   - newsItem, newsArticleItem, selectedArticleData types
   - List item delegates for news

3. **internal/files/types.go**: Move from ui/screens.go
   - fileItem type
   - List item delegates for files
   - Move TaskManager, Task, TaskStatus from ui/tasks.go

4. **internal/accounts/types.go**: Move from ui/screens.go
   - accountItem, selectedAccountData types
   - List item delegates for accounts

5. **internal/app/messages.go**: Move from ui/ui.go (lines 35-373)
   - All Bubble Tea message types (serverMsgMsg, notifyUserChangeMsg, etc.)
   - Keep in app package as they're used by Update() dispatcher

6. Update imports in ui/ package to use new type locations

**Files Modified**:
- ui/ui.go: Import new type packages, remove old definitions
- ui/screens.go: Import new type packages, remove old definitions

**Test**: `go build` succeeds, application runs

---

### Phase 5: Create New Model in internal/app

**Goal**: Define new composed Model struct using domain states

**Steps**:

1. **internal/app/model.go**: Create new Model struct
   ```go
   type Model struct {
       program        *tea.Program
       client         *client.Client
       config         *Config
       logger         *slog.Logger
       debugBuffer    *util.DebugBuffer
       soundPlayer    *util.SoundPlayer

       currentScreen  screen.Screen
       previousScreen screen.Screen
       width          int
       height         int
       welcomeBanner  string

       Chat     *chat.State
       News     *news.State
       Files    *files.State
       Accounts *accounts.State

       Join      *screen.JoinState
       Bookmarks *screen.BookmarksState
       Tracker   *screen.TrackerState
       Settings  *screen.SettingsState
   }
   ```

2. **internal/app/init.go**: Create NewModel() constructor
   - Initialize all domain states
   - Call domain constructors (chat.NewState(), etc.)
   - Set up Bubble Tea infrastructure
   - Return composed Model

3. **internal/app/config.go**: Move Settings struct from ui/settings.go
   - Keep config file loading logic
   - Add GetConfig() method

4. **internal/app/styles.go**: Move global styles from ui/ui.go
   - serverTitleStyle, subTitleStyle, etc.
   - Keep styles centralized for now

5. **DO NOT modify main.go yet** - old ui.Model still works

**Files Created**:
- internal/app/model.go (new Model definition)
- internal/app/init.go (NewModel constructor)
- internal/app/config.go (Settings moved)
- internal/app/styles.go (global styles moved)

**Test**: `go build` succeeds with both models present

---

### Phase 6: Move Domain Rendering (Domain by Domain)

**Goal**: Extract rendering functions from screens.go to domain packages

#### Phase 6a: Chat Domain

**Steps**:
1. **internal/chat/view.go**: Move from ui/screens.go
   - renderServerUI() - main chat view
   - renderChatViewport()
   - renderUserList()
   - formatChatMessage() - keep message formatting logic

2. **internal/chat/compose.go**: Move from ui/screens.go
   - renderComposeMsg() - private message modal
   - Related composition logic

3. **internal/chat/users.go**: Move from ui/screens.go
   - renderUserListContent()
   - User selection and highlighting

4. **internal/chat/formatting.go**: Move from ui/ui.go
   - formatChatMessage()
   - formatTime()
   - Other chat-specific formatters

5. Update function signatures to accept `*State` instead of `*Model`
   - Keep methods that need app.Model as methods on app.Model
   - Pure view functions take State only

**Files Modified**:
- internal/chat/view.go, compose.go, users.go, formatting.go (new content)
- ui/screens.go (remove moved functions, add imports)

#### Phase 6b: News Domain

**Steps**:
1. **internal/news/view.go**: Move from ui/screens.go
   - renderNews() - main news list
   - renderNewsArticle() - article viewer
   - renderNewsContent()

2. **internal/news/article.go**: Move from ui/screens.go
   - Article selection and display logic
   - selectedArticleData management

3. **internal/news/forms.go**: Move from ui/screens.go
   - initNewsArticlePostForm()
   - initNewsBundleForm()
   - initNewsCategoryForm()
   - Form rendering and handling

4. **internal/news/navigation.go**: Move from ui/screens.go
   - navigateNewsPath()
   - filterVisibleArticles()
   - scrollToFocusedArticle()
   - Tree manipulation functions

**Files Modified**:
- internal/news/view.go, article.go, forms.go, navigation.go (new content)
- ui/screens.go (remove moved functions)

#### Phase 6c: Files Domain

**Steps**:
1. **internal/files/view.go**: Move from ui/screens.go
   - renderFiles() - file browser
   - renderFileList()
   - renderTaskProgress()

2. **internal/files/browser.go**: Move from ui/screens.go
   - Directory navigation logic
   - Path management
   - File list initialization

3. **internal/files/picker.go**: Move from ui/screens.go
   - File picker integration
   - Upload file selection

4. **internal/files/transfer.go**: Move from ui/file_transfer.go
   - performFileTransfer()
   - performFileUpload()
   - copyWithProgress()
   - AppleDouble handling

5. **internal/files/tasks.go**: Move TaskManager from ui/tasks.go
   - Already have types defined
   - Move implementation

**Files Modified**:
- internal/files/view.go, browser.go, picker.go, transfer.go, tasks.go (new content)
- ui/screens.go, ui/file_transfer.go (remove moved functions)

#### Phase 6d: Accounts Domain

**Steps**:
1. **internal/accounts/view.go**: Move from ui/screens.go
   - renderAccounts() - account list and editor
   - renderAccountList()
   - renderAccountDetail()

2. **internal/accounts/editor.go**: Move from ui/screens.go
   - Account editing logic
   - submitAccountChanges()
   - deleteAccount()

3. **internal/accounts/permissions.go**: Move from ui/screens.go
   - renderAccountAccessBits()
   - scrollToFocusedCheckbox()
   - Permission checkbox UI

**Files Modified**:
- internal/accounts/view.go, editor.go, permissions.go (new content)
- ui/screens.go (remove moved functions)

#### Phase 6e: Screen Package

**Steps**:
1. **internal/screen/home.go**: Move from ui/screens.go
   - renderHome()
   - Home screen menu

2. **internal/screen/join.go**: Move from ui/screens.go
   - renderJoinServer()
   - Server connection form

3. **internal/screen/bookmarks.go**: Move from ui/screens.go
   - renderBookmarks()
   - Bookmark list and management
   - initializeBookmarkList()

4. **internal/screen/tracker.go**: Move from ui/screens.go
   - renderTracker()
   - Tracker server list
   - initializeTrackerList()

5. **internal/screen/settings.go**: Move from ui/screens.go
   - renderSettings()
   - Settings form

6. **internal/screen/modal.go**: Move from ui/screens.go
   - renderModal()
   - Generic modal dialog

**Files Modified**:
- internal/screen/home.go, join.go, bookmarks.go, tracker.go, settings.go, modal.go (new content)
- ui/screens.go (remove moved functions)

**Test After Phase 6**: `go build` succeeds, both Model versions coexist

---

### Phase 7: Move Key Handling (Domain by Domain)

**Goal**: Extract key handlers from screens.go to domain packages

**Steps**:

1. **internal/chat/keys.go**: Move from ui/screens.go
   - handleServerUIKeys()
   - serverUIKeyMap definition
   - Chat input key handling
   - User list navigation keys

2. **internal/chat/input.go**: Move from ui/ui.go
   - Chat message sending logic
   - Input field management

3. **internal/news/keys.go**: Move from ui/screens.go
   - handleNewsKeys()
   - Article navigation keys
   - Thread expansion/collapse

4. **internal/files/keys.go**: Move from ui/screens.go
   - handleFilesKeys()
   - Directory navigation keys
   - File selection and download

5. **internal/accounts/keys.go**: Move from ui/screens.go
   - handleAccountsKeys()
   - Checkbox navigation
   - Account editor keys

6. **internal/screen/keys.go**: Create for screen-specific handlers
   - handleHomeKeys()
   - handleJoinServerKeys()
   - handleBookmarksKeys()
   - handleTrackerKeys()
   - handleSettingsKeys()

**Files Modified**:
- internal/chat/keys.go, input.go (new content)
- internal/news/keys.go (new content)
- internal/files/keys.go (new content)
- internal/accounts/keys.go (new content)
- internal/screen/keys.go (new content)
- ui/screens.go (remove moved functions)

**Test**: `go build` succeeds

---

### Phase 8: Move Transaction Handlers

**Goal**: Move protocol handlers from ui/transaction_handlers.go to domain packages

**Steps**:

1. **internal/client/client.go**: Create client wrapper
   - Wrap hotline.Client
   - Provide connection interface
   - Keep as methods that need access to app.Model

2. **internal/chat/handlers.go**: Move from ui/transaction_handlers.go
   - HandleTranServerMsg()
   - HandleNotifyChangeUser()
   - HandleNotifyDeleteUser()
   - These need to call m.program.Send(), so remain as methods on app.Model
   - But organize by domain for clarity

3. **internal/news/handlers.go**: Move from ui/transaction_handlers.go
   - HandleGetNewsCatNameList()
   - HandleGetNewsArtNameList()
   - HandleGetNewsArtData()
   - TranPostNewsArt()

4. **internal/files/handlers.go**: Move from ui/transaction_handlers.go
   - HandleGetFileNameList()
   - HandleDownloadFile()
   - HandleUploadFile()
   - HandleSetFileInfo()
   - HandleMoveFile()
   - HandleMakeFileAlias()
   - HandleDeleteFile()
   - HandleNewFolder()
   - HandleGetFileInfo()

5. **internal/accounts/handlers.go**: Move from ui/transaction_handlers.go
   - TranGetUserNameList()
   - HandleGetUser()
   - HandleSetUser()
   - HandleNewUser()
   - HandleDeleteUser()

6. **internal/client/handlers.go**: Register all handlers
   - Keep handler registration centralized
   - RegisterHandlers() function

**Files Modified**:
- internal/chat/handlers.go (handlers moved, still methods on app.Model)
- internal/news/handlers.go (handlers moved)
- internal/files/handlers.go (handlers moved)
- internal/accounts/handlers.go (handlers moved)
- internal/client/handlers.go (registration)
- ui/transaction_handlers.go (remove moved functions)

**Note**: Handlers remain as methods on app.Model because they need access to m.program.Send() for Bubble Tea messaging. They're organized by domain for clarity, not for decoupling.

**Test**: Run application, verify server communication works

---

### Phase 9: Update Main Update() and View() Dispatchers

**Goal**: Rewrite Update() and View() in internal/app to use domain packages

**Steps**:

1. **internal/app/update.go**: Rewrite Update() function
   - Message routing simplified by delegating to domains
   - Chat messages → chat.Update(m.Chat, msg)
   - News messages → news.Update(m.News, msg)
   - Files messages → files.Update(m.Files, msg)
   - Accounts messages → accounts.Update(m.Accounts, msg)
   - Screen changes handled at app level
   - Each domain package gets Update(state *State, msg tea.Msg) function

2. **internal/app/view.go**: Rewrite View() function
   - Screen dispatching simplified
   - ScreenServerUI → chat.View(m.Chat)
   - ScreenNews → news.View(m.News)
   - ScreenFiles → files.View(m.Files)
   - ScreenAccounts → accounts.View(m.Accounts)
   - ScreenHome → screen.ViewHome()
   - Modal overlays handled at app level

3. Add Update() functions in each domain:
   - **internal/chat/update.go**: Handle chat-specific messages
   - **internal/news/update.go**: Handle news-specific messages
   - **internal/files/update.go**: Handle file-specific messages
   - **internal/accounts/update.go**: Handle account-specific messages

4. Keep Init() in internal/app/init.go
   - Initialize Bubble Tea
   - Return initial Cmd batch

**Files Created**:
- internal/app/update.go (new simplified Update)
- internal/app/view.go (new simplified View)
- internal/chat/update.go (domain Update)
- internal/news/update.go (domain Update)
- internal/files/update.go (domain Update)
- internal/accounts/update.go (domain Update)

**Files Modified**:
- ui/ui.go: Keep old Update() for transition

**Test**: `go build` succeeds with both implementations

---

### Phase 10: Switch main.go to Use internal/app

**Goal**: Update entry point to use new Model, deprecate ui/ package

**Steps**:

1. **main.go**: Update imports
   ```go
   import (
       "github.com/jhalter/mobius-hotline-client/internal/app"
       "github.com/jhalter/mobius-hotline-client/internal/util"
   )
   ```

2. Change initialization:
   ```go
   // Old:
   // model := ui.NewModel(*configDir, logger, db)

   // New:
   model := app.NewModel(*configDir, logger, db)
   ```

3. Test application thoroughly:
   - Connect to server
   - Chat functionality
   - News reading/posting
   - File browsing/download/upload
   - Account management
   - All screens and modals

4. Once verified working, delete ui/ package entirely:
   ```bash
   rm -rf ui/
   ```

5. Update go.mod if needed

**Files Modified**:
- main.go (switch to internal/app)

**Files Deleted**:
- ui/ui.go (1890 lines) → replaced by internal/app/* and domain packages
- ui/screens.go (2813 lines) → replaced by domain view/key files
- ui/transaction_handlers.go → replaced by domain handlers
- ui/file_transfer.go → replaced by internal/files/transfer.go
- ui/tasks.go → replaced by internal/files/tasks.go
- ui/sound.go → moved to internal/util/sound.go
- ui/debug_buffer.go → moved to internal/util/debug.go
- ui/banner.go → moved to internal/util/banner.go
- ui/settings.go → moved to internal/app/config.go

**Test**: Full application test, all features working

---

## Implementation Guidelines

### Go Idioms to Follow

1. **Internal packages**: Use internal/ to prevent external imports
2. **Package naming**: Short, lowercase, singular (chat not chats, news not newsboard)
3. **File naming**: Descriptive, grouped by purpose (state.go, view.go, keys.go)
4. **Exported vs unexported**: State structs exported, helpers unexported where possible
5. **Constructor pattern**: New*() functions for initialization
6. **Method receivers**: Use pointer receivers consistently for State
7. **Organize imports**: stdlib, external, internal groups

### Maintaining Bubble Tea Pattern

1. **Model interface**: Keep tea.Model interface on app.Model
2. **Message passing**: Use m.program.Send() for async updates
3. **Cmd batching**: Use tea.Batch for multiple commands
4. **Update pattern**: Each domain can have Update() but return (state, cmd), not (model, cmd)
5. **View composition**: Views return strings, composed by app.View()

### Handling Dependencies

1. **App → Domain**: App packages can import any domain
2. **Domain → Domain**: Minimize cross-domain imports, use app.Model for communication
3. **Domain → Util**: Domains can import util freely
4. **No circular imports**: Strictly enforce with go build

### Testing Strategy

After each phase:
1. `go build` - Verify compilation
2. Run application - Test basic functionality
3. Test affected domain - Verify specific features work
4. Check logs - Look for errors or warnings

Final testing checklist:
- [ ] Connect to test server
- [ ] Send/receive chat messages
- [ ] Navigate news threads
- [ ] Post news article
- [ ] Browse files
- [ ] Download file
- [ ] Upload file
- [ ] View account list
- [ ] Edit account permissions
- [ ] Test all modals
- [ ] Verify sound playback
- [ ] Check settings persistence
- [ ] Test bookmarks
- [ ] Test tracker

## Critical Files for Implementation

### Must Read Before Starting

1. **ui/ui.go (lines 375-511)** - Model struct field definitions
2. **ui/ui.go (lines 737-1427)** - Update() message routing logic
3. **ui/screens.go** - Understand all rendering functions and their dependencies
4. **ui/transaction_handlers.go** - Handler pattern and m.program.Send() usage
5. **ui/tasks.go** - Example of well-structured component

### Will Be Modified/Created

1. **main.go** - Update imports (Phase 10)
2. **internal/app/model.go** - New composed Model (Phase 5)
3. **internal/app/update.go** - Simplified Update() (Phase 9)
4. **internal/app/view.go** - Simplified View() (Phase 9)
5. **internal/*/state.go** - Domain state definitions (Phase 3)
6. **internal/*/view.go** - Domain rendering (Phase 6)
7. **internal/*/keys.go** - Domain key handling (Phase 7)
8. **internal/*/handlers.go** - Transaction handlers (Phase 8)

## Expected Outcomes

### Code Metrics

- **ui.go**: 1890 lines → ~300 lines across internal/app/model.go, update.go, view.go, init.go
- **screens.go**: 2813 lines → ~200-400 lines per domain package (8 packages × ~300 lines average)
- **Model fields**: 140+ → ~20 in app.Model + ~10-15 per domain State
- **Total files**: 10 → ~50 (more files, but each much smaller and focused)

### Developer Experience

- **Finding code**: "Where's news posting?" → internal/news/forms.go (obvious)
- **Understanding state**: Look at news.State struct (14 fields) instead of Model (140 fields)
- **Adding features**: Modify only affected domain package, clear boundaries
- **Code review**: Smaller, focused files easier to review
- **Testing**: Can unit test domains independently (future enhancement)
- **Onboarding**: New developers can understand one domain at a time

### Maintainability

- **Single Responsibility**: Each file has clear, focused purpose
- **Discoverability**: Package names indicate contents
- **Dependencies**: Clear import statements show relationships
- **Extensibility**: Add new domains without touching existing code
- **Testability**: Domains can be tested with mock States

## Risks and Mitigations

### Risk 1: Breaking Working Code

**Mitigation**: Incremental approach, test after each phase, keep old ui/ package until Phase 10

### Risk 2: Transaction Handlers Need app.Model

**Mitigation**: Keep handlers as methods on app.Model (organized by domain for clarity), don't try to fully decouple them

### Risk 3: Circular Import Dependencies

**Mitigation**: Strict bottom-up migration (util → types → state → view → keys → handlers), app is top-level orchestrator

### Risk 4: Bubble Tea State Management

**Mitigation**: Keep app.Model as single source of truth, domain States are composed fields, all updates flow through app.Update()

### Risk 5: Large Initial Time Investment

**Mitigation**: Can stop at any phase if working code is achieved, phases are checkpointed, incremental value delivered

## Alternative Considered: Light Refactoring

A lighter approach (splitting files within ui/ package without internal/ structure) was considered but rejected because:

1. Doesn't solve the Model complexity (140 fields)
2. Still a flat structure, not organized by domain
3. Less Go-idiomatic (no internal/ packages)
4. Doesn't provide clear boundaries for future growth
5. Similar effort with less long-term benefit

The medium approach provides better long-term maintainability with manageable refactoring scope.

## Summary

This plan transforms the Mobius Hotline Client from a monolithic structure (2 files with 4700 lines, 140-field Model) into a well-organized domain-driven architecture following Go idioms. The incremental approach ensures the application remains working throughout the refactoring process while delivering clear improvements in code organization, discoverability, and maintainability.
