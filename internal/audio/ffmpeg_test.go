package audio

import (
	"context"
	"errors"
	"testing"

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

func TestFFmpegArgsUseMacOSAudioInput(t *testing.T) {
	args := ffmpegArgs("out.wav", "BlackHole 2ch")
	joined := stringsJoin(args, " ")
	if joined != "-hide_banner -loglevel error -f avfoundation -i :BlackHole 2ch -ac 1 -ar 16000 -c:a pcm_s16le -f wav -y out.wav" {
		t.Fatalf("args = %q", joined)
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
