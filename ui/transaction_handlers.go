package ui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jhalter/mobius/hotline"
)

func (m *Model) HandleKeepAlive(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	return res, err
}

func (m *Model) HandleTranServerMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	now := time.Now().Format(time.RFC850)

	msg := strings.ReplaceAll(string(t.GetField(hotline.FieldData).Data), "\r", "\n")
	from := string(t.GetField(hotline.FieldUserName).Data)
	userIDField := t.GetField(hotline.FieldUserID)
	var userID [2]byte
	if len(userIDField.Data) >= 2 {
		copy(userID[:], userIDField.Data[:2])
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(serverMsgMsg{from: from, userID: userID, text: msg, time: now})

	return res, err
}

func (m *Model) HandleGetFileNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.ErrorCode == [4]byte{0, 0, 0, 1} {
		m.program.Send(errorMsg{text: string(t.GetField(hotline.FieldError).Data)})

		return res, err
	}

	var files []hotline.FileNameWithInfo
	for _, f := range t.Fields {
		var fn hotline.FileNameWithInfo
		_, err = fn.Write(f.Data)
		if err != nil {
			continue
		}
		files = append(files, fn)
	}

	m.program.Send(filesMsg{files: files})

	return res, err
}

func (m *Model) TranGetMsgs(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	messageBoardText := string(t.GetField(hotline.FieldData).Data)
	messageBoardText = strings.ReplaceAll(messageBoardText, "\r", "\n")

	// Send message to Bubble Tea program to update UI
	m.program.Send(messageBoardMsg{text: messageBoardText})

	return res, err
}

func (m *Model) HandleNotifyChangeUser(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	newUser := hotline.User{
		ID:    [2]byte(t.GetField(hotline.FieldUserID).Data),
		Name:  string(t.GetField(hotline.FieldUserName).Data),
		Icon:  t.GetField(hotline.FieldUserIconID).Data,
		Flags: t.GetField(hotline.FieldUserFlags).Data,
	}

	var oldName string
	var newUserList []hotline.User
	updatedUser := false

	for _, u := range m.userList {
		if newUser.ID == u.ID {
			oldName = u.Name
			if u.Name != newUser.Name {
				m.program.Send(chatMsg{text: fmt.Sprintf(" <<< %s is now known as %s >>>", oldName, newUser.Name)})
			}
			u = newUser
			updatedUser = true
		}
		newUserList = append(newUserList, u)
	}

	if !updatedUser {
		newUserList = append(newUserList, newUser)
		// Play sound for user joining
		if m.soundPlayer != nil {
			m.soundPlayer.PlayAsync(SoundUserJoin)
		}
		// Send join message to chat
		joinStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("241"))
		m.program.Send(chatMsg{text: joinStyle.Render(fmt.Sprintf("→ %s joined", newUser.Name))})
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(userListMsg{users: newUserList})

	return res, err
}

func (m *Model) HandleNotifyDeleteUser(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	exitUser := t.GetField(hotline.FieldUserID).Data

	// Find the username before removing
	var leavingUsername string
	var newUserList []hotline.User
	for _, u := range m.userList {
		if !bytes.Equal(exitUser, u.ID[:]) {
			newUserList = append(newUserList, u)
		} else {
			leavingUsername = u.Name
		}
	}

	// Play sound for user leaving
	if m.soundPlayer != nil {
		m.soundPlayer.PlayAsync(SoundUserLeave)
	}

	// Send leave message to chat
	if leavingUsername != "" {
		leaveStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("241"))
		m.program.Send(chatMsg{text: leaveStyle.Render(fmt.Sprintf("← %s left", leavingUsername))})
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(userListMsg{users: newUserList})

	return res, err
}

func (m *Model) HandleClientGetUserNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	var users []hotline.User
	for _, field := range t.Fields {
		if field.Type == hotline.FieldUsernameWithInfo {
			var user hotline.User
			if _, err := user.Write(field.Data); err != nil {
				return res, fmt.Errorf("unable to read user data: %w", err)
			}
			users = append(users, user)
		}
	}

	// Send message to Bubble Tea program to update UI
	m.program.Send(userListMsg{users: users})

	return res, err
}

func (m *Model) updateUserListDisplay() {
	var userListContent strings.Builder
	for _, u := range m.userList {
		flagBitmap := big.NewInt(int64(binary.BigEndian.Uint16(u.Flags)))
		isAdmin := flagBitmap.Bit(hotline.UserFlagAdmin) == 1
		isAway := flagBitmap.Bit(hotline.UserFlagAway) == 1

		userName := u.Name

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
}

func (m *Model) HandleClientChatMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	// Play sound for chat message
	if m.soundPlayer != nil {
		m.soundPlayer.PlayAsync(SoundChatMsg)
	}

	// Terminal bell (independent of sounds)
	if m.prefs.EnableBell {
		fmt.Print("\a")
	}

	chatText := string(t.GetField(hotline.FieldData).Data)
	chatText = strings.ReplaceAll(chatText, "\r", "")
	// Send message to Bubble Tea program to update UI
	m.program.Send(chatMsg{text: chatText})

	return res, err
}

