package provider

import (
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestDefaultRegistryIncludesOpenAIModelsAndAuthSeams(t *testing.T) {
	registry := DefaultRegistry()
	openai, ok := registry.Get(app.ProviderOpenAI)
	if !ok {
		t.Fatal("openai provider missing")
	}
	if !openai.SupportsAuth(AuthMethodAPIKey) || !openai.SupportsAuth(AuthMethodBrowser) {
		t.Fatalf("auth methods = %+v", openai.AuthMethods)
	}
	if got := openai.ModelsForPurpose(app.ModelPurposeTranscription); len(got) != 1 || got[0].Name != app.DefaultTranscriptionModel {
		t.Fatalf("transcription models = %+v", got)
	}
	if got := openai.ModelsForPurpose(app.ModelPurposeChat); len(got) != 1 || got[0].Name != app.DefaultChatModel {
		t.Fatalf("chat models = %+v", got)
	}
}
