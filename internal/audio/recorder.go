package audio

import (
	"errors"
	"os"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

var _ app.Recorder = (*FFmpegRecorder)(nil)

var (
	ErrAlreadyRecording      = errors.New("ffmpeg recorder already active")
	ErrNotRecording          = errors.New("ffmpeg recorder is not active")
	ErrUnsupportedPlatform   = errors.New("ffmpeg microphone recording is only configured for macOS in this slice")
	ErrMissingOutputFile     = errors.New("ffmpeg did not produce an audio file")
	ErrUnsupportedAudioInput = errors.New("ffmpeg recorder supports microphone input only in this slice")
)

type RecorderOption func(*FFmpegRecorder)

func WithCommandPath(path string) RecorderOption {
	return func(r *FFmpegRecorder) {
		if path != "" {
			r.commandPath = path
		}
	}
}

func WithInputDevice(device string) RecorderOption {
	return func(r *FFmpegRecorder) {
		if device != "" {
			r.inputDevice = device
		}
	}
}

func WithTempDir(dir string) RecorderOption {
	return func(r *FFmpegRecorder) {
		if dir != "" {
			r.tempDir = dir
		}
	}
}

func WithStopTimeout(timeout time.Duration) RecorderOption {
	return func(r *FFmpegRecorder) {
		if timeout > 0 {
			r.stopTimeout = timeout
		}
	}
}

func NewFFmpegRecorder(options ...RecorderOption) *FFmpegRecorder {
	recorder := &FFmpegRecorder{
		commandPath: "ffmpeg",
		inputDevice: "0",
		tempDir:     os.TempDir(),
		stopTimeout: 5 * time.Second,
	}
	for _, option := range options {
		option(recorder)
	}
	return recorder
}