func (m *Model) HandleClientTranUserAccess(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	copy(m.userAccess[:], t.GetField(hotline.FieldUserAccess).Data)
	m.logger.Debug("Permissions", "AccessNewsDeleteArt", m.userAccess.IsSet(hotline.AccessNewsDeleteArt))

	m.serverUIKeys.News.SetEnabled(m.userAccess.IsSet(hotline.AccessNewsReadArt))
	m.serverUIKeys.Accounts.SetEnabled(m.userAccess.IsSet(hotline.AccessModifyUser))

	return res, err
}

func (m *Model) HandleClientTranShowAgreement(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	agreement := string(t.GetField(hotline.FieldData).Data)
	agreement = strings.ReplaceAll(agreement, "\r", "\n")

	// Show agreement modal with Agree/Disagree options
	m.program.Send(agreementMsg{text: agreement})

	return res, err
}

func (m *Model) HandleClientTranLogin(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.ErrorCode != [4]byte{0, 0, 0, 0} {
		errMsg := string(t.GetField(hotline.FieldError).Data)

		m.program.Send(errorMsg{text: errMsg})

		m.logger.Error(errMsg)
		return nil, errors.New("login error: " + errMsg)
	}

	// Send server connected message with the name to display
	m.program.Send(serverConnectedMsg{name: m.pendingServerName})

	if err := c.Send(hotline.NewTransaction(hotline.TranGetUserNameList, [2]byte{})); err != nil {
		m.logger.Error("err", "err", err)
	}

	return res, err
}

func (m *Model) HandleDownloadFile(ctx context.Context, c *hotline.Client, t *hotline.Transaction) ([]hotline.Transaction, error) {
	refNum := t.GetField(hotline.FieldRefNum).Data
	transferSize := binary.BigEndian.Uint32(t.GetField(hotline.FieldTransferSize).Data)
	fileSize := binary.BigEndian.Uint32(t.GetField(hotline.FieldFileSize).Data)

	var refNumBytes [4]byte
	copy(refNumBytes[:], refNum)

	m.program.Send(downloadReplyMsg{
		txID:         t.ID,
		refNum:       refNumBytes,
		transferSize: transferSize,
		fileSize:     fileSize,
	})

	return nil, nil
}

func (m *Model) HandleUploadFile(ctx context.Context, c *hotline.Client, t *hotline.Transaction) ([]hotline.Transaction, error) {
	refNum := t.GetField(hotline.FieldRefNum).Data

	var refNumBytes [4]byte
	copy(refNumBytes[:], refNum)

	m.program.Send(uploadReplyMsg{
		txID:   t.ID,
		refNum: refNumBytes,
	})

	return nil, nil
}

func (m *Model) HandleListUsers(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.ErrorCode != [4]byte{0, 0, 0, 0} {
		m.program.Send(errorMsg{text: string(t.GetField(hotline.FieldError).Data)})
		return res, err
	}

	var accounts []accountItem

	// Each FieldData contains one account
	for i, field := range t.Fields {
		if field.Type != hotline.FieldData {
			continue
		}

		var acct accountItem
		acct.index = i

		// Parse sub-fields from FieldData using scanner
		scanner := bufio.NewScanner(bytes.NewReader(field.Data[2:]))
		scanner.Split(hotline.FieldScanner)

		fieldCount := int(binary.BigEndian.Uint16(field.Data[0:2]))

		// Read each sub-field
		for j := 0; j < fieldCount; j++ {
			if !scanner.Scan() {
				break
			}

			var subField hotline.Field
			if _, err := subField.Write(scanner.Bytes()); err != nil {
				m.logger.Error("Error reading sub-field", "err", err)
				break
			}

			switch subField.Type {
			case hotline.FieldUserLogin:
				acct.login = string(hotline.EncodeString(subField.Data))
			case hotline.FieldUserName:
				acct.name = string(subField.Data)
			case hotline.FieldUserAccess:
				if len(subField.Data) >= 8 {
					copy(acct.access[:], subField.Data)
				}
			case hotline.FieldUserPassword:
				acct.hasPass = len(subField.Data) > 0
			}
		}

		accounts = append(accounts, acct)
	}

	m.program.Send(accountListMsg{accounts: accounts})
	return res, err
}

func (m *Model) HandleGetNewsCatNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.ErrorCode == [4]byte{0, 0, 0, 1} {
		m.program.Send(errorMsg{text: string(t.GetField(hotline.FieldError).Data)})

		return res, err
	}

	var categories []newsItem

	// Parse category list data from transaction fields
	for _, f := range t.Fields {
		m.logger.Debug("type", "f.Type", f.Type, "catlist", f.Type == hotline.FieldNewsCatListData15)
		if f.Type == hotline.FieldNewsCatListData15 {
			var catData hotline.NewsCategoryListData15
			_, err = catData.Write(f.Data)
			if err != nil {
				m.logger.Error("Error parsing category data", "err", err)
				continue
			}

			// Determine if this is a bundle or category
			isBundle := bytes.Equal(catData.Type[:], hotline.NewsBundle[:])

			categories = append(categories, newsItem{
				name:     catData.Name,
				isBundle: isBundle,
			})
		}
	}

	// Send categories to UI
	m.program.Send(newsCategoriesMsg{categories: categories})

	return res, err
}

