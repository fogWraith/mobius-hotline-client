package ui

import "encoding/binary"

type Settings struct {
	Username     string     `yaml:"Username"`
	IconID       int        `yaml:"IconID"`
	Bookmarks    []Bookmark `yaml:"Bookmarks"`
	Tracker      string     `yaml:"Tracker"`
	EnableBell   bool       `yaml:"EnableBell"`
	EnableSounds bool       `yaml:"EnableSounds"`
	DownloadDir  string     `yaml:"DownloadDir"`
}

func (cp *Settings) IconBytes() []byte {
	iconBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(iconBytes, uint16(cp.IconID))
	return iconBytes
}

func (cp *Settings) AddBookmark(_, addr, login, pass string, useTLS bool) {
	cp.Bookmarks = append(cp.Bookmarks, Bookmark{Addr: addr, Login: login, Password: pass, TLS: useTLS})
}
