//go:build integration

package openai_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
	"github.com/FacundoTenuta/lingoTUI/internal/provider/openai"
)

func TestClientIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OpenAI integration test in short mode")
	}
	apiKey := os.Getenv("LINGOTUI_OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("set LINGOTUI_OPENAI_API_KEY to run OpenAI integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client, err := openai.NewClient(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("chat summary", func(t *testing.T) {
		summary, err := client.Summarize(ctx, app.Transcript{Text: "Hola, necesito encontrar la estación de tren."}, app.SummaryLanguages(), app.ModelRef{Name: app.DefaultChatModel})
		if err != nil {
			t.Fatal(err)
		}
		if summary[app.LanguageSpanish] == "" || summary[app.LanguageEnglish] == "" || summary[app.LanguageGerman] == "" {
			t.Fatalf("summary = %+v", summary)
		}
	})

	t.Run("transcription", func(t *testing.T) {
		if os.Getenv("LINGOTUI_OPENAI_TRANSCRIBE") != "1" {
			t.Skip("set LINGOTUI_OPENAI_TRANSCRIBE=1 to also run live transcription")
		}
		audioPath := writeSilentWAV(t)
		transcript, err := client.Transcribe(ctx, app.AudioFile{Path: audioPath}, app.ModelRef{Name: app.DefaultTranscriptionModel})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("transcript from silent fixture: %q", transcript.Text)
	})
}

func writeSilentWAV(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "silence.wav")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	const sampleRate = 16000
	const bitsPerSample = 16
	const channels = 1
	dataSize := sampleRate * channels * bitsPerSample / 8
	chunkSize := uint32(36 + dataSize)
	if _, err := file.WriteString("RIFF"); err != nil {
		t.Fatal(err)
	}
	_ = binary.Write(file, binary.LittleEndian, chunkSize)
	_, _ = file.WriteString("WAVEfmt ")
	_ = binary.Write(file, binary.LittleEndian, uint32(16))
	_ = binary.Write(file, binary.LittleEndian, uint16(1))
	_ = binary.Write(file, binary.LittleEndian, uint16(channels))
	_ = binary.Write(file, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(file, binary.LittleEndian, uint32(sampleRate*channels*bitsPerSample/8))
	_ = binary.Write(file, binary.LittleEndian, uint16(channels*bitsPerSample/8))
	_ = binary.Write(file, binary.LittleEndian, uint16(bitsPerSample))
	_, _ = file.WriteString("data")
	_ = binary.Write(file, binary.LittleEndian, uint32(dataSize))
	if _, err := file.Write(make([]byte, dataSize)); err != nil {
		t.Fatal(err)
	}
	return path
}
