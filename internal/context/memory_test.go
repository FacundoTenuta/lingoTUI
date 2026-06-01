package context

import (
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestMemoryStoresCopiesAndClearsRecentContext(t *testing.T) {
	memory := NewMemory()
	if _, ok := memory.Current(); ok {
		t.Fatal("empty memory returned context")
	}

	summary := app.Summary{
		app.LanguageSpanish: "resumen",
		app.LanguageEnglish: "summary",
		app.LanguageGerman:  "zusammenfassung",
	}
	memory.Replace(app.RecentContext{
		Transcript: app.Transcript{Text: "hello"},
		Summary:    summary,
	})
	summary[app.LanguageEnglish] = "mutated"

	ctx, ok := memory.Current()
	if !ok {
		t.Fatal("stored context missing")
	}
	if ctx.Transcript.Text != "hello" || ctx.Summary[app.LanguageEnglish] != "summary" {
		t.Fatalf("context = %+v", ctx)
	}
	ctx.Summary[app.LanguageGerman] = "changed"
	again, _ := memory.Current()
	if again.Summary[app.LanguageGerman] != "zusammenfassung" {
		t.Fatalf("memory leaked mutable summary: %+v", again.Summary)
	}

	memory.Clear()
	if _, ok := memory.Current(); ok {
		t.Fatal("cleared memory returned context")
	}
}
