//go:build linux

package internal

// SoundEvent represents different types of sound events in the client
type SoundEvent int

const (
	SoundChatMsg SoundEvent = iota
	SoundUserJoin
	SoundUserLeave
	SoundServerMsg
	SoundError
	SoundLoggedIn
	SoundNewNews
	SoundTransferComplete
)

// SoundPlayer is a no-op implementation for Linux where CGO is not available
type SoundPlayer struct{}

// NewSoundPlayer returns a no-op sound player on Linux
func NewSoundPlayer(_ bool) (*SoundPlayer, error) {
	return &SoundPlayer{}, nil
}

// PlayAsync is a no-op on Linux
func (sp *SoundPlayer) PlayAsync(_ SoundEvent) {}

// SetEnabled is a no-op on Linux
func (sp *SoundPlayer) SetEnabled(_ bool) {}

// Close is a no-op on Linux
func (sp *SoundPlayer) Close() {}
