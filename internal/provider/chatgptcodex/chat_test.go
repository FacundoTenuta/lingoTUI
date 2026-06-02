package chatgptcodex

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestChatSummarizeParsesMultilingualJSON(t *testing.T) {
	fake := &fakeResponseCreator{response: Response{Text: `{"es":" saludo ","en":"greeting","de":"begrüßung","fr":"ignored"}`}}
	chat := NewChat(fake)

	summary, err := chat.Summarize(context.Background(), app.Transcript{Text: "hola secret transcript"}, app.SummaryLanguages(), app.ModelRef{Name: "codex-mini"})
	if err != nil {
		t.Fatal(err)
	}

	if summary[app.LanguageSpanish] != "saludo" || summary[app.LanguageEnglish] != "greeting" || summary[app.LanguageGerman] != "begrüßung" {
		t.Fatalf("summary = %+v", summary)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.requests))
	}
	req := fake.requests[0]
	if req.Model != "codex-mini" {
		t.Fatalf("model = %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Text, "compact JSON object") {
		t.Fatalf("system message = %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || !strings.Contains(req.Messages[1].Text, "Languages: es, en, de") || !strings.Contains(req.Messages[1].Text, "hola secret transcript") {
		t.Fatalf("user message = %+v", req.Messages[1])
	}
}

func TestChatAnswerBuildsRequestAndTrimsResponse(t *testing.T) {
	fake := &fakeResponseCreator{response: Response{Text: "  It means hello.\n"}}
	chat := NewChat(fake)
	recent := app.RecentContext{
		Transcript: app.Transcript{Text: "Hola mundo"},
		Summary: app.Summary{
			app.LanguageSpanish: "saludo",
			app.LanguageEnglish: "greeting",
		},
	}

	answer, err := chat.Answer(context.Background(), app.Question("What does hola mean?"), recent, app.ModelRef{Name: " codex-mini "})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "It means hello." {
		t.Fatalf("answer = %q", answer)
	}

	req := fake.requests[0]
	if req.Model != "codex-mini" {
		t.Fatalf("model = %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(req.Messages))
	}
	user := req.Messages[1].Text
	for _, want := range []string{"Transcript:\nHola mundo", "Summary:\nES: saludo\nEN: greeting", "Question: What does hola mean?"} {
		if !strings.Contains(user, want) {
			t.Fatalf("user message = %q, missing %q", user, want)
		}
	}
}

func TestChatErrorsAreSanitizedAndContextErrorsPreserved(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "client error", err: errors.New("secret transcript secret question raw response token account-id")},
		{name: "context canceled", err: context.Canceled, wantErr: context.Canceled},
		{name: "deadline", err: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := NewChat(&fakeResponseCreator{err: tt.err})
			_, err := chat.Answer(context.Background(), app.Question("secret question"), app.RecentContext{Transcript: app.Transcript{Text: "secret transcript"}}, app.ModelRef{Name: "codex-mini"})
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			assertNoLeak(t, err.Error(), "secret transcript", "secret question", "raw response", "token", "account-id")
			if err.Error() != "create ChatGPT Codex response" {
				t.Fatalf("error = %q", err.Error())
			}
		})
	}
}

func TestChatSummarizeMalformedOrEmptyResponseReturnsSanitizedError(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "malformed json", text: `{"es":"saludo", secret transcript`},
		{name: "empty text", text: "  "},
		{name: "wrong json shape", text: `{"es":{"text":"secret transcript"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := NewChat(&fakeResponseCreator{response: Response{Text: tt.text}})
			_, err := chat.Summarize(context.Background(), app.Transcript{Text: "secret transcript"}, app.SummaryLanguages(), app.ModelRef{Name: "codex-mini"})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "parse ChatGPT Codex summary response") {
				t.Fatalf("error = %q", err.Error())
			}
			assertNoLeak(t, err.Error(), "secret transcript")
		})
	}
}

type fakeResponseCreator struct {
	response Response
	err      error
	requests []ResponseRequest
}

func (f *fakeResponseCreator) CreateResponse(_ context.Context, req ResponseRequest) (Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return Response{}, f.err
	}
	return f.response, nil
}
