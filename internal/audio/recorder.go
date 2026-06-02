package audio

import (
	"errors"
	"os"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

var _ app.Recorder = (*FFmpegRecorder)(nil)
var _ app.ChunkRecorder = (*FFmpegChunkRecorder)(nil)

var (
	ErrAlreadyRecording      = errors.New("ffmpeg recorder already active")
	ErrNotRecording          = errors.New("ffmpeg recorder is not active")
	ErrUnsupportedPlatform   = errors.New("ffmpeg microphone recording is only configured for macOS in this slice")
	ErrMissingOutputFile     = errors.New("ffmpeg did not produce an audio file")
	ErrUnsupportedAudioInput = errors.New("ffmpeg recorder supports microphone input only in this slice")
)

type RecorderOption func(*FFmpegRecorder)

type ChunkRecorderOption func(*FFmpegChunkRecorder)

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

func WithChunkCommandPath(path string) ChunkRecorderOption {
	return func(r *FFmpegChunkRecorder) {
		if path != "" {
			r.commandPath = path
		}
	}
}

func WithChunkInputDevice(device string) ChunkRecorderOption {
	return func(r *FFmpegChunkRecorder) {
		if device != "" {
			r.inputDevice = device
		}
	}
}

func WithChunkTempDir(dir string) ChunkRecorderOption {
	return func(r *FFmpegChunkRecorder) {
		if dir != "" {
			r.tempDir = dir
		}
	}
}

func WithChunkDuration(duration time.Duration) ChunkRecorderOption {
	return func(r *FFmpegChunkRecorder) {
		if duration > 0 {
			r.chunkDuration = duration
		}
	}
}

func WithChunkStopTimeout(timeout time.Duration) ChunkRecorderOption {
	return func(r *FFmpegChunkRecorder) {
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

func NewFFmpegChunkRecorder(options ...ChunkRecorderOption) *FFmpegChunkRecorder {
	recorder := &FFmpegChunkRecorder{
		commandPath:   "ffmpeg",
		inputDevice:   "0",
		tempDir:       os.TempDir(),
		chunkDuration: 4 * time.Second,
		stopTimeout:   5 * time.Second,
	}
	for _, option := range options {
		option(recorder)
	}
	return recorder
}
