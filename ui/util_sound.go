package ui

import (
	"embed"
	"fmt"
	"io"
	"log/slog"
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
	logger  *slog.Logger
}

// NewSoundPlayer creates and initializes a new sound player
func NewSoundPlayer(prefs *Settings, logger *slog.Logger) (*SoundPlayer, error) {
	sp := &SoundPlayer{
		enabled: prefs.EnableSounds,
		sounds:  make(map[SoundEvent]*beep.Buffer),
		logger:  logger,
	}

	// Initialize the speaker (44.1kHz sample rate, 4096 buffer size)
	sampleRate := beep.SampleRate(44100)
	err := speaker.Init(sampleRate, 4096)
	if err != nil {
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

	for event, filename := range soundFiles {
		if err := sp.loadSound(event, filename, sampleRate); err != nil {
			logger.Warn("Failed to load sound file", "file", filename, "error", err)
			// Continue loading other sounds even if one fails
		}
	}

	return sp, nil
}

// loadSound loads and decodes a WAV file into a buffer
func (sp *SoundPlayer) loadSound(event SoundEvent, filename string, sampleRate beep.SampleRate) error {
	file, err := soundFS.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open sound file: %w", err)
	}
	defer file.Close()

	streamer, format, err := wav.Decode(io.NopCloser(file))
	if err != nil {
		return fmt.Errorf("failed to decode WAV file: %w", err)
	}
	defer streamer.Close()

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
		sp.logger.Debug("Sound event not found", "event", event)
		return
	}

	// Play sound in a goroutine to avoid blocking
	go func() {
		streamer := buffer.Streamer(0, buffer.Len())
		done := make(chan bool)

		speaker.Play(beep.Seq(streamer, beep.Callback(func() {
			done <- true
		})))

		// Wait for playback to complete with timeout
		select {
		case <-done:
			// Playback completed
		case <-time.After(5 * time.Second):
			// Timeout to prevent goroutine leak
			sp.logger.Warn("Sound playback timeout", "event", event)
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
