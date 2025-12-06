//go:build !linux

package internal

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

//go:embed sounds/*.wav
var soundFS embed.FS

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

// SoundPlayer manages sound playback for various client events
type SoundPlayer struct {
	enabled bool
	sounds  map[SoundEvent]*beep.Buffer
	mu      sync.Mutex
}

// NewSoundPlayer creates and initializes a new sound player.
// Returns the SoundPlayer and any errors encountered while loading sound files.
// Speaker initialization failure is fatal; sound file load failures are non-fatal.
func NewSoundPlayer(enabled bool) (*SoundPlayer, error) {
	sp := &SoundPlayer{
		enabled: enabled,
		sounds:  make(map[SoundEvent]*beep.Buffer),
	}

	// Initialize the speaker (44.1kHz sample rate, 4096 buffer size)
	sampleRate := beep.SampleRate(44100)
	if err := speaker.Init(sampleRate, 4096); err != nil {
		return nil, fmt.Errorf("failed to initialize speaker: %w", err)
	}

	// Load sound files
	soundFiles := map[SoundEvent]string{
		SoundChatMsg:          "sounds/chat-message.wav",
		SoundUserJoin:         "sounds/user-login.wav",
		SoundUserLeave:        "sounds/user-logout.wav",
		SoundServerMsg:        "sounds/server-message.wav",
		SoundError:            "sounds/error.wav",
		SoundLoggedIn:         "sounds/logged-in.wav",
		SoundNewNews:          "sounds/new-news.wav",
		SoundTransferComplete: "sounds/transfer-complete.wav",
	}

	var loadErrs []error
	for event, filename := range soundFiles {
		if err := sp.loadSound(event, filename, sampleRate); err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %w", filename, err))
		}
	}

	return sp, errors.Join(loadErrs...)
}

// loadSound loads and decodes a WAV file into a buffer
func (sp *SoundPlayer) loadSound(event SoundEvent, filename string, sampleRate beep.SampleRate) error {
	file, err := soundFS.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open sound file: %w", err)
	}
	defer func() { _ = file.Close() }()

	streamer, format, err := wav.Decode(io.NopCloser(file))
	if err != nil {
		return fmt.Errorf("failed to decode WAV file: %w", err)
	}
	defer func() { _ = streamer.Close() }()

	// Resample if necessary to match speaker sample rate
	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	// Buffer the entire sound in memory
	buffer := beep.NewBuffer(format)
	buffer.Append(resampled)

	sp.sounds[event] = buffer
	return nil
}

// PlayAsync plays a sound asynchronously without blocking
func (sp *SoundPlayer) PlayAsync(event SoundEvent) {
	if !sp.enabled {
		return
	}

	sp.mu.Lock()
	buffer, exists := sp.sounds[event]
	sp.mu.Unlock()

	if !exists {
		return
	}

	// Play sound in a goroutine to avoid blocking
	go func() {
		streamer := buffer.Streamer(0, buffer.Len())
		done := make(chan bool)

		speaker.Play(beep.Seq(streamer, beep.Callback(func() {
			done <- true
		})))

		// Wait for playback to complete with timeout to prevent goroutine leak
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}()
}

// SetEnabled enables or disables sound playback
func (sp *SoundPlayer) SetEnabled(enabled bool) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.enabled = enabled
}

// Close cleans up the sound player resources
func (sp *SoundPlayer) Close() {
	speaker.Clear()
}
