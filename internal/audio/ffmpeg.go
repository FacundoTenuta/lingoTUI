package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

type FFmpegRecorder struct {
	mu          sync.Mutex
	commandPath string
	inputDevice string
	tempDir     string
	stopTimeout time.Duration

	cmd      *exec.Cmd
	cancel   context.CancelFunc
	stdin    io.WriteCloser
	stderr   *bytes.Buffer
	filePath string
}

func (r *FFmpegRecorder) Start(ctx context.Context, source app.AudioSource) error {
	if source != app.AudioSourceMic {
		return fmt.Errorf("%w: %s is deferred", ErrUnsupportedAudioInput, source)
	}
	if runtime.GOOS != "darwin" {
		return ErrUnsupportedPlatform
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil {
		return ErrAlreadyRecording
	}
	commandPath, err := exec.LookPath(r.commandPath)
	if err != nil {
		return fmt.Errorf("find ffmpeg: %w", err)
	}
	if err := os.MkdirAll(r.tempDir, 0o700); err != nil {
		return fmt.Errorf("create temp audio directory: %w", err)
	}
	file, err := os.CreateTemp(r.tempDir, "lingotui-*.wav")
	if err != nil {
		return fmt.Errorf("create temp audio file: %w", err)
	}
	filePath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(filePath)
		return fmt.Errorf("close temp audio file: %w", err)
	}

	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, commandPath, ffmpegArgs(filePath, r.inputDevice)...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		_ = os.Remove(filePath)
		return fmt.Errorf("prepare ffmpeg stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.Remove(filePath)
		return fmt.Errorf("start ffmpeg microphone recording: %w", err)
	}

	r.cmd = cmd
	r.cancel = cancel
	r.stdin = stdin
	r.stderr = stderr
	r.filePath = filePath
	return nil
}

func (r *FFmpegRecorder) Stop(context.Context) (app.AudioFile, error) {
	r.mu.Lock()
	cmd := r.cmd
	cancel := r.cancel
	stdin := r.stdin
	stderr := r.stderr
	filePath := r.filePath
	stopTimeout := r.stopTimeout
	r.cmd, r.cancel, r.stdin, r.stderr, r.filePath = nil, nil, nil, nil, ""
	r.mu.Unlock()

	if cmd == nil {
		return app.AudioFile{}, ErrNotRecording
	}
	if stdin != nil {
		_, _ = stdin.Write([]byte("q\n"))
		_ = stdin.Close()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(stopTimeout):
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		waitErr = <-done
	}
	if cancel != nil {
		cancel()
	}
	if waitErr != nil {
		_ = os.Remove(filePath)
		return app.AudioFile{}, fmt.Errorf("stop ffmpeg microphone recording: %w%s", waitErr, stderrSuffix(stderr))
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return app.AudioFile{}, fmt.Errorf("stat temp audio file: %w", err)
	}
	if info.Size() == 0 {
		_ = os.Remove(filePath)
		return app.AudioFile{}, ErrMissingOutputFile
	}
	return app.AudioFile{Path: filePath}, nil
}

func ffmpegArgs(outputPath, inputDevice string) []string {
	input := strings.TrimSpace(inputDevice)
	if input == "" {
		input = "0"
	}
	if !strings.HasPrefix(input, ":") {
		input = ":" + input
	}
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "avfoundation",
		"-i", input,
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		"-f", "wav",
		"-y", outputPath,
	}
}

func stderrSuffix(stderr *bytes.Buffer) string {
	if stderr == nil {
		return ""
	}
	message := strings.TrimSpace(stderr.String())
	if message == "" {
		return ""
	}
	if len(message) > 512 {
		message = message[:512] + "..."
	}
	return ": " + message
}
