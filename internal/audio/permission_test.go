package audio

import (
	"context"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/setup"
)

func TestMicrophonePermissionCheckerIsStatusOnly(t *testing.T) {
	status := NewMicrophonePermissionChecker().Microphone(context.Background())
	if status.Name != "Microphone" || status.State != setup.StateUnknown {
		t.Fatalf("status = %+v", status)
	}
	message := strings.ToLower(status.Message)
	for _, want := range []string{"no recording", "microphone"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q: %q", want, status.Message)
		}
	}
}