func (m *Model) HandleGetNewsArtNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.ErrorCode == [4]byte{0, 0, 0, 1} {
		m.program.Send(errorMsg{text: string(t.GetField(hotline.FieldError).Data)})
		return res, err
	}

	var articles []newsArticleItem

	// Get the NewsArtListData field
	artListField := t.GetField(hotline.FieldNewsArtListData)

	// Parse the NewsArtListData using the Write method
	var artListData hotline.NewsArtListData
	_, err = artListData.Write(artListField.Data)
	if err != nil {
		m.logger.Error("Error parsing article list data", "err", err)
		return res, err
	}

	// Parse individual articles from the NewsArtList payload
	data := artListData.NewsArtList
	offset := 0
	for i := 0; i < artListData.Count && offset < len(data); i++ {
		if offset+4 > len(data) {
			break
		}

		// Read article ID (4 bytes)
		articleID := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Read timestamp (8 bytes), parent ID (4 bytes), then skip flags (4 bytes) and flavor count (2 bytes)
		if offset+18 > len(data) {
			break
		}
		timestamp := [8]byte{}
		copy(timestamp[:], data[offset:offset+8])
		offset += 8

		// Read parent ID (4 bytes)
		parentID := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		offset += 4 + 2 // skip flags + flavor count

		// Read title (1 byte length + data)
		if offset+1 > len(data) {
			break
		}
		titleLen := int(data[offset])
		offset++
		if offset+titleLen > len(data) {
			break
		}
		title := string(data[offset : offset+titleLen])
		offset += titleLen

		// Read poster (1 byte length + data)
		if offset+1 > len(data) {
			break
		}
		posterLen := int(data[offset])
		offset++
		if offset+posterLen > len(data) {
			break
		}
		poster := string(data[offset : offset+posterLen])
		offset += posterLen

		// Skip flavor text (1 byte length + data) and article size (2 bytes)
		if offset+1 > len(data) {
			break
		}
		flavorLen := int(data[offset])
		offset++
		if offset+flavorLen+2 > len(data) {
			break
		}
		offset += flavorLen + 2 // skip flavor + article size

		articles = append(articles, newsArticleItem{
			id:          articleID,
			title:       title,
			poster:      poster,
			date:        timestamp,
			parentID:    parentID,
			depth:       0,
			isExpanded:  false,
			hasChildren: false,
		})
	}

	// Send articles to UI
	m.program.Send(newsArticlesMsg{articles: articles})

	return res, err
}

func (m *Model) HandleGetNewsArtData(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.ErrorCode == [4]byte{0, 0, 0, 1} {
		m.program.Send(errorMsg{text: string(t.GetField(hotline.FieldError).Data)})

		return res, err
	}

	// Parse article data from transaction fields
	var article hotline.NewsArtData
	article.Title = string(t.GetField(hotline.FieldNewsArtTitle).Data)
	article.Poster = string(t.GetField(hotline.FieldNewsArtPoster).Data)
	article.Data = string(t.GetField(hotline.FieldNewsArtData).Data)
	article.Data = strings.ReplaceAll(article.Data, "\r", "\n")

	dateField := t.GetField(hotline.FieldNewsArtDate)
	if len(dateField.Data) >= 8 {
		copy(article.Date[:], dateField.Data[:8])
	}

	// Send article to UI
	m.program.Send(newsArticleDataMsg{article: article})

	return res, err
}

func (m *Model) HandlePostNewsArt(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	// Check for errors
	if t.ErrorCode != [4]byte{0, 0, 0, 0} {
		errMsg := string(t.GetField(hotline.FieldError).Data)
		m.program.Send(errorMsg{text: "Failed to post article: " + errMsg})
		m.logger.Error("Error posting news article", "error", errMsg)
		return res, err
	}

	m.logger.Info("News article posted successfully")
	return res, err
}

func (m *Model) HandleNewNewsFldr(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	// Check for errors
	if t.ErrorCode != [4]byte{0, 0, 0, 0} {
		errMsg := string(t.GetField(hotline.FieldError).Data)
		m.program.Send(errorMsg{text: "Failed to create bundle: " + errMsg})

		m.logger.Error("Error creating news bundle", "error", errMsg)
		return res, err
	}

	m.logger.Info("News bundle created successfully")
	return res, err
}

func (m *Model) HandleNewNewsCat(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	m.program.Send(errorMsg{text: "Failed to create category: " + "errMsg"})
	// Check for errors
	if t.ErrorCode != [4]byte{0, 0, 0, 0} {
		errMsg := string(t.GetField(hotline.FieldError).Data)
		m.program.Send(errorMsg{text: "Failed to create category: " + errMsg})

		m.logger.Error("Error creating news category", "error", errMsg)
		return res, err
	}

	m.logger.Info("News category created successfully")
	return res, err
}
