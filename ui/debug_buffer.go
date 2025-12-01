package ui

import "strings"

// DebugBuffer wraps a buffer for logging
type DebugBuffer struct {
	content strings.Builder
}

func (db *DebugBuffer) Write(p []byte) (int, error) {
	return db.content.Write(p)
}

func (db *DebugBuffer) String() string {
	return db.content.String()
}
