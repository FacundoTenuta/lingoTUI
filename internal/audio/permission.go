package audio

import (
	"context"
	"runtime"

	"github.com/FacundoTenuta/lingoTUI/internal/setup"
)

type MicrophonePermissionChecker struct{}

func NewMicrophonePermissionChecker() MicrophonePermissionChecker {
	return MicrophonePermissionChecker{}
}

func (MicrophonePermissionChecker) Microphone(context.Context) setup.ItemStatus {
	if runtime.GOOS != "darwin" {
		return setup.ItemStatus{
			Name:    "Microphone",
			State:   setup.StateUnknown,
			Message: "microphone recording is configured for macOS in this slice; no recording has started",
		}
	}
	return setup.ItemStatus{
		Name:    "Microphone",
		State:   setup.StateUnknown,
		Message: "macOS will ask for microphone access when /record mic starts; grant Terminal or iTerm in System Settings > Privacy & Security > Microphone. No recording has started",
	}
}
