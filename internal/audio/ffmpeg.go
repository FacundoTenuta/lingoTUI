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

type FFmpegChunkRecorder struct {
	mu            sync.Mutex
	commandPath   string
	inputDevice   string
	tempDir       string
	chunkDuration time.Duration
	stopTimeout   time.Duration

	cmd        *exec.Cmd
	cancel     context.CancelFunc
	stdin      io.WriteCloser
	stderr     *bytes.Buffer
	sessionDir string
	nextIndex  int
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

func (r *FFmpegRecorder) Cleanup(_ context.Context, file app.AudioFile) error {
	if strings.TrimSpace(file.Path) == "" {
		return nil
	}
	return os.Remove(file.Path)
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

func (r *FFmpegChunkRecorder) Start(ctx context.Context, source app.AudioSource) error {
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
	sessionDir, err := os.MkdirTemp(r.tempDir, "lingotui-realtime-*")
	if err != nil {
		return fmt.Errorf("create temp realtime audio directory: %w", err)
	}

	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, commandPath, ffmpegSegmentArgs(r.segmentPattern(sessionDir), r.inputDevice, r.chunkDuration)...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		_ = os.RemoveAll(sessionDir)
		return fmt.Errorf("prepare ffmpeg stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.RemoveAll(sessionDir)
		return fmt.Errorf("start ffmpeg realtime microphone recording: %w", err)
	}

	r.cmd = cmd
	r.cancel = cancel
	r.stdin = stdin
	r.stderr = stderr
	r.sessionDir = sessionDir
	r.nextIndex = 0
	return nil
}

func (r *FFmpegChunkRecorder) NextChunk(context.Context) (app.AudioFile, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == nil {
		return app.AudioFile{}, false, ErrNotRecording
	}
	return r.nextReadyChunkLocked(false)
}

func (r *FFmpegChunkRecorder) Stop(context.Context) ([]app.AudioFile, error) {
	r.mu.Lock()
	cmd := r.cmd
	cancel := r.cancel
	stdin := r.stdin
	stderr := r.stderr
	stopTimeout := r.stopTimeout
	r.cmd, r.cancel, r.stdin, r.stderr = nil, nil, nil, nil
	r.mu.Unlock()

	if cmd == nil {
		return nil, ErrNotRecording
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
		_ = r.Cleanup(context.Background())
		return nil, fmt.Errorf("stop ffmpeg realtime microphone recording: %w%s", waitErr, stderrSuffix(stderr))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drainReadyChunksLocked(), nil
}

func (r *FFmpegChunkRecorder) CleanupChunk(_ context.Context, file app.AudioFile) error {
	if strings.TrimSpace(file.Path) == "" {
		return nil
	}
	if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove realtime audio chunk: %w", err)
	}
	return nil
}

func (r *FFmpegChunkRecorder) Cleanup(context.Context) error {
	r.mu.Lock()
	sessionDir := r.sessionDir
	r.sessionDir = ""
	r.nextIndex = 0
	r.mu.Unlock()
	if sessionDir == "" {
		return nil
	}
	if err := os.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("remove realtime audio directory: %w", err)
	}
	return nil
}

func (r *FFmpegChunkRecorder) nextReadyChunkLocked(final bool) (app.AudioFile, bool, error) {
	if r.sessionDir == "" {
		return app.AudioFile{}, false, ErrNotRecording
	}
	path := r.segmentPath(r.sessionDir, r.nextIndex)
	if !final && !fileExists(r.segmentPath(r.sessionDir, r.nextIndex+1)) {
		return app.AudioFile{}, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return app.AudioFile{}, false, nil
		}
		return app.AudioFile{}, false, fmt.Errorf("stat realtime audio chunk: %w", err)
	}
	if info.Size() == 0 {
		return app.AudioFile{}, false, nil
	}
	r.nextIndex++
	return app.AudioFile{Path: path}, true, nil
}

func (r *FFmpegChunkRecorder) drainReadyChunksLocked() []app.AudioFile {
	var files []app.AudioFile
	for {
		file, ok, err := r.nextReadyChunkLocked(true)
		if err != nil || !ok {
			return files
		}
		files = append(files, file)
	}
}

func (r *FFmpegChunkRecorder) segmentPattern(sessionDir string) string {
	return strings.TrimRight(sessionDir, string(os.PathSeparator)) + string(os.PathSeparator) + "chunk-%06d.wav"
}

func (r *FFmpegChunkRecorder) segmentPath(sessionDir string, index int) string {
	return strings.TrimRight(sessionDir, string(os.PathSeparator)) + string(os.PathSeparator) + fmt.Sprintf("chunk-%06d.wav", index)
}

func ffmpegSegmentArgs(outputPattern, inputDevice string, chunkDuration time.Duration) []string {
	input := strings.TrimSpace(inputDevice)
	if input == "" {
		input = "0"
	}
	if !strings.HasPrefix(input, ":") {
		input = ":" + input
	}
	seconds := chunkDuration.Seconds()
	if seconds <= 0 {
		seconds = 4
	}
	return []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "avfoundation",
		"-i", input,
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%.3f", seconds),
		"-reset_timestamps", "1",
		"-y", outputPattern,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
