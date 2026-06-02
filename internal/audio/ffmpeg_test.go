package audio

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestFFmpegRecorderRejectsDeferredSources(t *testing.T) {
	recorder := NewFFmpegRecorder()
	for _, source := range []app.AudioSource{app.AudioSourceSystem, app.AudioSourceBoth} {
		t.Run(string(source), func(t *testing.T) {
			err := recorder.Start(context.Background(), source)
			if !errors.Is(err, ErrUnsupportedAudioInput) {
				t.Fatalf("error = %v, want %v", err, ErrUnsupportedAudioInput)
			}
		})
	}
}

func TestFFmpegChunkRecorderRejectsDeferredSources(t *testing.T) {
	recorder := NewFFmpegChunkRecorder()
	for _, source := range []app.AudioSource{app.AudioSourceSystem, app.AudioSourceBoth} {
		t.Run(string(source), func(t *testing.T) {
			err := recorder.Start(context.Background(), source)
			if !errors.Is(err, ErrUnsupportedAudioInput) {
				t.Fatalf("error = %v, want %v", err, ErrUnsupportedAudioInput)
			}
		})
	}
}

func TestFFmpegArgsUseMacOSAudioInput(t *testing.T) {
	args := ffmpegArgs("out.wav", "BlackHole 2ch")
	joined := stringsJoin(args, " ")
	if joined != "-hide_banner -loglevel error -f avfoundation -i :BlackHole 2ch -ac 1 -ar 16000 -c:a pcm_s16le -f wav -y out.wav" {
		t.Fatalf("args = %q", joined)
	}
}

func TestFFmpegSegmentArgsUseMacOSAudioInputAndChunkDuration(t *testing.T) {
	args := ffmpegSegmentArgs("chunks/chunk-%06d.wav", "BlackHole 2ch", 2500*time.Millisecond)
	joined := stringsJoin(args, " ")
	want := "-hide_banner -loglevel error -f avfoundation -i :BlackHole 2ch -ac 1 -ar 16000 -c:a pcm_s16le -f segment -segment_time 2.500 -reset_timestamps 1 -y chunks/chunk-%06d.wav"
	if joined != want {
		t.Fatalf("args = %q, want %q", joined, want)
	}
}

func TestFFmpegChunkRecorderNextChunkWaitsForCompletedSegment(t *testing.T) {
	dir := t.TempDir()
	recorder := NewFFmpegChunkRecorder()
	recorder.sessionDir = dir
	recorder.cmd = &exec.Cmd{}

	first := filepath.Join(dir, "chunk-000000.wav")
	if err := os.WriteFile(first, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if file, ok, err := recorder.NextChunk(context.Background()); err != nil || ok || file.Path != "" {
		t.Fatalf("active chunk result = %+v/%v/%v, want no chunk", file, ok, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "chunk-000001.wav"), []byte("next"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, ok, err := recorder.NextChunk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok || file.Path != first {
		t.Fatalf("completed chunk result = %+v/%v, want %s", file, ok, first)
	}
}

func TestFFmpegChunkRecorderFinalChunkDoesNotRequireNextSegment(t *testing.T) {
	dir := t.TempDir()
	recorder := NewFFmpegChunkRecorder()
	recorder.sessionDir = dir
	final := filepath.Join(dir, "chunk-000000.wav")
	if err := os.WriteFile(final, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}

	file, ok, err := recorder.nextReadyChunkLocked(true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || file.Path != final {
		t.Fatalf("final chunk result = %+v/%v, want %s", file, ok, final)
	}
}

func TestFFmpegChunkRecorderDrainsQueuedFinalChunks(t *testing.T) {
	dir := t.TempDir()
	recorder := NewFFmpegChunkRecorder()
	recorder.sessionDir = dir
	for _, name := range []string{"chunk-000000.wav", "chunk-000001.wav"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("audio"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files := recorder.drainReadyChunksLocked()

	if len(files) != 2 || files[0].Path != filepath.Join(dir, "chunk-000000.wav") || files[1].Path != filepath.Join(dir, "chunk-000001.wav") {
		t.Fatalf("files = %+v", files)
	}
}

func TestFFmpegChunkRecorderCleanupRemovesChunksAndSession(t *testing.T) {
	dir := t.TempDir()
	recorder := NewFFmpegChunkRecorder()
	recorder.sessionDir = dir
	chunk := filepath.Join(dir, "chunk-000000.wav")
	if err := os.WriteFile(chunk, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := recorder.CleanupChunk(context.Background(), app.AudioFile{Path: chunk}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(chunk); !os.IsNotExist(err) {
		t.Fatalf("chunk still exists or unexpected stat error: %v", err)
	}
	if err := recorder.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("session dir still exists or unexpected stat error: %v", err)
	}
}

func stringsJoin(values []string, separator string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += separator + value
	}
	return result
}
