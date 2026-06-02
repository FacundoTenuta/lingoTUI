//go:build integration

package audio_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/audio"
)

func TestFFmpegRecorderIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ffmpeg integration test in short mode")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("ffmpeg integration test currently targets macOS avfoundation input")
	}
	device := os.Getenv("LINGOTUI_FFMPEG_MIC_DEVICE")
	if device == "" {
		t.Skip("set LINGOTUI_FFMPEG_MIC_DEVICE to run ffmpeg microphone integration tests")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	recorder := audio.NewFFmpegRecorder(audio.WithInputDevice(device), audio.WithTempDir(t.TempDir()))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := recorder.Start(ctx, app.AudioSourceMic); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	file, err := recorder.Stop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatalf("recorded file %s is empty", file.Path)
	}
	if !strings.HasSuffix(file.Path, ".wav") {
		t.Fatalf("recorded file path = %q, want .wav extension", file.Path)
	}
	assertWAVHeader(t, file.Path)
}

func assertWAVHeader(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		t.Fatalf("read WAV header: %v", err)
	}
	if string(header[:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		t.Fatalf("recorded file header = %q/%q, want RIFF/WAVE", header[:4], header[8:12])
	}
}
