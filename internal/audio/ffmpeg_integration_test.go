package audio_test

import (
	"context"
	"os"
	"os/exec"
	"runtime"
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
}
