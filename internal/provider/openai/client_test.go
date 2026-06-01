package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestClientSummarizeParsesJSONSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"es\":\"saludo\",\"en\":\"greeting\",\"de\":\"begrüßung\"}"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient("sk-test", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := client.Summarize(context.Background(), app.Transcript{Text: "hola"}, app.SummaryLanguages(), app.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if summary[app.LanguageSpanish] != "saludo" || summary[app.LanguageGerman] == "" {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestClientErrorDoesNotLeakAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key sk-secret", http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient("sk-secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Answer(context.Background(), app.Question("q"), app.RecentContext{}, app.ModelRef{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error leaked api key: %v", err)
	}
}
